package config

const (
	// Zerolog values from [trace, debug, info, warn, error, fatal, panic].
	LogLevel = "LOG_LEVEL"

	ServerHost = "SERVER_HOST"
	ServerPort = "SERVER_PORT"

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

	// CORS configuration
	CORSAllowOrigins = "CORS_ALLOW_ORIGINS"
	CORSAllowMethods = "CORS_ALLOW_METHODS"

	// Cloudflare Turnstile configuration
	TurnstileSecretKey = "TURNSTILE_SECRET_KEY"

	// JWT configuration
	// #nosec G101 -- This is a configuration key name, not a hardcoded credential
	JWTSecret = "JWT_SECRET"

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

	defaultPostgreSQLHost     = "127.0.0.1"
	defaultPostgreSQLPort     = "5432"
	defaultPostgreSQLUser     = "cafe"
	defaultPostgreSQLDatabase = "cafe"
	defaultPostgreSQLSSLMode  = "disable"
	defaultNATSURL            = "nats://localhost:4222"
	defaultRedisURL           = "redis://localhost:6379"
	defaultServerHost         = "0.0.0.0"
	defaultServerPort         = "8080"
	defaultCORSAllowOrigins   = "http://localhost:3000,http://localhost:3001,http://localhost:5173"
	defaultCORSAllowMethods   = "GET,POST,PUT,DELETE,OPTIONS"

	defaultDiscoveryInternalAuthzEnabled = true
)

func GetDefaultConfigValues() map[string]any {
	return map[string]any{
		PostgreSQLHost:     defaultPostgreSQLHost,
		PostgreSQLPort:     defaultPostgreSQLPort,
		PostgreSQLUser:     defaultPostgreSQLUser,
		PostgreSQLDatabase: defaultPostgreSQLDatabase,
		PostgreSQLSSLMode:  defaultPostgreSQLSSLMode,
		NATSURL:            defaultNATSURL,
		RedisURL:           defaultRedisURL,
		ServerHost:         defaultServerHost,
		ServerPort:         defaultServerPort,
		CORSAllowOrigins:   defaultCORSAllowOrigins,
		CORSAllowMethods:   defaultCORSAllowMethods,

		DiscoveryInternalAuthzEnabled: defaultDiscoveryInternalAuthzEnabled,
	}
}
