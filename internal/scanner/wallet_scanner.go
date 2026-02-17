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

const maxConcurrentWalletScans = 5

// WalletScanner processes wallet scan messages from NATS via the wallet plugin.
type WalletScanner struct {
	plugin scan.Plugin
	base   *BaseScanner
	sem    chan struct{}
}

// NewWalletScanner creates a new wallet scanner.
func NewWalletScanner(plugin scan.Plugin, natsConn nats.Connection) *WalletScanner {
	w := &WalletScanner{
		plugin: plugin,
		sem:    make(chan struct{}, maxConcurrentWalletScans),
	}
	d := plugin.Descriptor()
	w.base = NewBaseScanner(natsConn, d.Subject, "Wallet", w.handleMessage)
	return w
}

// Start starts the scanner and subscribes to NATS messages.
func (w *WalletScanner) Start(ctx context.Context) error {
	return w.base.Start(ctx)
}

// IsRunning returns whether the scanner is currently running.
func (w *WalletScanner) IsRunning() bool {
	return w.base.IsRunning()
}

func (w *WalletScanner) handleMessage(msg *natslib.Msg) error {
	return ProcessWithConcurrency("Wallet", scan.KindWallet, w.plugin.Descriptor().Subject, w.sem, msg, func() error {
		var scanMsg nats.WalletScanMessage
		if err := UnmarshalMessage(msg, &scanMsg); err != nil {
			log.Printf("Failed to unmarshal wallet scan message: %v", err)
			return err
		}
		if scanMsg.ScanID == uuid.Nil {
			scanMsg.ScanID = uuid.New()
		}
		started := nats.ScanStartedMessage{
			ScanID: scanMsg.ScanID, Kind: "wallet", UserID: scanMsg.UserID,
			StartedAt: time.Now().UTC().Format(time.RFC3339), Address: scanMsg.Address,
		}
		if err := nats.PublishJSON(w.base.natsConn, nats.SubjectScanStarted, started); err != nil {
			log.Printf("Failed to publish scan.started: %v", err)
		}
		target, err := w.plugin.DecodeMessage(&scanMsg)
		if err != nil {
			log.Printf("Error decoding wallet scan message: %v", err)
			publishWalletScanFailed(w.base.natsConn, scanMsg.ScanID, scanMsg.UserID, scanMsg.Address, err.Error())
			return err
		}
		userID := &scanMsg.UserID
		result, err := w.plugin.Run(context.Background(), userID, target, scan.RunOptions{SkipPersist: true})
		if err != nil {
			publishWalletScanFailed(w.base.natsConn, scanMsg.ScanID, scanMsg.UserID, scanMsg.Address, err.Error())
			return err
		}
		var resultPayload interface{} = result
		if r, ok := result.(scan.RawResult); ok {
			resultPayload = r.Raw()
		}
		completed := nats.ScanCompletedMessage{
			ScanID: scanMsg.ScanID, Kind: "wallet", UserID: scanMsg.UserID,
			CompletedAt: time.Now().UTC().Format(time.RFC3339), Address: scanMsg.Address,
			Result: resultPayload,
		}
		if err := nats.PublishJSON(w.base.natsConn, nats.SubjectScanCompleted, completed); err != nil {
			log.Printf("Failed to publish scan.completed: %v", err)
			return err
		}
		return nil
	})
}

func publishWalletScanFailed(conn nats.Connection, scanID, userID uuid.UUID, address, errMsg string) {
	_ = nats.PublishJSON(conn, nats.SubjectScanFailed, nats.ScanFailedMessage{
		ScanID: scanID, Kind: "wallet", UserID: userID,
		Error: errMsg, CompletedAt: time.Now().UTC().Format(time.RFC3339), Address: address,
	})
}
