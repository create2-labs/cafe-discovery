package service

import (
	"context"
	"log"
	"time"

	"cafe-discovery/pkg/nats"

	natsio "github.com/nats-io/nats.go"
)

const (
	defaultPersistenceWaitTimeout = 15 * time.Second
	defaultScannersWaitTimeout     = 30 * time.Second
	defaultEndpointsPollInterval   = 2 * time.Second
	defaultEndpointsPollTimeout    = 3 * time.Minute
)

// WaitForPersistence blocks until a message is received on persistence.ready or the context times out.
func WaitForPersistence(ctx context.Context, conn nats.Connection, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = defaultPersistenceWaitTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan struct{})
	sub, err := conn.Subscribe(nats.SubjectPersistenceReady, func(msg *natsio.Msg) {
		select {
		case done <- struct{}{}:
		default:
		}
	})
	if err != nil {
		return err
	}
	defer func() { _ = sub.Unsubscribe() }()

	// In case persistence already published before we subscribed, do a short delay and check again
	// by waiting for either message or timeout
	select {
	case <-done:
		log.Printf("✅ Persistence ready (received persistence.ready)")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// WaitForScanners blocks until both TLS and Wallet scanners have sent a heartbeat (or context timeout).
func WaitForScanners(ctx context.Context, tracker *ScannerPresenceTracker, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = defaultScannersWaitTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		if tracker.HasScanner("tls") && tracker.HasScanner("wallet") {
			log.Printf("✅ Scanners ready (TLS + Wallet)")
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
