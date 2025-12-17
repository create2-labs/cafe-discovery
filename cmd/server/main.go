package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"cafe-discovery/internal/app"
	"cafe-discovery/internal/config"

	"github.com/spf13/viper"
)

func initConfig() {

	for configName, defaultValue := range config.GetDefaultConfigValues() {
		viper.SetDefault(configName, defaultValue)
	}

	if err := viper.ReadInConfig(); err != nil {
		// Config file not found is acceptable, we use defaults and env vars
		log.Printf("Config file not found, using defaults and environment variables: %v", err)
	}
	viper.AutomaticEnv()
}

func main() {
	initConfig()
	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	cfgChain, err := config.LoadChainConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize application container
	container, err := app.NewContainer(cfgChain)
	if err != nil {
		log.Fatalf("Failed to initialize container: %v", err)
	}

	// Setup graceful shutdown
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		log.Println("Shutting down server...")
		if err := container.Shutdown(); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
	}()

	// Start server
	log.Printf("Starting server on %s:%s", viper.GetString(config.ServerHost), viper.GetString(config.ServerPort))
	if err := container.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
