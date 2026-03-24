package main

import (
	"context"
	"log"
	"os"
	"strings"

	"cafe-discovery/internal/config"
	"cafe-discovery/internal/scanner/core"
	"cafe-discovery/internal/scanner/tlsrunner"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

func initLogging() {
	logLevel := viper.GetString(config.LogLevel)
	if logLevel == "" {
		logLevel = "info"
	}
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
	output := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"}
	logger := zerolog.New(output).With().Timestamp().Logger()
	zerolog.DefaultContextLogger = &logger
}

func initConfig() {
	for configName, defaultValue := range config.GetDefaultConfigValues() {
		viper.SetDefault(configName, defaultValue)
	}
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Config file not found, using defaults and environment variables: %v", err)
	} else {
		log.Printf("Loaded config from: %s", viper.ConfigFileUsed())
	}
	viper.AutomaticEnv()
}

func main() {
	initConfig()
	initLogging()

	deps, err := core.Setup("tls")
	if err != nil {
		log.Fatalf("Setup failed: %v", err)
	}
	defer func() {
		if deps.NATS != nil {
			deps.NATS.Close()
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runners := []core.Runner{tlsrunner.Runner{}}
	if err := core.Run(ctx, cancel, deps, runners); err != nil {
		log.Fatalf("Run failed: %v", err)
	}
}
