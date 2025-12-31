package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"cafe-discovery/internal/config"
	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"
	"cafe-discovery/internal/service"
	"cafe-discovery/internal/worker"
	"cafe-discovery/pkg/evm"
	"cafe-discovery/pkg/moralis"
	"cafe-discovery/pkg/nats"
	postgresdb "cafe-discovery/pkg/postgres"
	redisconn "cafe-discovery/pkg/redis"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func initLogging() {
	logLevel := viper.GetString(config.LogLevel)
	if logLevel == "" {
		logLevel = "info" // Default to info if not set
	}

	// Parse log level
	var level zerolog.Level
	switch strings.ToLower(logLevel) {
	case "trace":
		level = zerolog.TraceLevel
	case "debug":
		level = zerolog.DebugLevel
	case "info":
		level = zerolog.InfoLevel
	case "warn":
		level = zerolog.WarnLevel
	case "error":
		level = zerolog.ErrorLevel
	case "fatal":
		level = zerolog.FatalLevel
	case "panic":
		level = zerolog.PanicLevel
	default:
		level = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(level)
	
	// Use console writer for better readability
	output := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"}
	logger := zerolog.New(output).With().Timestamp().Logger()
	zerolog.DefaultContextLogger = &logger
}

func initConfig() {
	for configName, defaultValue := range config.GetDefaultConfigValues() {
		viper.SetDefault(configName, defaultValue)
	}

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Config file not found, using defaults and environment variables: %v", err)
	}
	viper.AutomaticEnv()
}

func main() {
	initConfig()
	initLogging()

	// Load chain configuration
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	cfgChain, err := config.LoadChainConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize PostgreSQL database
	db := postgresdb.New()
	if err := db.Run(); err != nil {
		log.Fatalf("Failed to initialize PostgreSQL database: %v", err)
	}
	defer db.Shutdown()

	// Run database migrations (simplified - migrations should ideally be run separately)
	if err := db.GetDB().AutoMigrate(
		&domain.Plan{},
		&domain.User{},
		&domain.ScanResultEntity{},
		&domain.TLSScanResultEntity{},
		&domain.CafeWallet{},
	); err != nil {
		log.Printf("Warning: failed to run migrations: %v", err)
	}

	// Initialize NATS connection
	natsConn, err := nats.New()
	if err != nil {
		log.Fatalf("Failed to initialize NATS: %v", err)
	}
	defer natsConn.Close()

	// Initialize Redis connection
	redisConn, err := redisconn.New()
	if err != nil {
		log.Fatalf("Failed to initialize Redis: %v", err)
	}
	defer redisConn.Close()

	// Create EVM clients for each configured blockchain
	clients := make(map[string]*evm.Client)
	for _, blockchain := range cfgChain.Blockchains {
		clients[blockchain.Name] = evm.NewClient(blockchain.RPC, blockchain.MoralisChainName)
	}

	// Initialize Moralis client
	moralisClient := moralis.NewMoralisClient(viper.GetString(config.MoralisAPIKey), viper.GetString(config.MoralisAPIURL))

	// Initialize repositories
	scanResultRepo := repository.NewScanResultRepository(db.GetDB())
	tlsScanResultRepo := repository.NewTLSScanResultRepository(db.GetDB())
	redisTLSScanRepo := repository.NewRedisTLSScanRepository(redisConn)
	userRepo := repository.NewUserRepository(db.GetDB())
	planRepo := repository.NewPlanRepository(db.GetDB())

	// Initialize plan service
	planService := service.NewPlanService(planRepo, userRepo)

	// Initialize services
	discoveryService := service.NewDiscoveryService(clients, moralisClient, scanResultRepo, planService)
	tlsService := service.NewTLSService(tlsScanResultRepo, planService)

	// Initialize workers
	walletWorker := worker.NewWalletWorker(discoveryService, natsConn)
	tlsWorker := worker.NewTLSWorker(tlsService, natsConn)
	tlsAnonymousWorker := worker.NewTLSAnonymousWorker(tlsService, redisTLSScanRepo, natsConn)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start workers
	if err := walletWorker.Start(ctx); err != nil {
		log.Fatalf("Failed to start wallet worker: %v", err)
	}

	if err := tlsWorker.Start(ctx); err != nil {
		log.Fatalf("Failed to start TLS worker: %v", err)
	}

	if err := tlsAnonymousWorker.Start(ctx); err != nil {
		log.Fatalf("Failed to start TLS anonymous worker: %v", err)
	}

	log.Println("Workers started successfully")

	// Start health check HTTP server
	healthPort := viper.GetString(config.WorkerHealthPort)

	app := fiber.New(fiber.Config{
		AppName: "Cafe Discovery Worker",
	})

	// Health check endpoint
	app.Get("/health", func(c *fiber.Ctx) error {
		// Check NATS connection
		natsConnected := natsConn.IsConnected()

		// Check workers status
		walletWorkerRunning := walletWorker.IsRunning()
		tlsWorkerRunning := tlsWorker.IsRunning()
		tlsAnonymousWorkerRunning := tlsAnonymousWorker.IsRunning()

		// Determine overall health status
		status := "ok"
		httpStatus := 200
		if !natsConnected || !walletWorkerRunning || !tlsWorkerRunning || !tlsAnonymousWorkerRunning {
			status = "degraded"
			httpStatus = 503
		}

		return c.Status(httpStatus).JSON(fiber.Map{
			"status":    status,
			"app_name":  "Cafe Discovery Worker",
			"timestamp": time.Now().Format(time.RFC3339),
			"checks": fiber.Map{
				"nats": fiber.Map{
					"connected": natsConnected,
				},
				"workers": fiber.Map{
					"wallet": fiber.Map{
						"running": walletWorkerRunning,
					},
					"tls": fiber.Map{
						"running": tlsWorkerRunning,
					},
					"tls_anonymous": fiber.Map{
						"running": tlsAnonymousWorkerRunning,
					},
				},
			},
		})
	})

	// Start health check server in a goroutine
	go func() {
		addr := "0.0.0.0:" + healthPort
		log.Printf("Starting health check server on %s", addr)
		if err := app.Listen(addr); err != nil {
			log.Printf("Failed to start health check server: %v", err)
		}
	}()

	// Setup graceful shutdown
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
	<-sigint

	log.Println("Shutting down workers...")
	cancel()

	// Shutdown health check server
	if err := app.Shutdown(); err != nil {
		log.Printf("Error shutting down health check server: %v", err)
	}
}
