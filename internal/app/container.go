package app

import (
	"cafe-discovery/internal/config"
	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/handler"
	"cafe-discovery/internal/middleware"
	"cafe-discovery/internal/repository"
	"cafe-discovery/internal/service"
	"cafe-discovery/pkg/evm"
	"cafe-discovery/pkg/moralis"
	"cafe-discovery/pkg/nats"
	postgresdb "cafe-discovery/pkg/postgres"
	"fmt"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/google/uuid"
	"github.com/spf13/viper"
)

// Container holds all application dependencies
type Container struct {
	ChainConfig       *config.ChainConfig
	DiscoveryService  *service.DiscoveryService
	DiscoveryHandler  *handler.DiscoveryHandler
	TLSService        *service.TLSService
	TLSHandler        *handler.TLSHandler
	AuthService       *service.AuthService
	AuthHandler       *handler.AuthHandler
	CafeWalletService *service.CafeWalletService
	CafeWalletHandler *handler.CafeWalletHandler
	App               *fiber.App
	DB                postgresdb.PostgreSQLConnection
	NATSConn          nats.Connection
	MoralisClient     *moralis.MoralisClient
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

	// Run database migrations
	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Get JWT secret from environment or use default
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-in-production" // Default for development
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
	authService := service.NewAuthService(userRepo, planRepo, jwtSecret, jwtExpiry)
	cafeWalletService := service.NewCafeWalletService(cafeWalletRepo)

	// Initialize handlers
	discoveryHandler := handler.NewDiscoveryHandler(discoveryService, cfgChain, natsConn)
	tlsHandler := handler.NewTLSHandler(tlsService, natsConn)
	authHandler := handler.NewAuthHandler(authService)
	cafeWalletHandler := handler.NewCafeWalletHandler(cafeWalletService)
	planHandler := handler.NewPlanHandler(planService, scanResultRepo, tlsScanResultRepo)

	// Initialize Fiber app
	app := fiber.New(fiber.Config{
		AppName: "Cafe Discovery Service",
	})

	// Enable CORS
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:3000,http://localhost:3001,http://localhost:5173",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization",
		AllowCredentials: true,
		ExposeHeaders:    "Content-Length",
	}))

	// Setup routes
	setupRoutes(app, discoveryHandler, tlsHandler, authHandler, authService, cafeWalletHandler, planHandler)

	container := &Container{
		ChainConfig:       cfgChain,
		DiscoveryService:  discoveryService,
		DiscoveryHandler:  discoveryHandler,
		TLSService:        tlsService,
		TLSHandler:        tlsHandler,
		AuthService:       authService,
		AuthHandler:       authHandler,
		CafeWalletService: cafeWalletService,
		CafeWalletHandler: cafeWalletHandler,
		App:               app,
		DB:                db,
		NATSConn:          natsConn,
		MoralisClient:     moralisClient,
	}

	return container, nil
}

// setupRoutes configures all HTTP routes
func setupRoutes(app *fiber.App, discoveryHandler *handler.DiscoveryHandler, tlsHandler *handler.TLSHandler, authHandler *handler.AuthHandler, authService *service.AuthService, cafeWalletHandler *handler.CafeWalletHandler, planHandler *handler.PlanHandler) {
	// Public auth routes
	auth := app.Group("/auth")
	auth.Post("/signup", authHandler.Signup)
	auth.Post("/signin", authHandler.Signin)

	// Health check endpoint (public)
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "ok",
			"app_name":  "Cafe Discovery Service",
			"version":   "1.0.0",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})

	// Protected routes - require JWT authentication
	api := app.Group("/discovery", middleware.JWTMiddleware(authService))
	api.Post("/scan/wallet", discoveryHandler.Scan)
	api.Post("/scan/endpoints", tlsHandler.Scan)
	api.Get("/rpcs", discoveryHandler.ListRPCs)
	api.Get("/scans", discoveryHandler.ListScans)
	api.Get("/tls/scans", tlsHandler.ListScans)

	// Wallet management routes
	wallets := app.Group("/wallets", middleware.JWTMiddleware(authService))
	wallets.Get("/", cafeWalletHandler.GetAllWallets)
	wallets.Post("/", cafeWalletHandler.CreateWallet)
	wallets.Get("/:pubKeyHash", cafeWalletHandler.GetWallet)
	wallets.Put("/:pubKeyHash", cafeWalletHandler.UpdateWallet)
	wallets.Delete("/:pubKeyHash", cafeWalletHandler.DeleteWallet)

	// Plan routes
	plans := app.Group("/plans", middleware.JWTMiddleware(authService))
	plans.Get("/", planHandler.GetAllPlans)
	plans.Get("/current", planHandler.GetUserPlan)
	plans.Get("/usage", planHandler.GetPlanUsage)
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

	// Create default plans if they don't exist
	planRepo := repository.NewPlanRepository(db.GetDB())

	// Check if FREE plan exists
	freePlan, _ := planRepo.FindByType(domain.PlanTypeFree)
	if freePlan == nil {
		freePlan = &domain.Plan{
			Name:              "Free Plan",
			Type:              domain.PlanTypeFree,
			WalletScanLimit:   5,
			EndpointScanLimit: 5,
			Price:             0,
			IsActive:          true,
		}
		if err := planRepo.Create(freePlan); err != nil {
			return fmt.Errorf("failed to create free plan: %w", err)
		}
	}

	// Check if PREMIUM plan exists
	premiumPlan, _ := planRepo.FindByType(domain.PlanTypePremium)
	if premiumPlan == nil {
		premiumPlan = &domain.Plan{
			Name:              "Premium Plan",
			Type:              domain.PlanTypePremium,
			WalletScanLimit:   0, // Unlimited
			EndpointScanLimit: 0, // Unlimited
			Price:             29.99,
			IsActive:          false, // Coming soon
		}
		if err := planRepo.Create(premiumPlan); err != nil {
			return fmt.Errorf("failed to create premium plan: %w", err)
		}
	}

	// Assign FREE plan to existing users without a plan
	var usersWithoutPlan []domain.User
	if err := db.GetDB().Where("plan_id = ? OR plan_id IS NULL", uuid.Nil).Find(&usersWithoutPlan).Error; err == nil {
		for _, user := range usersWithoutPlan {
			if user.PlanID == uuid.Nil {
				if err := db.GetDB().Model(&user).Update("plan_id", freePlan.ID).Error; err != nil {
					// Log error but continue
					fmt.Printf("Warning: failed to assign plan to user %s: %v\n", user.ID, err)
				}
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
	if c.NATSConn != nil {
		c.NATSConn.Close()
	}
	if c.DB != nil {
		c.DB.Shutdown()
	}
	return c.App.Shutdown()
}
