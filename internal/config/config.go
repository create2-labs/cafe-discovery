package config

const (
	// Zerolog values from [trace, debug, info, warn, error, fatal, panic].
	LogLevel = "LOG_LEVEL"

	ServerHost = "SERVER_HOST"
	ServerPort = "SERVER_PORT"

	// MySQL URL with the following format: HOST:PORT.
	MySQLURL = "MYSQL_URL"

	// MySQL user.
	MySQLUser = "MYSQL_USER"

	// MySQL password.
	MySQLPassword = "MYSQL_PASSWORD"

	// MySQL database name.
	MySQLDatabase = "MYSQL_DATABASE"

	// Boolean; used to register commands at development guild level or globally.
	Production = "PRODUCTION"

	// Moralis API key.
	// #nosec G101 -- This is a configuration key name, not a hardcoded credential
	MoralisAPIKey = "MORALIS_API_KEY"

	// Moralis API URL.
	MoralisAPIURL = "MORALIS_API_URL"

	defaultProduction    = true
	defaultMySQLURL      = "127.0.0.1:3306"
	defaultMySQLUser     = "cafe"
	defaultMySQLPassword = "cafe"
	defaultMySQLDatabase = "cafe"
	defaultMoralisAPIKey = ""
	defaultMoralisAPIURL = "https://deep-index.moralis.io"
	defaultServerHost    = "0.0.0.0"
	defaultServerPort    = "8080"
)

func GetDefaultConfigValues() map[string]any {
	return map[string]any{
		MySQLURL:      defaultMySQLURL,
		MySQLUser:     defaultMySQLUser,
		MySQLPassword: defaultMySQLPassword,
		MySQLDatabase: defaultMySQLDatabase,
		Production:    defaultProduction,
		ServerHost:    defaultServerHost,
		ServerPort:    defaultServerPort,
		MoralisAPIKey: defaultMoralisAPIKey,
		MoralisAPIURL: defaultMoralisAPIURL,
	}
}
