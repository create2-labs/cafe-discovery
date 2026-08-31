package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"cafe-discovery/internal/config"
	"cafe-discovery/internal/discoveryroutes"
	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/handler"
	"cafe-discovery/internal/metrics"
	"cafe-discovery/internal/middleware"
	"cafe-discovery/internal/repository"
	"cafe-discovery/internal/service"
	"cafe-discovery/internal/version"
	"cafe-discovery/pkg/nats"
	postgresdb "cafe-discovery/pkg/postgres"
	redisconn "cafe-discovery/pkg/redis"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/google/uuid"
	natsio "github.com/nats-io/nats.go"
	"github.com/spf13/viper"
)

const (
	walletPubKeyHashPath = "/:pubKeyHash"
)

// Container holds all application dependencies
type Container struct {
	ChainConfig              *config.ChainConfig
	DiscoveryHandler         *handler.DiscoveryHandler
	TLSHandler               *handler.TLSHandler
	AuthService              *service.AuthService
	AuthHandler              *handler.AuthHandler
	CafeWalletService        *service.CafeWalletService
	CafeWalletHandler        *handler.CafeWalletHandler
	ScanAuthorizationService *service.ScanAuthorizationService
	ScanAuthorizationHandler *handler.ScanAuthorizationHandler
	App                      *fiber.App
	DB                       postgresdb.PostgreSQLConnection
	NATSConn                 nats.Connection
	RedisConn                redisconn.Connection
	ScannerPresenceTracker   *service.ScannerPresenceTracker
}

// NewContainer creates and initializes the application container
func NewContainer(cfgChain *config.ChainConfig) (*Container, error) {
	// Initialize PostgreSQL database
	db := postgresdb.New()
	if err := db.Run(); err != nil {
		return nil, fmt.Errorf("failed to initialize PostgreSQL database: %w", err)
	}

	// Initialize NATS connection
	natsConn, err := nats.New()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize NATS: %w", err)
	}

	// Initialize Redis connection
	redisConn, err := redisconn.New()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Redis: %w", err)
	}

	// Run identity migrations; scan DDL is owned by cafe-persistence (PERS-D2b).
	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Get JWT secret from Viper (reads from config file or environment variable)
	jwtSecret := viper.GetString(config.JWTSecret)
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET not set in config file or environment variable")
	}
	jwtExpiry := 24 * time.Hour // Token expires in 24 hours

	// Initialize repositories
	userRepo := repository.NewUserRepository(db.GetDB())
	scanResultRepo := repository.NewScanResultRepository(db.GetDB())
	tlsScanResultRepo := repository.NewTLSScanResultRepository(db.GetDB())
	cafeWalletRepo := repository.NewCafeWalletRepository(db.GetDB())
	planRepo := repository.NewPlanRepository(db.GetDB())
	scanUsageLedgerRepo := repository.NewScanUsageLedgerRepository(db.GetDB())

	// Initialize plan service
	planService := service.NewPlanService(planRepo, userRepo)

	authService, err := service.NewAuthService(userRepo, planRepo, jwtSecret, jwtExpiry)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize auth service: %w", err)
	}
	cafeWalletService := service.NewCafeWalletService(cafeWalletRepo)

	scannerPresence, err := service.NewScannerPresenceTracker(natsConn, redisConn)
	if err != nil {
		return nil, fmt.Errorf("failed to create scanner presence tracker: %w", err)
	}

	// Redis: TLS read-through cache.
	redisTLSRepo := repository.NewRedisTLSScanRepository(redisConn)

	// User scan cache: TLS read-through and warm on sign-in (wallet history is Postgres-only).
	userScanCache := service.NewUserScanCacheService(tlsScanResultRepo, redisTLSRepo)

	scanClient, err := newScanPersistenceClient()
	if err != nil {
		return nil, err
	}
	policyRef, err := newPolicyReferenceChecker()
	if err != nil {
		return nil, err
	}

	// Initialize handlers (v1 GET/list/delete/pending via cafe-persistence internal/scan/v1).
	discoveryHandler := handler.NewDiscoveryHandler(cfgChain, natsConn, planService, scannerPresence, userScanCache, scanClient, scanResultRepo, scanUsageLedgerRepo, scanClient, policyRef)
	tlsHandler := handler.NewTLSHandler(scanClient, scanClient, policyRef)
	authHandler := handler.NewAuthHandler(authService, userScanCache)
	cafeWalletHandler := handler.NewCafeWalletHandler(cafeWalletService)
	planHandler := handler.NewPlanHandler(planService, scanUsageLedgerRepo)

	// AUTH-05: internal scan-authorization service consumed by CPM (AUTH-02).
	// Discovery remains the authoritative source for scan visibility; CPM
	// must not access Discovery persistence directly.
	scanAuthzService := service.NewScanAuthorizationService(scanResultRepo, tlsScanResultRepo)
	scanAuthzEnabled := viper.GetBool(config.DiscoveryInternalAuthzEnabled)
	scanAuthzHandler := handler.NewScanAuthorizationHandler(scanAuthzService, scanAuthzEnabled)
	scanAuthzServiceToken := viper.GetString(config.DiscoveryInternalAuthzServiceToken)
	if scanAuthzEnabled && scanAuthzServiceToken == "" {
		log.Printf("Warning: AUTH-05 internal scan-authorization endpoint enabled without %s; the endpoint will reject every caller until a service token is configured", config.DiscoveryInternalAuthzServiceToken)
	}

	// Initialize Prometheus metrics
	// This must be called before starting the server to register all metrics
	metrics.Init()

	// Initialize Fiber app
	app := fiber.New(fiber.Config{
		AppName: "Cafe Discovery Service",
		// Buffer sizes to support PQC JWT tokens (hybrid tokens are larger)
		ReadBufferSize:  10240,
		WriteBufferSize: 10240,
	})

	// Enable CORS — Viper keeps CSV env strings; Fiber v3 requires []string.
	app.Use(cors.New(cors.Config{
		AllowOrigins:     splitCSV(viper.GetString(config.CORSAllowOrigins)),
		AllowMethods:     splitCSV(viper.GetString(config.CORSAllowMethods)),
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		AllowCredentials: true,
		ExposeHeaders:    []string{"Content-Length"},
		MaxAge:           60, // 1 mn - cache preflight requests (reduces OPTIONS requests)
		// MaxAge:           3600, // 1 hour - cache preflight requests (reduces OPTIONS requests)
	}))

	// HTTP request metrics (http_requests_total, http_request_duration_seconds) for Prometheus / Grafana
	app.Use(metrics.HTTPMiddleware())

	// Setup routes
	setupRoutes(app, discoveryHandler, tlsHandler, authHandler, authService, cafeWalletHandler, planHandler, scanAuthzHandler, scanAuthzServiceToken)

	container := &Container{
		ChainConfig:              cfgChain,
		DiscoveryHandler:         discoveryHandler,
		TLSHandler:               tlsHandler,
		AuthService:              authService,
		AuthHandler:              authHandler,
		CafeWalletService:        cafeWalletService,
		CafeWalletHandler:        cafeWalletHandler,
		ScanAuthorizationService: scanAuthzService,
		ScanAuthorizationHandler: scanAuthzHandler,
		App:                      app,
		DB:                       db,
		NATSConn:                 natsConn,
		RedisConn:                redisConn,
		ScannerPresenceTracker:   scannerPresence,
	}

	// PERS-D2b: persistence must be ready (scan migrations + NATS) before scan API traffic.
	ctx := context.Background()
	if err := service.WaitForPersistence(ctx, natsConn, 15*time.Second); err != nil {
		return nil, fmt.Errorf("persistence not ready: %w (scan tables are owned by cafe-persistence; start persistence first)", err)
	}
	if err := service.WaitForScanners(ctx, scannerPresence, 30*time.Second); err != nil {
		log.Printf("Warning: scanners not ready in time: %v (default endpoints may be empty)", err)
	}
	service.InitializeDefaultEndpointsSync(ctx, natsConn, redisTLSRepo)

	// Subscribe to scan.ready so backend is notified when a scan is stored (Redis/Postgres).
	if _, err := natsConn.Subscribe(nats.SubjectScanReady, func(msg *natsio.Msg) {
		var m nats.ScanReadyMessage
		if err := json.Unmarshal(msg.Data, &m); err != nil {
			return
		}
		log.Printf("scan.ready: user=%s kind=%s status=%s endpoint=%s address=%s", m.UserID.String(), m.Kind, m.Status, m.Endpoint, m.Address)
	}); err != nil {
		log.Printf("Warning: subscribe scan.ready failed: %v", err)
	}

	return container, nil
}

// setupRoutes configures all HTTP routes
func setupRoutes(app *fiber.App, discoveryHandler *handler.DiscoveryHandler, tlsHandler *handler.TLSHandler, authHandler *handler.AuthHandler, authService *service.AuthService, cafeWalletHandler *handler.CafeWalletHandler, planHandler *handler.PlanHandler, scanAuthzHandler *handler.ScanAuthorizationHandler, scanAuthzServiceToken string) {
	// Public auth routes
	auth := app.Group("/auth")
	auth.Post("/signup", authHandler.Signup)
	auth.Post("/signin", authHandler.Signin)

	// Health check endpoint (public).
	healthHandler := func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "ok",
			"app_name":  "Cafe Discovery Service",
			"version":   "1.0.0",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
	app.Get("/health", healthHandler)

	// Version endpoint (public) — DISC-OPS-1: same contract as CPM GET /version.
	app.Get("/version", func(c fiber.Ctx) error {
		return c.JSON(version.Payload())
	})

	// Prometheus metrics endpoint (public) — Fiber v3 registers net/http.Handler directly.
	app.Get("/metrics", metrics.Handler())

	// Public discovery utilities v1 (no JWT) — WORKPLAN_API_PR PR13d.
	v1Public := app.Group(discoveryroutes.V1Base)
	v1Public.Get("/rpcs", discoveryHandler.ListRPCs)
	v1Public.Get("/scanners", discoveryHandler.ListAvailableScanners)

	// WORKPLAN §0.1 — /discovery/v1 (PR2 skeleton, PR3 POST /scan, PR4–PR6 list/detail/delete).
	apiV1 := app.Group(discoveryroutes.V1Base, middleware.JWTMiddleware(authService))
	registerDiscoveryV1Routes(apiV1, discoveryHandler, tlsHandler, cafeWalletHandler)

	// Plan routes
	plans := app.Group("/plans", middleware.JWTMiddleware(authService))
	plans.Get("/", planHandler.GetAllPlans)
	plans.Get("/current", planHandler.GetUserPlan)
	plans.Get("/usage", planHandler.GetPlanUsage)

	// AUTH-05: internal scan-authorization endpoint consumed by CPM AUTH-02.
	// This route is gated by an internal service-token middleware; the
	// X-User-Id, X-Tenant-Id, and X-Request-Id headers are only trusted
	// after the service-auth check has passed. The endpoint must not be
	// reachable through public ingress.
	internalAuth := app.Group("/internal/auth", middleware.InternalServiceAuth(middleware.InternalServiceAuthConfig{
		ExpectedToken: scanAuthzServiceToken,
	}))
	internalAuth.Post("/session/validate", authHandler.ValidateSessionForCPM)

	internalAuthz := app.Group("/internal/authz", middleware.InternalServiceAuth(middleware.InternalServiceAuthConfig{
		ExpectedToken: scanAuthzServiceToken,
	}))
	internalAuthz.Post("/scans/:scanId/can-read", scanAuthzHandler.CanReadScan)
}

// ensurePlanExists ensures a plan exists, creating it if it doesn't
func ensurePlanExists(planRepo repository.PlanRepository, planType domain.PlanType, defaultPlan *domain.Plan) (*domain.Plan, error) {
	plan, _ := planRepo.FindByType(planType)
	if plan != nil {
		return plan, nil
	}

	if err := planRepo.Create(defaultPlan); err != nil {
		return nil, fmt.Errorf("failed to create %s plan: %w", planType, err)
	}
	return defaultPlan, nil
}

// assignPlanToUsersWithoutPlan assigns the free plan to users without a plan
func assignPlanToUsersWithoutPlan(db postgresdb.PostgreSQLConnection, freePlan *domain.Plan) error {
	var usersWithoutPlan []domain.User
	if err := db.GetDB().Where("plan_id = ? OR plan_id IS NULL", uuid.Nil).Find(&usersWithoutPlan).Error; err != nil {
		return nil // Ignore query errors, continue with migration
	}

	for _, user := range usersWithoutPlan {
		if user.PlanID == uuid.Nil {
			if err := db.GetDB().Model(&user).Update("plan_id", freePlan.ID).Error; err != nil {
				// Log error but continue
				fmt.Printf("Warning: failed to assign plan to user %s: %v\n", user.ID, err)
			}
		}
	}
	return nil
}

// splitCSV splits a comma-separated config string into a trimmed slice.
// Empty input and empty segments are dropped so Fiber never sees [""].
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

// Start starts the HTTP server
func (c *Container) Start() error {
	addr := viper.GetString(config.ServerHost) + ":" + viper.GetString(config.ServerPort)
	return c.App.Listen(addr, fiber.ListenConfig{DisableStartupMessage: true})
}

// Shutdown gracefully shuts down the server
func (c *Container) Shutdown() error {
	if c.ScannerPresenceTracker != nil {
		_ = c.ScannerPresenceTracker.Close()
	}
	if c.AuthService != nil {
		c.AuthService.Close()
	}
	if c.NATSConn != nil {
		c.NATSConn.Close()
	}
	if c.DB != nil {
		c.DB.Shutdown()
	}
	return c.App.Shutdown()
}
