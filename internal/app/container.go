package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"cafe-discovery/internal/config"
	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/handler"
	"cafe-discovery/internal/metrics"
	"cafe-discovery/internal/middleware"
	"cafe-discovery/internal/repository"
	"cafe-discovery/internal/service"
	"cafe-discovery/pkg/evm"
	"cafe-discovery/pkg/moralis"
	"cafe-discovery/pkg/nats"
	postgresdb "cafe-discovery/pkg/postgres"
	redisconn "cafe-discovery/pkg/redis"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/google/uuid"
	natsio "github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
)

const (
	walletPubKeyHashPath = "/:pubKeyHash"
)

// Container holds all application dependencies
type Container struct {
	ChainConfig              *config.ChainConfig
	DiscoveryService         *service.DiscoveryService
	DiscoveryHandler         *handler.DiscoveryHandler
	TLSService               *service.TLSService
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
	MoralisClient            *moralis.MoralisClient
	ScannerPresenceTracker   *service.ScannerPresenceTracker
}

// NewContainer creates and initializes the application container
func NewContainer(cfgChain *config.ChainConfig) (*Container, error) {
	// Create EVM clients for each configured blockchain
	clients := make(map[string]*evm.Client)
	for _, blockchain := range cfgChain.Blockchains {
		clients[blockchain.Name] = evm.NewClient(blockchain.RPC, blockchain.MoralisChainName)
	}

	// Initialize Moralis client
	moralisClient := moralis.NewMoralisClient(viper.GetString(config.MoralisAPIKey), viper.GetString(config.MoralisAPIURL))

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

	// Run database migrations
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

	// Initialize plan service
	planService := service.NewPlanService(planRepo, userRepo)

	// Initialize services
	discoveryService := service.NewDiscoveryService(clients, moralisClient, scanResultRepo, planService)
	tlsService := service.NewTLSService(tlsScanResultRepo, planService)
	authService, err := service.NewAuthService(userRepo, planRepo, jwtSecret, jwtExpiry)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize auth service: %w", err)
	}
	cafeWalletService := service.NewCafeWalletService(cafeWalletRepo)

	scannerPresence, err := service.NewScannerPresenceTracker(natsConn, redisConn)
	if err != nil {
		return nil, fmt.Errorf("failed to create scanner presence tracker: %w", err)
	}

	// Redis scan repos (backend read-through: Redis then Postgres for scan list/get)
	redisTLSRepo := repository.NewRedisTLSScanRepository(redisConn)
	redisWalletRepo := repository.NewRedisWalletScanRepository(redisConn)

	// User scan cache: read-through and warm cache on sign-in
	userScanCache := service.NewUserScanCacheService(scanResultRepo, tlsScanResultRepo, redisWalletRepo, redisTLSRepo)

	// Initialize handlers (read-through for scan list/get; plan usage from Redis counts)
	discoveryHandler := handler.NewDiscoveryHandler(discoveryService, tlsService, cfgChain, natsConn, planService, scannerPresence, redisWalletRepo, redisTLSRepo, userScanCache)
	tlsHandler := handler.NewTLSHandler(tlsService, natsConn, redisTLSRepo, planService, userScanCache)
	authHandler := handler.NewAuthHandler(authService, userScanCache)
	cafeWalletHandler := handler.NewCafeWalletHandler(cafeWalletService)
	planHandler := handler.NewPlanHandler(planService, redisWalletRepo, redisTLSRepo)

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

	// Enable CORS
	corsOrigins := viper.GetString(config.CORSAllowOrigins)
	corsMethods := viper.GetString(config.CORSAllowMethods)
	app.Use(cors.New(cors.Config{
		AllowOrigins:     corsOrigins,
		AllowMethods:     corsMethods,
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Requested-With",
		AllowCredentials: true,
		ExposeHeaders:    "Content-Length",
		MaxAge:           60, // 1 mn - cache preflight requests (reduces OPTIONS requests)
		// MaxAge:           3600, // 1 hour - cache preflight requests (reduces OPTIONS requests)
	}))

	// HTTP request metrics (http_requests_total, http_request_duration_seconds) for Prometheus / Grafana
	app.Use(metrics.HTTPMiddleware())

	// Setup routes
	setupRoutes(app, discoveryHandler, tlsHandler, authHandler, authService, cafeWalletHandler, planHandler, scanAuthzHandler, scanAuthzServiceToken)

	container := &Container{
		ChainConfig:              cfgChain,
		DiscoveryService:         discoveryService,
		DiscoveryHandler:         discoveryHandler,
		TLSService:               tlsService,
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
		MoralisClient:            moralisClient,
		ScannerPresenceTracker:   scannerPresence,
	}

	// Wait for persistence and scanners, then initialize default endpoints via NATS and wait until they are in Redis
	ctx := context.Background()
	if err := service.WaitForPersistence(ctx, natsConn, 15*time.Second); err != nil {
		log.Printf("Warning: persistence not ready in time: %v (default endpoints may be empty)", err)
	}
	if err := service.WaitForScanners(ctx, scannerPresence, 30*time.Second); err != nil {
		log.Printf("Warning: scanners not ready in time: %v (default endpoints may be empty)", err)
	}
	service.InitializeDefaultEndpointsSync(ctx, natsConn, redisTLSRepo)

	// Subscribe to scan.ready so backend is notified when a scan is stored (Redis/Postgres); GET /discovery/cbom will then return the result
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
	healthHandler := func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "ok",
			"app_name":  "Cafe Discovery Service",
			"version":   "1.0.0",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
	app.Get("/health", healthHandler)

	// Prometheus metrics endpoint (public)
	// This endpoint exposes metrics in Prometheus format for scraping
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))

	// Public discovery routes (no authentication)
	discoveryPublic := app.Group("/discovery")
	discoveryPublic.Get("/rpcs", discoveryHandler.ListRPCs)
	discoveryPublic.Get("/scanners", discoveryHandler.ListAvailableScanners)

	// Protected discovery routes - require JWT authentication
	api := app.Group("/discovery", middleware.JWTMiddleware(authService))
	api.Post("/scan", discoveryHandler.UnifiedScan) // Unified scan endpoint - automatically detects wallet or TLS endpoint
	api.Post("/assessments/request", discoveryHandler.RequestAssessment)
	api.Get("/scans", discoveryHandler.ListScans)
	api.Get("/wallet-policy-contexts", discoveryHandler.ListWalletPolicyContexts)
	api.Get("/cbom/*", discoveryHandler.GetCBOM) // Get CBOM for a wallet address or TLS endpoint (wildcard to handle URLs)
	api.Get("/tls/scans", tlsHandler.ListScans)

	// WORKPLAN §0.1 — /discovery/v1 (PR2 skeleton, PR3 POST /scan, PR4–PR6 list/detail/delete).
	apiV1 := app.Group("/discovery/v1", middleware.JWTMiddleware(authService))
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

// runMigrations runs database migrations
func runMigrations(db postgresdb.PostgreSQLConnection) error {
	// Auto-migrate all models
	if err := db.GetDB().AutoMigrate(
		&domain.Plan{},
		&domain.User{},
		&domain.ScanResultEntity{},
		&domain.TLSScanResultEntity{},
		&domain.CafeWallet{},
	); err != nil {
		return err
	}

	planRepo := repository.NewPlanRepository(db.GetDB())

	// Create default plans if they don't exist
	freePlan, err := ensurePlanExists(planRepo, domain.PlanTypeFree, &domain.Plan{
		Name:              "Free Plan",
		Type:              domain.PlanTypeFree,
		WalletScanLimit:   5,
		EndpointScanLimit: 5,
		Price:             0,
		IsActive:          true,
	})
	if err != nil {
		return err
	}

	_, err = ensurePlanExists(planRepo, domain.PlanTypePremium, &domain.Plan{
		Name:              "CAFEIN Premium Plan",
		Type:              domain.PlanTypePremium,
		WalletScanLimit:   0, // Unlimited
		EndpointScanLimit: 0, // Unlimited
		Price:             29.99,
		IsActive:          false, // Coming soon
	})
	if err != nil {
		return err
	}

	// Assign FREE plan to existing users without a plan
	if err := assignPlanToUsersWithoutPlan(db, freePlan); err != nil {
		return err
	}

	return nil
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

// Start starts the HTTP server
func (c *Container) Start() error {
	addr := viper.GetString(config.ServerHost) + ":" + viper.GetString(config.ServerPort)
	return c.App.Listen(addr)
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
