package config

const (
	// Zerolog values from [trace, debug, info, warn, error, fatal, panic].
	LogLevel = "LOG_LEVEL"

	ServerHost = "SERVER_HOST"
	ServerPort = "SERVER_PORT"

	// Scanner health check configuration
	ScannerHealthPort = "SCANNER_HEALTH_PORT"

	// PostgreSQL configuration
	PostgreSQLHost = "POSTGRES_HOST"
	PostgreSQLPort = "POSTGRES_PORT"
	PostgreSQLUser = "POSTGRES_USER"
	// #nosec G101 -- This is a configuration key name, not a hardcoded credential
	PostgreSQLPassword = "POSTGRES_PASSWORD"
	PostgreSQLDatabase = "POSTGRES_DATABASE"
	PostgreSQLSSLMode  = "POSTGRES_SSLMODE"

	// NATS configuration
	NATSURL = "NATS_URL"

	// Redis configuration
	RedisURL = "REDIS_URL"

	// Boolean; used to register commands at development guild level or globally.
	Production = "PRODUCTION"

	// CORS configuration
	CORSAllowOrigins = "CORS_ALLOW_ORIGINS"
	CORSAllowMethods = "CORS_ALLOW_METHODS"

	// Cloudflare Turnstile configuration
	TurnstileSecretKey = "TURNSTILE_SECRET_KEY"
	TurnstileSiteKey   = "TURNSTILE_SITE_KEY"

	// JWT configuration
	// #nosec G101 -- This is a configuration key name, not a hardcoded credential
	JWTSecret = "JWT_SECRET"

	// Scan plugin versions (config file: scan.plugins.tls.version, scan.plugins.wallet.version)
	ScanPluginsTLSVersion    = "scan.plugins.tls.version"
	ScanPluginsWalletVersion = "scan.plugins.wallet.version"

	// Scanner type: "tls" | "wallet" | "" or "all" (both). Used when running as separate scanner processes.
	DiscoveryScannerType = "DISCOVERY_SCANNER_TYPE"

	// AUTH-05: internal scan-authorization lookup consumed by CPM (AUTH-02).
	// When DiscoveryInternalAuthzEnabled is false the endpoint replies with
	// 503 SCAN_AUTHZ_DISABLED so CPM fails closed. The static service token
	// is a temporary measure until mTLS or a signed service JWT is available.
	DiscoveryInternalAuthzEnabled = "DISCOVERY_INTERNAL_AUTHZ_ENABLED"
	// #nosec G101 -- This is a configuration key name, not a hardcoded credential
	DiscoveryInternalAuthzServiceToken = "DISCOVERY_INTERNAL_AUTHZ_SERVICE_TOKEN"

	// DiscoveryPersistenceURL is the cafe-persistence origin for internal scan/cp HTTP APIs.
	DiscoveryPersistenceURL = "DISCOVERY_PERSISTENCE_URL"
	// DiscoveryPersistenceTimeoutSec is the HTTP client timeout for persistence scan reads (default 15).
	DiscoveryPersistenceTimeoutSec = "DISCOVERY_PERSISTENCE_TIMEOUT_SEC"
	// CafePersistenceServiceToken is the bearer for internal scan/cp APIs on cafe-persistence
	// (internal/scan/v1 reads and internal/cp/v1 W1/W3 existence checks).
	// #nosec G101 -- configuration key name
	CafePersistenceServiceToken = "CAFE_PERSISTENCE_SERVICE_TOKEN"

	defaultProduction         = true
	defaultPostgreSQLHost     = "127.0.0.1"
	defaultPostgreSQLPort     = "5432"
	defaultPostgreSQLUser     = "cafe"
	defaultPostgreSQLPassword = "cafe"
	defaultPostgreSQLDatabase = "cafe"
	defaultPostgreSQLSSLMode  = "disable"
	defaultNATSURL            = "nats://localhost:4222"
	defaultRedisURL           = "redis://localhost:6379"
	defaultServerHost         = "0.0.0.0"
	defaultServerPort         = "8080"
	defaultScannerHealthPort  = "8081"
	defaultCORSAllowOrigins   = "http://localhost:3000,http://localhost:3001,http://localhost:5173"
	defaultCORSAllowMethods   = "GET,POST,PUT,DELETE,OPTIONS"
	// Cloudflare Turnstile development keys (always pass verification)
	// These are free test keys provided by Cloudflare for development
	defaultTurnstileSecretKey = "1x0000000000000000000000000000000AA"
	defaultTurnstileSiteKey   = "1x00000000000000000000AA"
	defaultScanPluginVersion  = "1.0"

	defaultDiscoveryInternalAuthzEnabled = true
)

func GetDefaultConfigValues() map[string]any {
	return map[string]any{
		PostgreSQLHost:           defaultPostgreSQLHost,
		PostgreSQLPort:           defaultPostgreSQLPort,
		PostgreSQLUser:           defaultPostgreSQLUser,
		PostgreSQLPassword:       defaultPostgreSQLPassword,
		PostgreSQLDatabase:       defaultPostgreSQLDatabase,
		PostgreSQLSSLMode:        defaultPostgreSQLSSLMode,
		NATSURL:                  defaultNATSURL,
		RedisURL:                 defaultRedisURL,
		Production:               defaultProduction,
		ServerHost:               defaultServerHost,
		ServerPort:               defaultServerPort,
		ScannerHealthPort:        defaultScannerHealthPort,
		CORSAllowOrigins:         defaultCORSAllowOrigins,
		CORSAllowMethods:         defaultCORSAllowMethods,
		TurnstileSecretKey:       defaultTurnstileSecretKey,
		TurnstileSiteKey:         defaultTurnstileSiteKey,
		ScanPluginsTLSVersion:    defaultScanPluginVersion,
		ScanPluginsWalletVersion: defaultScanPluginVersion,

		DiscoveryInternalAuthzEnabled: defaultDiscoveryInternalAuthzEnabled,
	}
}
