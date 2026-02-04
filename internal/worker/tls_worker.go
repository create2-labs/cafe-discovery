package worker

import (
	"context"
	"log"

	"cafe-discovery/internal/service"
	"cafe-discovery/pkg/nats"

	natslib "github.com/nats-io/nats.go"
)

// maxConcurrentTLSScans limits how many TLS scans run at once (each does network I/O + optional OpenSSL).
const maxConcurrentTLSScans = 5

// TLSWorker processes TLS scan messages from NATS
type TLSWorker struct {
	tlsService *service.TLSService
	base       *BaseWorker
	sem        chan struct{} // semaphore to limit concurrent scans
}

// NewTLSWorker creates a new TLS worker
func NewTLSWorker(tlsService *service.TLSService, natsConn nats.Connection) *TLSWorker {
	sem := make(chan struct{}, maxConcurrentTLSScans)
	w := &TLSWorker{tlsService: tlsService, sem: sem}
	handler := func(msg *natslib.Msg) error {
		return w.processTLSScan(msg)
	}
	w.base = NewBaseWorker(natsConn, nats.SubjectTLSScan, "TLS", handler)
	return w
}

// Start starts the TLS worker and subscribes to NATS messages
func (w *TLSWorker) Start(ctx context.Context) error {
	return w.base.Start(ctx)
}

// IsRunning returns whether the TLS worker is currently running
func (w *TLSWorker) IsRunning() bool {
	return w.base.IsRunning()
}

// processTLSScan processes a TLS scan message
func (w *TLSWorker) processTLSScan(msg *natslib.Msg) error {
	var scanMsg nats.TLSScanMessage
	if err := UnmarshalMessage(msg, &scanMsg); err != nil {
		log.Printf("Failed to unmarshal TLS scan message: %v", err)
		return err
	}

	// Limit concurrent scans so we don't exhaust connections or overload targets
	w.sem <- struct{}{}
	defer func() { <-w.sem }()

	log.Printf("Processing TLS scan for user %s, endpoint %s", scanMsg.UserID, scanMsg.Endpoint)

	// Pass pointer to userID for user-scanned endpoints
	userID := &scanMsg.UserID
	_, err := w.tlsService.ScanTLS(context.Background(), userID, scanMsg.Endpoint, false) // false = user-scanned endpoint
	if err != nil {
		log.Printf("Failed to scan TLS endpoint %s for user %s: %v", scanMsg.Endpoint, scanMsg.UserID, err)
		return err
	}

	log.Printf("Successfully processed TLS scan for endpoint %s", scanMsg.Endpoint)
	return nil
}
