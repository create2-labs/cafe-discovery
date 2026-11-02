package nats

import (
	"testing"
	"time"

	lifecyclev1 "github.com/create2-labs/cafe-contracts/discovery/scanlifecycle/v1"
	"github.com/nats-io/nats.go/jetstream"
)

func TestWalletLifecycleConsumerConfig(t *testing.T) {
	if StreamScanLifecycle != "SCAN_LIFECYCLE" {
		t.Fatal(StreamScanLifecycle)
	}
	cfg := walletLifecycleConsumerConfig()
	if cfg.Durable != "discovery-wallet-lifecycle" ||
		cfg.AckPolicy != jetstream.AckExplicitPolicy ||
		cfg.AckWait != 2*time.Minute ||
		cfg.MaxDeliver != -1 ||
		cfg.MaxAckPending != 64 ||
		cfg.DeliverPolicy != jetstream.DeliverAllPolicy {
		t.Fatalf("%+v", cfg)
	}
	want := []string{
		lifecyclev1.SubjectScanStarted,
		lifecyclev1.SubjectScanCompleted,
		lifecyclev1.SubjectScanFailed,
	}
	if len(cfg.FilterSubjects) != len(want) {
		t.Fatalf("subjects %v", cfg.FilterSubjects)
	}
	for i := range want {
		if cfg.FilterSubjects[i] != want[i] {
			t.Fatalf("subjects %v", cfg.FilterSubjects)
		}
	}
}

func TestConsumeWalletLifecycleNilConn(t *testing.T) {
	err := (&natsConnection{}).ConsumeWalletLifecycle(t.Context(), nil)
	if err == nil {
		t.Fatal("nil connection")
	}
}
