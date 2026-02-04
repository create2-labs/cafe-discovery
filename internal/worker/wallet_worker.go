package worker

import (
	"context"
	"log"

	"cafe-discovery/internal/service"
	"cafe-discovery/pkg/nats"

	natslib "github.com/nats-io/nats.go"
)

// maxConcurrentWalletScans limits how many wallet scans run at once (each does RPC calls to multiple networks).
const maxConcurrentWalletScans = 5

// WalletWorker processes wallet scan messages from NATS
type WalletWorker struct {
	discoveryService *service.DiscoveryService
	base             *BaseWorker
	sem              chan struct{} // semaphore to limit concurrent scans
}

// NewWalletWorker creates a new wallet worker
func NewWalletWorker(discoveryService *service.DiscoveryService, natsConn nats.Connection) *WalletWorker {
	sem := make(chan struct{}, maxConcurrentWalletScans)
	w := &WalletWorker{discoveryService: discoveryService, sem: sem}
	handler := w.createMessageHandler()
	w.base = NewBaseWorker(natsConn, nats.SubjectWalletScan, "Wallet", handler)
	return w
}

// Start starts the wallet worker and subscribes to NATS messages
func (w *WalletWorker) Start(ctx context.Context) error {
	return w.base.Start(ctx)
}

// IsRunning returns whether the wallet worker is currently running
func (w *WalletWorker) IsRunning() bool {
	return w.base.IsRunning()
}

// createMessageHandler creates a message handler for wallet scans
func (w *WalletWorker) createMessageHandler() MessageHandler {
	return func(msg *natslib.Msg) error {
		var scanMsg nats.WalletScanMessage
		if err := UnmarshalMessage(msg, &scanMsg); err != nil {
			log.Printf("Failed to unmarshal wallet scan message: %v", err)
			return err
		}

		// Limit concurrent scans so we don't exhaust RPC connections or overload providers
		w.sem <- struct{}{}
		defer func() { <-w.sem }()

		log.Printf("Processing wallet scan for user %s, address %s", scanMsg.UserID, scanMsg.Address)

		_, err := w.discoveryService.ScanWallet(context.Background(), scanMsg.UserID, scanMsg.Address)
		if err != nil {
			log.Printf("Failed to scan wallet %s for user %s: %v", scanMsg.Address, scanMsg.UserID, err)
			return err
		}

		log.Printf("Successfully processed wallet scan for address %s", scanMsg.Address)
		return nil
	}
}
