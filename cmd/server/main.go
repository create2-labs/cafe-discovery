package main

import (
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"cafe-discovery/internal/app"
	"cafe-discovery/internal/config"

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
	// Set defaults first
	for configName, defaultValue := range config.GetDefaultConfigValues() {
		viper.SetDefault(configName, defaultValue)
	}

	// Configure Viper to read from CONFIG_PATH or default location
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	// Set config file path and type
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// Try to read config file (not found is acceptable, we use defaults and env vars)
	if err := viper.ReadInConfig(); err != nil {
		// Config file not found is acceptable, we use defaults and env vars
		log.Printf("Config file not found, using defaults and environment variables: %v", err)
	} else {
		log.Printf("Loaded config from: %s", viper.ConfigFileUsed())
	}

	// Enable automatic environment variable reading
	viper.AutomaticEnv()
}

func main() {
	initConfig()
	initLogging()
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
