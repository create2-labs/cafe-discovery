package worker

import (
	"context"
	"encoding/json"
	"log"

	"cafe-discovery/pkg/nats"

	natslib "github.com/nats-io/nats.go"
)

// MessageHandler is a function that processes a NATS message
type MessageHandler func(msg *natslib.Msg) error

// BaseWorker provides common functionality for all workers
type BaseWorker struct {
	natsConn nats.Connection
	subject  string
	handler  MessageHandler
	name     string
}

// NewBaseWorker creates a new base worker
func NewBaseWorker(natsConn nats.Connection, subject, name string, handler MessageHandler) *BaseWorker {
	return &BaseWorker{
		natsConn: natsConn,
		subject:  subject,
		handler:  handler,
		name:     name,
	}
}

// Start starts the worker and subscribes to NATS messages
func (w *BaseWorker) Start(ctx context.Context) error {
	_, err := w.natsConn.QueueSubscribe(
		w.subject,
		nats.QueueWorkers,
		w.handleMessage,
	)
	if err != nil {
		return err
	}

	log.Printf("%s worker started and subscribed to %s", w.name, w.subject)
	return nil
}

// handleMessage processes a NATS message
func (w *BaseWorker) handleMessage(msg *natslib.Msg) {
	if err := w.handler(msg); err != nil {
		log.Printf("Error processing message in %s worker: %v", w.name, err)
		// In a production system, you might want to publish to a dead letter queue
	}
}

// UnmarshalMessage is a helper function to unmarshal JSON messages
// This is a generic function that works with any message type
func UnmarshalMessage(msg *natslib.Msg, v interface{}) error {
	return json.Unmarshal(msg.Data, v)
}
