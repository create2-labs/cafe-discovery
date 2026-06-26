package app

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"cafe-discovery/internal/config"
	"cafe-discovery/internal/persistence/scanhttp"
	"cafe-discovery/internal/persistence/scanread"

	"github.com/spf13/viper"
)

func newScanReadStore() (scanread.Store, error) {
	persistenceURL := strings.TrimSpace(viper.GetString(config.DiscoveryPersistenceURL))
	token := strings.TrimSpace(viper.GetString(config.CafePersistenceServiceToken))
	if persistenceURL == "" {
		return nil, fmt.Errorf("%s is required (PERS-D6a-read)", config.DiscoveryPersistenceURL)
	}
	if token == "" {
		return nil, fmt.Errorf("%s is required (PERS-D6a-read)", config.CafePersistenceServiceToken)
	}
	timeoutSec := viper.GetInt(config.DiscoveryPersistenceTimeoutSec)
	if timeoutSec <= 0 {
		timeoutSec = 15
	}
	return scanhttp.NewClient(scanhttp.Config{
		BaseURL:    persistenceURL,
		Token:      token,
		HTTPClient: &http.Client{Timeout: time.Duration(timeoutSec) * time.Second},
	}), nil
}
