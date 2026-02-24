// Persistence service: single writer to Postgres and Redis for scan lifecycle events.
// Subscribes to scan.started, scan.completed, scan.failed and writes idempotently.
package main

import (
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"cafe-discovery/internal/config"
	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/persistence/handlers"
	persistenceNats "cafe-discovery/internal/persistence/nats"
	persistenceStorage "cafe-discovery/internal/persistence/storage"
	natsconn "cafe-discovery/pkg/nats"
	postgresdb "cafe-discovery/pkg/postgres"
	redisconn "cafe-discovery/pkg/redis"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func main() {
	initConfig()
	initLogging()

	// Postgres
	db := postgresdb.New()
	if err := db.Run(); err != nil {
		log.Fatal().Err(err).Msg("postgres run failed")
	}
	defer db.Shutdown()

	// Migrate only scan tables (persistence owns these).
	// Ignore "column already exists" when backend and persistence both run migrations.
	if err := db.GetDB().AutoMigrate(&domain.TLSScanResultEntity{}, &domain.ScanResultEntity{}); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			log.Warn().Err(err).Msg("scan tables: some columns already exist (idempotent migration)")
		} else {
			log.Fatal().Err(err).Msg("scan tables migration failed")
		}
	}
	// Unique constraints for upsert-by-(user_id, url) and (user_id, address). PostgreSQL 15+ for NULLS NOT DISTINCT.
	for _, q := range []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_tls_scan_results_user_url ON tls_scan_results (user_id, url) NULLS NOT DISTINCT`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_scan_results_user_address ON scan_results (user_id, address)`,
	} {
		if err := db.GetDB().Exec(q).Error; err != nil {
			log.Warn().Err(err).Str("sql", q).Msg("unique index creation (may already exist)")
		}
	}

	// Redis
	redis, err := redisconn.New()
	if err != nil {
		log.Fatal().Err(err).Msg("redis connect failed")
	}
	defer func() {
		if err := redis.Close(); err != nil {
			log.Warn().Err(err).Msg("redis close failed")
		}
	}()

	// NATS
	nc, err := natsconn.New()
	if err != nil {
		log.Fatal().Err(err).Msg("nats connect failed")
	}
	defer nc.Close()

	// Storage and handlers
	tlsWriter := persistenceStorage.NewTLSWriter(db.GetDB())
	walletWriter := persistenceStorage.NewWalletWriter(db.GetDB())
	cache := persistenceStorage.NewRedisCache(redis)
	scanHandler := handlers.NewScanEventHandler(tlsWriter, walletWriter, cache, nc)

	subs, err := persistenceNats.SubscribeScanEvents(nc, scanHandler)
	if err != nil {
		log.Fatal().Err(err).Msg("subscribe scan events failed")
	}
	defer func() {
		for _, sub := range subs {
			_ = sub.Unsubscribe()
		}
	}()

	// Signal to backend that persistence is ready; repeat for a while so backend can catch it when it starts after us
	go func() {
		payload := []byte("{}")
		for i := 0; i < 40; i++ {
			if err := nc.Publish(natsconn.SubjectPersistenceReady, payload); err != nil {
				log.Warn().Err(err).Msg("persistence.ready publish failed")
			}
			time.Sleep(3 * time.Second)
		}
	}()
	log.Info().Msg("persistence.ready will be published every 3s for 1 minute")

	log.Info().Msg("persistence-service running (scan.started / scan.completed / scan.failed)")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	log.Info().Msg("shutting down persistence-service")
}

func initConfig() {
	for k, v := range config.GetDefaultConfigValues() {
		viper.SetDefault(k, v)
	}
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")
	_ = viper.ReadInConfig()
	viper.AutomaticEnv()
}

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
	log.Logger = zerolog.New(output).With().Timestamp().Logger()
}
