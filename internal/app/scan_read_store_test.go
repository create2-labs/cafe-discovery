package app

import (
	"testing"

	"cafe-discovery/internal/config"

	"github.com/spf13/viper"
)

func TestNewScanPersistenceClientRequiresURLAndToken(t *testing.T) {
	t.Setenv(config.DiscoveryPersistenceURL, "")
	t.Setenv(config.CafePersistenceServiceToken, "token")
	viper.Set(config.DiscoveryPersistenceURL, "")
	viper.Set(config.CafePersistenceServiceToken, "token")

	if _, err := newScanPersistenceClient(); err == nil {
		t.Fatal("expected error when DISCOVERY_PERSISTENCE_URL is empty")
	}

	t.Setenv(config.DiscoveryPersistenceURL, "http://persistence:8082")
	t.Setenv(config.CafePersistenceServiceToken, "")
	viper.Set(config.DiscoveryPersistenceURL, "http://persistence:8082")
	viper.Set(config.CafePersistenceServiceToken, "")

	if _, err := newScanPersistenceClient(); err == nil {
		t.Fatal("expected error when CAFE_PERSISTENCE_SERVICE_TOKEN is empty")
	}
}

func TestNewScanPersistenceClientOK(t *testing.T) {
	viper.Set(config.DiscoveryPersistenceURL, "http://persistence:8082")
	viper.Set(config.CafePersistenceServiceToken, "secret")
	viper.Set(config.DiscoveryPersistenceTimeoutSec, 12)

	client, err := newScanPersistenceClient()
	if err != nil {
		t.Fatalf("newScanPersistenceClient: %v", err)
	}
	if client == nil {
		t.Fatal("expected client")
	}
}

func TestNewPolicyReferenceCheckerOK(t *testing.T) {
	viper.Set(config.DiscoveryPersistenceURL, "http://persistence:8082")
	viper.Set(config.CafePersistenceServiceToken, "secret")

	checker, err := newPolicyReferenceChecker()
	if err != nil {
		t.Fatalf("newPolicyReferenceChecker: %v", err)
	}
	if checker == nil {
		t.Fatal("expected checker")
	}
}
