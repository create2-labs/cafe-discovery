package scanner

import (
	"context"
	"log"
	"time"

	"cafe-discovery/pkg/nats"
	"cafe-discovery/pkg/scan"

	"github.com/google/uuid"
	natslib "github.com/nats-io/nats.go"
)

// maxConcurrentTLSScans limits how many TLS scans run at once (each does network I/O + optional OpenSSL).
const maxConcurrentTLSScans = 5

// TLSScanner processes TLS scan messages from NATS via the TLS plugin.
type TLSScanner struct {
	plugin scan.Plugin
	base   *BaseScanner
	sem    chan struct{}
}

// NewTLSScanner creates a new TLS scanner.
func NewTLSScanner(plugin scan.Plugin, natsConn nats.Connection) *TLSScanner {
	w := &TLSScanner{
		plugin: plugin,
		sem:    make(chan struct{}, maxConcurrentTLSScans),
	}
	d := plugin.Descriptor()
	w.base = NewBaseScanner(natsConn, d.Subject, "TLS", w.handleMessage)
	return w
}

// Start starts the scanner and subscribes to NATS messages.
func (w *TLSScanner) Start(ctx context.Context) error {
	return w.base.Start(ctx)
}

// IsRunning returns whether the scanner is currently running.
func (w *TLSScanner) IsRunning() bool {
	return w.base.IsRunning()
}

func (w *TLSScanner) handleMessage(msg *natslib.Msg) error {
	return ProcessWithConcurrency("TLS", scan.KindTLS, w.plugin.Descriptor().Subject, w.sem, msg, func() error {
		var scanMsg nats.TLSScanMessage
		if err := UnmarshalMessage(msg, &scanMsg); err != nil {
			log.Printf("Failed to unmarshal TLS scan message: %v", err)
			return err
		}
		if scanMsg.ScanID == uuid.Nil {
			scanMsg.ScanID = uuid.New()
		}
		log.Printf("[NATS] RECV scan_id=%s endpoint=%s component=scanner-tls", scanMsg.ScanID.String(), scanMsg.Endpoint)
		// Notify persistence: scan started
		started := nats.ScanStartedMessage{
			ScanID: scanMsg.ScanID, Kind: "tls", UserID: scanMsg.UserID,
			StartedAt: time.Now().UTC().Format(time.RFC3339), Endpoint: scanMsg.Endpoint,
		}
		log.Printf("[NATS] PUB subject=scan.started scan_id=%s component=scanner-tls", scanMsg.ScanID.String())
		if err := nats.PublishJSON(w.base.natsConn, nats.SubjectScanStarted, started); err != nil {
			log.Printf("Failed to publish scan.started: %v", err)
		}
		target, err := w.plugin.DecodeMessage(&scanMsg)
		if err != nil {
			log.Printf("Error decoding tls scan message: %v", err)
			publishTLSScanFailed(w.base.natsConn, scanMsg.ScanID, scanMsg.UserID, scanMsg.Endpoint, err.Error())
			return err
		}
		userID := &scanMsg.UserID
		if scanMsg.UserID == uuid.Nil {
			userID = nil
		}
		result, err := w.plugin.Run(context.Background(), userID, target, scan.RunOptions{IsDefault: scanMsg.IsDefault, SkipPersist: true})
		if err != nil {
			publishTLSScanFailed(w.base.natsConn, scanMsg.ScanID, scanMsg.UserID, scanMsg.Endpoint, err.Error())
			return err
		}
		var resultPayload interface{} = result
		if r, ok := result.(scan.RawResult); ok {
			resultPayload = r.Raw()
		}
		completed := nats.ScanCompletedMessage{
			ScanID: scanMsg.ScanID, Kind: "tls", UserID: scanMsg.UserID,
			CompletedAt: time.Now().UTC().Format(time.RFC3339), Endpoint: scanMsg.Endpoint,
			Result: resultPayload,
		}
		log.Printf("[NATS] PUB subject=scan.completed scan_id=%s component=scanner-tls", scanMsg.ScanID.String())
		if err := nats.PublishJSON(w.base.natsConn, nats.SubjectScanCompleted, completed); err != nil {
			log.Printf("Failed to publish scan.completed: %v", err)
			return err
		}
		return nil
	})
}

func publishTLSScanFailed(conn nats.Connection, scanID, userID uuid.UUID, endpoint, errMsg string) {
	log.Printf("[NATS] PUB subject=scan.failed scan_id=%s component=scanner-tls error=%s", scanID.String(), errMsg)
	_ = nats.PublishJSON(conn, nats.SubjectScanFailed, nats.ScanFailedMessage{
		ScanID: scanID, Kind: "tls", UserID: userID,
		Error: errMsg, CompletedAt: time.Now().UTC().Format(time.RFC3339), Endpoint: endpoint,
	})
}
