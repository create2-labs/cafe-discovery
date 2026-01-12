package worker

import (
	"context"
	"log"

	"cafe-discovery/internal/repository"
	"cafe-discovery/internal/service"
	"cafe-discovery/pkg/nats"

	natslib "github.com/nats-io/nats.go"
)

// WalletAnonymousWorker processes anonymous wallet scan messages from NATS and stores results in Redis
type WalletAnonymousWorker struct {
	discoveryService *service.DiscoveryService
	redisScanRepo    repository.RedisWalletScanRepository
	base             *BaseWorker
}

// NewWalletAnonymousWorker creates a new anonymous wallet worker
func NewWalletAnonymousWorker(discoveryService *service.DiscoveryService, redisScanRepo repository.RedisWalletScanRepository, natsConn nats.Connection) *WalletAnonymousWorker {
	w := &WalletAnonymousWorker{
		discoveryService: discoveryService,
		redisScanRepo:    redisScanRepo,
	}
	handler := func(msg *natslib.Msg) error {
		return w.processAnonymousWalletScan(msg)
	}
	w.base = NewBaseWorker(natsConn, nats.SubjectWalletScanAnonymous, "Wallet-Anonymous", handler)
	return w
}

// Start starts the anonymous wallet worker and subscribes to NATS messages
func (w *WalletAnonymousWorker) Start(ctx context.Context) error {
	return w.base.Start(ctx)
}

// IsRunning returns whether the anonymous wallet worker is currently running
func (w *WalletAnonymousWorker) IsRunning() bool {
	return w.base.IsRunning()
}

// processAnonymousWalletScan processes an anonymous wallet scan message
func (w *WalletAnonymousWorker) processAnonymousWalletScan(msg *natslib.Msg) error {
	var scanMsg nats.WalletScanMessage
	if err := UnmarshalMessage(msg, &scanMsg); err != nil {
		log.Printf("Failed to unmarshal anonymous wallet scan message: %v", err)
		return err
	}

	log.Printf("Processing anonymous wallet scan for address %s", scanMsg.Address)

	// For anonymous users, userID is uuid.Nil
	result, err := w.discoveryService.ScanWallet(context.Background(), scanMsg.UserID, scanMsg.Address)
	if err != nil {
		log.Printf("Failed to scan wallet %s (anonymous): %v", scanMsg.Address, err)
		return err
	}

	// Hash the token to create a unique identifier for this anonymous session
	tokenHash := repository.HashToken(scanMsg.Token)

	// Store result in Redis with TTL, using token hash to isolate scans per anonymous session
	if err := w.redisScanRepo.Save(context.Background(), tokenHash, scanMsg.Address, result); err != nil {
		log.Printf("Failed to save anonymous wallet scan result to Redis for address %s: %v", scanMsg.Address, err)
		return err
	}

	log.Printf("Successfully processed and stored anonymous wallet scan for address %s", scanMsg.Address)
	return nil
}
