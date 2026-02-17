package scanner

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"cafe-discovery/pkg/nats"

	natslib "github.com/nats-io/nats.go"
)

// MessageHandler is a function that processes a NATS message
type MessageHandler func(msg *natslib.Msg) error

// BaseScanner provides common functionality for all scanners
type BaseScanner struct {
	natsConn  nats.Connection
	subject   string
	handler   MessageHandler
	name      string
	isRunning bool
	mu        sync.RWMutex
}

// NewBaseScanner creates a new base scanner
func NewBaseScanner(natsConn nats.Connection, subject, name string, handler MessageHandler) *BaseScanner {
	return &BaseScanner{
		natsConn: natsConn,
		subject:  subject,
		handler:  handler,
		name:     name,
	}
}

// Start starts the scanner and subscribes to NATS messages
func (w *BaseScanner) Start(ctx context.Context) error {
	_, err := w.natsConn.QueueSubscribe(
		w.subject,
		nats.QueueScanners,
		w.handleMessage,
	)
	if err != nil {
		return err
	}

	w.mu.Lock()
	w.isRunning = true
	w.mu.Unlock()

	log.Printf("%s scanner started and subscribed to %s", w.name, w.subject)
	return nil
}

// IsRunning returns whether the scanner is currently running
func (w *BaseScanner) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.isRunning
}

// GetName returns the scanner name
func (w *BaseScanner) GetName() string {
	return w.name
}

// handleMessage processes a NATS message asynchronously so slow handlers (e.g. TLS scan)
// do not block the NATS subscription and the next message can be delivered immediately.
func (w *BaseScanner) handleMessage(msg *natslib.Msg) {
	go func() {
		if err := w.handler(msg); err != nil {
			log.Printf("Error processing message in %s scanner: %v", w.name, err)
			// In a production system, you might want to publish to a dead letter queue
		}
	}()
}

// UnmarshalMessage is a helper function to unmarshal JSON messages
// This is a generic function that works with any message type
func UnmarshalMessage(msg *natslib.Msg, v interface{}) error {
	return json.Unmarshal(msg.Data, v)
}
