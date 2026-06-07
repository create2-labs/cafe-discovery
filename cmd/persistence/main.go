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
	"cafe-discovery/internal/persistence/planlimit"
	persistenceNats "cafe-discovery/internal/persistence/nats"
	persistenceStorage "cafe-discovery/internal/persistence/storage"
	"cafe-discovery/internal/repository"
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

	// Scan tables (persistence owns these). Dev: reset Postgres volume if schema changes brutally.
	if err := db.GetDB().AutoMigrate(
		&domain.TLSScanResultEntity{},
		&domain.ScanResultEntity{},
		&domain.ScanUsageEventEntity{},
	); err != nil {
		log.Fatal().Err(err).Msg("scan tables AutoMigrate failed")
	}
	// IMM-2: multiple rows per (user_id, address|url); list indexes for history queries.
	// IMM-6b-1: ledger index for plan quota counters by user and scan kind.
	for _, q := range []string{
		`DROP INDEX IF EXISTS idx_scan_results_user_address`,
		`DROP INDEX IF EXISTS idx_tls_scan_results_user_url`,
		`CREATE INDEX IF NOT EXISTS idx_scan_results_user_address_created_at ON scan_results (user_id, address, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_tls_scan_results_user_url_created_at ON tls_scan_results (user_id, url, created_at DESC) NULLS NOT DISTINCT`,
		`CREATE INDEX IF NOT EXISTS idx_scan_usage_events_user_kind ON scan_usage_events (user_id, scan_kind)`,
		// IMM-D2: status must not default to RUNNING; OnStarted sets RUNNING on scan.started.
		`ALTER TABLE scan_results ALTER COLUMN status DROP DEFAULT`,
		`ALTER TABLE tls_scan_results ALTER COLUMN status DROP DEFAULT`,
	} {
		if err := db.GetDB().Exec(q).Error; err != nil {
			log.Fatal().Err(err).Str("sql", q).Msg("scan history indexes failed")
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

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}
	cfgChain, err := config.LoadChainConfig(configPath)
	if err != nil {
		log.Fatal().Err(err).Str("path", configPath).Msg("chain config load failed (need blockchains[].chain_id)")
	}

	// Storage and handlers
	tlsWriter := persistenceStorage.NewTLSWriter(db.GetDB())
	walletWriter := persistenceStorage.NewWalletWriter(db.GetDB())
	cache := persistenceStorage.NewRedisCache(redis)
	ledgerRepo := repository.NewScanUsageLedgerRepository(db.GetDB())
	planLimits := planlimit.NewResolver(
		repository.NewUserRepository(db.GetDB()),
		repository.NewPlanRepository(db.GetDB()),
	)
	scanHandler := handlers.NewScanEventHandler(
		tlsWriter, walletWriter, cache, nc, cfgChain.ChainIDByNetwork(),
		db.GetDB(), ledgerRepo, planLimits,
	)

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
