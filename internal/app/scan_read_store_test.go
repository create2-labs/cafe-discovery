package app

import (
	"testing"

	"cafe-discovery/internal/config"

	"github.com/spf13/viper"
)

func TestNewScanReadStoreRequiresURLAndToken(t *testing.T) {
	t.Setenv(config.DiscoveryPersistenceURL, "")
	t.Setenv(config.CafePersistenceServiceToken, "token")
	viper.Set(config.DiscoveryPersistenceURL, "")
	viper.Set(config.CafePersistenceServiceToken, "token")

	if _, err := newScanReadStore(); err == nil {
		t.Fatal("expected error when DISCOVERY_PERSISTENCE_URL is empty")
	}

	t.Setenv(config.DiscoveryPersistenceURL, "http://persistence:8082")
	t.Setenv(config.CafePersistenceServiceToken, "")
	viper.Set(config.DiscoveryPersistenceURL, "http://persistence:8082")
	viper.Set(config.CafePersistenceServiceToken, "")

	if _, err := newScanReadStore(); err == nil {
		t.Fatal("expected error when CAFE_PERSISTENCE_SERVICE_TOKEN is empty")
	}
}

func TestNewScanReadStoreOK(t *testing.T) {
	viper.Set(config.DiscoveryPersistenceURL, "http://persistence:8082")
	viper.Set(config.CafePersistenceServiceToken, "secret")
	viper.Set(config.DiscoveryPersistenceTimeoutSec, 12)

	store, err := newScanReadStore()
	if err != nil {
		t.Fatalf("newScanReadStore: %v", err)
	}
	if store == nil {
		t.Fatal("expected store")
	}
}
