package nats

import (
	"cafe-discovery/internal/config"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// Connection wraps NATS connection
type Connection interface {
	Publish(subject string, data []byte) error
	PublishJetStream(subject string, data []byte) error
	ListStreamPayloads(ctx context.Context) ([][]byte, error)
	Subscribe(subject string, handler func(msg *nats.Msg)) (*nats.Subscription, error)
	QueueSubscribe(subject, queue string, handler func(msg *nats.Msg)) (*nats.Subscription, error)
	Close()
	IsConnected() bool
}

type natsConnection struct {
	conn *nats.Conn
	jsMu sync.Mutex
	js   jetstream.JetStream
}

// New creates a new NATS connection
func New() (Connection, error) {
	natsURL := viper.GetString(config.NATSURL)
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	log.Info().Str("url", natsURL).Msg("Connecting to NATS")

	conn, err := nats.Connect(natsURL,
		nats.Name("cafe-discovery"),
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(10),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				log.Warn().Err(err).Msg("NATS disconnected")
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Info().Msg("NATS reconnected")
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			log.Info().Msg("NATS connection closed")
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	log.Info().Msg("Connected to NATS")

	return &natsConnection{conn: conn}, nil
}

func (nc *natsConnection) Publish(subject string, data []byte) error {
	return nc.conn.Publish(subject, data)
}

func (nc *natsConnection) jet() (jetstream.JetStream, error) {
	nc.jsMu.Lock()
	defer nc.jsMu.Unlock()
	if nc.js != nil {
		return nc.js, nil
	}
	if nc.conn == nil {
		return nil, errors.New("nats connection is nil")
	}
	js, err := jetstream.New(nc.conn)
	if err != nil {
		return nil, err
	}
	nc.js = js
	return js, nil
}

// PublishJetStream publishes into the stream that captures subject and waits for the stream ack.
func (nc *natsConnection) PublishJetStream(subject string, data []byte) error {
	js, err := nc.jet()
	if err != nil {
		return fmt.Errorf("jetstream: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := js.Publish(ctx, subject, data); err != nil {
		return fmt.Errorf("jetstream publish %s: %w", subject, err)
	}
	return nil
}

// ListStreamPayloads returns the bodies still held by the scan request work queue.
func (nc *natsConnection) ListStreamPayloads(ctx context.Context) ([][]byte, error) {
	js, err := nc.jet()
	if err != nil {
		return nil, err
	}
	stream, err := js.Stream(ctx, StreamScanRequested)
	if err != nil {
		return nil, err
	}
	info, err := stream.Info(ctx)
	if err != nil {
		return nil, err
	}
	if info.State.Msgs == 0 {
		return nil, nil
	}
	out := make([][]byte, 0, info.State.Msgs)
	for seq := info.State.FirstSeq; seq <= info.State.LastSeq; seq++ {
		msg, err := stream.GetMsg(ctx, seq)
		if err != nil {
			if errors.Is(err, jetstream.ErrMsgNotFound) {
				continue
			}
			return nil, err
		}
		out = append(out, append([]byte(nil), msg.Data...))
	}
	return out, nil
}

func (nc *natsConnection) Subscribe(subject string, handler func(msg *nats.Msg)) (*nats.Subscription, error) {
	return nc.conn.Subscribe(subject, handler)
}

func (nc *natsConnection) QueueSubscribe(subject, queue string, handler func(msg *nats.Msg)) (*nats.Subscription, error) {
	return nc.conn.QueueSubscribe(subject, queue, handler)
}

func (nc *natsConnection) Close() {
	if nc.conn != nil {
		nc.conn.Close()
	}
}

func (nc *natsConnection) IsConnected() bool {
	return nc.conn != nil && nc.conn.IsConnected()
}

// PublishJSON publishes a JSON message
func PublishJSON(conn Connection, subject string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return conn.Publish(subject, data)
}

// PublishJSONJetStream publishes a JSON message and returns when the stream has stored it.
func PublishJSONJetStream(conn Connection, subject string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	return conn.PublishJetStream(subject, data)
}

// Subjects for NATS messaging
const (
	SubjectWalletScan      = "cafe.discovery.wallet.scan"
	SubjectTLSScan         = "cafe.discovery.tls.scan"
	SubjectScannerPresence = "cafe.discovery.scanners.presence"
	QueueScanners          = "cafe.scanners"

	// Event subjects for persistence service (scan lifecycle)
	SubjectScanRequestedTLS    = "scan.requested.tls"
	SubjectScanRequestedWallet = "scan.requested.wallet"
	// StreamScanRequested is the work queue that captures the two scan.requested subjects.
	StreamScanRequested  = "SCAN_REQUESTED"
	SubjectScanStarted   = "scan.started"
	SubjectScanCompleted = "scan.completed"
	SubjectScanFailed    = "scan.failed"
	SubjectScanReady     = "scan.ready" // published by persistence after writing to Redis/Postgres so API can return result on GET
	// SubjectDiscoveryWalletObserved carries JSON matching cafe-contracts cafe.discovery.wallet.observed v0.1 (observation/informational).
	// CPM must not use this stream as the canonical assessment trigger (v0.7: use policy.assessment.requested).
	SubjectDiscoveryWalletObserved = "cafe.discovery.events.wallet.observed.v0_1"
	SubjectScannerHeartbeatTLS     = "scanner.heartbeat.tls"
	SubjectScannerHeartbeatWallet  = "scanner.heartbeat.wallet"
	SubjectPersistenceReady        = "persistence.ready"
)

// QueuePersistence is the queue name for persistence service consumers
const QueuePersistence = "cafe.persistence"
