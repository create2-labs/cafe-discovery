package worker

import (
	"context"
	"log"

	"cafe-discovery/internal/service"
	"cafe-discovery/pkg/nats"

	natslib "github.com/nats-io/nats.go"
)

// TLSWorker processes TLS scan messages from NATS
type TLSWorker struct {
	tlsService *service.TLSService
	base       *BaseWorker
}

// NewTLSWorker creates a new TLS worker
func NewTLSWorker(tlsService *service.TLSService, natsConn nats.Connection) *TLSWorker {
	w := &TLSWorker{tlsService: tlsService}
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

	log.Printf("Processing TLS scan for user %s, endpoint %s", scanMsg.UserID, scanMsg.Endpoint)

	_, err := w.tlsService.ScanTLS(context.Background(), scanMsg.UserID, scanMsg.Endpoint)
	if err != nil {
		log.Printf("Failed to scan TLS endpoint %s for user %s: %v", scanMsg.Endpoint, scanMsg.UserID, err)
		return err
	}

	log.Printf("Successfully processed TLS scan for endpoint %s", scanMsg.Endpoint)
	return nil
}
