package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"cafe-discovery/internal/config"
	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"
	"cafe-discovery/internal/service"
	"cafe-discovery/internal/worker"
	"cafe-discovery/pkg/evm"
	"cafe-discovery/pkg/moralis"
	"cafe-discovery/pkg/nats"
	postgresdb "cafe-discovery/pkg/postgres"

	"github.com/spf13/viper"
)

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

	log.Println("Workers started successfully")

	// Setup graceful shutdown
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
	<-sigint

	log.Println("Shutting down workers...")
	cancel()
}
