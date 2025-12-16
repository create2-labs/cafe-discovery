package databases

import (
	"cafe-discovery/internal/config"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type MySQLConnection interface {
	GetDB() *gorm.DB
	IsConnected() bool
	Run() error
	Shutdown()
}

type mySQLConnection struct {
	dsn string
	db  *gorm.DB
}

func New() MySQLConnection {

	dbUser := viper.GetString(config.MySQLUser)
	dbPassword := viper.GetString(config.MySQLPassword)
	dbURL := viper.GetString(config.MySQLURL)
	dbName := viper.GetString(config.MySQLDatabase)
	return &mySQLConnection{
		dsn: fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=true&loc=UTC",
			dbUser, dbPassword, dbURL, dbName),
	}
}

func (c *mySQLConnection) GetDB() *gorm.DB {
	return c.db
}

func (c *mySQLConnection) IsConnected() bool {
	if c.db == nil {
		return false
	}

	dbSQL, errSQL := c.db.DB()
	if errSQL != nil {
		return false
	}

	if errPing := dbSQL.Ping(); errPing != nil {
		return false
	}

	return true
}

func (c *mySQLConnection) Run() error {
	// Log connection attempt (without password)
	dbUser := viper.GetString(config.MySQLUser)
	dbURL := viper.GetString(config.MySQLURL)
	dbName := viper.GetString(config.MySQLDatabase)
	log.Info().
		Str("user", dbUser).
		Str("url", dbURL).
		Str("database", dbName).
		Msg("Attempting to connect to MySQL")

	db, err := gorm.Open(mysql.Open(c.dsn), &gorm.Config{})
	if err != nil {
		log.Error().
			Err(err).
			Str("dsn", maskDSN(c.dsn)).
			Msg("Failed to connect to MySQL")
		return fmt.Errorf("failed to connect to MySQL: %w", err)
	}

	c.db = db
	log.Info().Msg("Connected to MySQL")
	return nil
}

// maskDSN masks the password in the DSN for logging
func maskDSN(dsn string) string {
	// Simple masking - replace password with ***
	// Format: user:password@tcp(host:port)/database
	parts := dsn
	if len(parts) > 0 {
		// Find @ symbol and mask everything between : and @
		for i := 0; i < len(parts); i++ {
			if parts[i] == ':' {
				for j := i + 1; j < len(parts); j++ {
					if parts[j] == '@' {
						return parts[:i+1] + "***" + parts[j:]
					}
				}
			}
		}
	}
	return "***"
}

func (c *mySQLConnection) Shutdown() {
	log.Info().Msg("Shutdown the connection to MySQL")
	dbSQL, err := c.db.DB()
	if err != nil {
		log.Error().Err(err).Msgf("Failed to shutdown database connection")
		return
	}

	if errClose := dbSQL.Close(); errClose != nil {
		log.Error().Err(errClose).Msgf("Failed to shutdown database connection")
	}
}
