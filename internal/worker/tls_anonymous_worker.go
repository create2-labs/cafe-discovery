package worker

import (
	"context"
	"log"

	"cafe-discovery/internal/repository"
	"cafe-discovery/internal/service"
	"cafe-discovery/pkg/nats"

	natslib "github.com/nats-io/nats.go"
)

// TLSAnonymousWorker processes anonymous TLS scan messages from NATS and stores results in Redis
type TLSAnonymousWorker struct {
	tlsService    *service.TLSService
	redisScanRepo repository.RedisTLSScanRepository
	base          *BaseWorker
}

// NewTLSAnonymousWorker creates a new anonymous TLS worker
func NewTLSAnonymousWorker(tlsService *service.TLSService, redisScanRepo repository.RedisTLSScanRepository, natsConn nats.Connection) *TLSAnonymousWorker {
	w := &TLSAnonymousWorker{
		tlsService:    tlsService,
		redisScanRepo: redisScanRepo,
	}
	handler := func(msg *natslib.Msg) error {
		return w.processAnonymousTLSScan(msg)
	}
	w.base = NewBaseWorker(natsConn, nats.SubjectTLSScanAnonymous, "TLS-Anonymous", handler)
	return w
}

// Start starts the anonymous TLS worker and subscribes to NATS messages
func (w *TLSAnonymousWorker) Start(ctx context.Context) error {
	return w.base.Start(ctx)
}

// IsRunning returns whether the anonymous TLS worker is currently running
func (w *TLSAnonymousWorker) IsRunning() bool {
	return w.base.IsRunning()
}

// processAnonymousTLSScan processes an anonymous TLS scan message
func (w *TLSAnonymousWorker) processAnonymousTLSScan(msg *natslib.Msg) error {
	var scanMsg nats.TLSScanMessage
	if err := UnmarshalMessage(msg, &scanMsg); err != nil {
		log.Printf("Failed to unmarshal anonymous TLS scan message: %v", err)
		return err
	}

	log.Printf("Processing anonymous TLS scan for endpoint %s", scanMsg.Endpoint)

	// For anonymous users, userID is uuid.Nil
	userID := &scanMsg.UserID
	result, err := w.tlsService.ScanTLS(context.Background(), userID, scanMsg.Endpoint, false)
	if err != nil {
		log.Printf("Failed to scan TLS endpoint %s (anonymous): %v", scanMsg.Endpoint, err)
		return err
	}

	// Hash the token to create a unique identifier for this anonymous session
	tokenHash := repository.HashToken(scanMsg.Token)

	// Store result in Redis with TTL, using token hash to isolate scans per anonymous session
	if err := w.redisScanRepo.Save(context.Background(), tokenHash, scanMsg.Endpoint, result); err != nil {
		log.Printf("Failed to save anonymous TLS scan result to Redis for endpoint %s: %v", scanMsg.Endpoint, err)
		return err
	}

	log.Printf("Successfully processed and stored anonymous TLS scan for endpoint %s", scanMsg.Endpoint)
	return nil
}
