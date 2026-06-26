package app

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"cafe-discovery/internal/config"
	"cafe-discovery/internal/persistence/cphttp"
	"cafe-discovery/internal/persistence/scanhttp"
	"cafe-discovery/internal/policyref"

	"github.com/spf13/viper"
)

func loadPersistenceHTTPConfig() (scanhttp.Config, error) {
	persistenceURL := strings.TrimSpace(viper.GetString(config.DiscoveryPersistenceURL))
	token := strings.TrimSpace(viper.GetString(config.CafePersistenceServiceToken))
	if persistenceURL == "" {
		return scanhttp.Config{}, fmt.Errorf("%s is required", config.DiscoveryPersistenceURL)
	}
	if token == "" {
		return scanhttp.Config{}, fmt.Errorf("%s is required", config.CafePersistenceServiceToken)
	}
	timeoutSec := viper.GetInt(config.DiscoveryPersistenceTimeoutSec)
	if timeoutSec <= 0 {
		timeoutSec = 15
	}
	return scanhttp.Config{
		BaseURL:    persistenceURL,
		Token:      token,
		HTTPClient: &http.Client{Timeout: time.Duration(timeoutSec) * time.Second},
	}, nil
}

func newScanPersistenceClient() (*scanhttp.Client, error) {
	cfg, err := loadPersistenceHTTPConfig()
	if err != nil {
		return nil, err
	}
	return scanhttp.NewClient(cfg), nil
}

func newPolicyReferenceChecker() (policyref.Checker, error) {
	cfg, err := loadPersistenceHTTPConfig()
	if err != nil {
		return nil, err
	}
	return cphttp.NewClient(cphttp.Config{
		BaseURL:    cfg.BaseURL,
		Token:      cfg.Token,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}), nil
}
