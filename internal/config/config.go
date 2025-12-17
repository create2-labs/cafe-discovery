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

	// Boolean; used to register commands at development guild level or globally.
	Production = "PRODUCTION"

	// Moralis API key.
	// #nosec G101 -- This is a configuration key name, not a hardcoded credential
	MoralisAPIKey = "MORALIS_API_KEY"

	// Moralis API URL.
	MoralisAPIURL = "MORALIS_API_URL"

	defaultProduction         = true
	defaultPostgreSQLHost     = "127.0.0.1"
	defaultPostgreSQLPort     = "5432"
	defaultPostgreSQLUser     = "cafe"
	defaultPostgreSQLPassword = "cafe"
	defaultPostgreSQLDatabase = "cafe"
	defaultPostgreSQLSSLMode  = "disable"
	defaultNATSURL            = "nats://localhost:4222"
	defaultMoralisAPIKey      = ""
	defaultMoralisAPIURL      = "https://deep-index.moralis.io"
	defaultServerHost         = "0.0.0.0"
	defaultServerPort         = "8080"
)

func GetDefaultConfigValues() map[string]any {
	return map[string]any{
		PostgreSQLHost:     defaultPostgreSQLHost,
		PostgreSQLPort:     defaultPostgreSQLPort,
		PostgreSQLUser:     defaultPostgreSQLUser,
		PostgreSQLPassword: defaultPostgreSQLPassword,
		PostgreSQLDatabase: defaultPostgreSQLDatabase,
		PostgreSQLSSLMode:  defaultPostgreSQLSSLMode,
		NATSURL:            defaultNATSURL,
		Production:         defaultProduction,
		ServerHost:         defaultServerHost,
		ServerPort:         defaultServerPort,
		MoralisAPIKey:      defaultMoralisAPIKey,
		MoralisAPIURL:      defaultMoralisAPIURL,
	}
}
