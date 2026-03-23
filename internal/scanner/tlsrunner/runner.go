package tlsrunner

import (
	"context"
	"log"
	"time"

	"cafe-discovery/internal/config"
	"cafe-discovery/internal/scan/tls"
	scanner "cafe-discovery/internal/scanner"
	"cafe-discovery/internal/scanner/core"
	"cafe-discovery/internal/tlsscan"
	"cafe-discovery/pkg/nats"
	"cafe-discovery/pkg/scan"

	"github.com/google/uuid"
	"github.com/spf13/viper"
)

const heartbeatInterval = 30 * time.Second

// Runner starts the TLS scan scanner (consumes NATS TLS subject).
type Runner struct{}

// Name implements core.Runner.
func (Runner) Name() string { return "tls" }

// Start implements core.Runner: announces joined via NATS, starts heartbeat, wires plugin and scanner, returns health checkers and shutdown func.
func (Runner) Start(ctx context.Context, deps *core.Deps) ([]core.HealthChecker, func(), error) {
	scannerID := uuid.New().String()
	presence := nats.ScannerPresenceMessage{Event: nats.ScannerPresenceJoined, ScannerID: scannerID, Type: "tls"}
	if err := nats.PublishJSON(deps.NATS, nats.SubjectScannerPresence, presence); err != nil {
		return nil, nil, err
	}
	log.Printf("TLS scanner %s announced joined", scannerID)

	heartbeatCtx, stopHeartbeat := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				_ = nats.PublishJSON(deps.NATS, nats.SubjectScannerPresence, presence)
				_ = nats.PublishJSON(deps.NATS, nats.SubjectScannerHeartbeatTLS, nats.ScannerHeartbeatMessage{
					ScannerID: scannerID, Kind: "tls", Timestamp: time.Now().UTC().Format(time.RFC3339),
				})
			}
		}
	}()

	engine := tlsscan.NewTLSScanEngine()
	tlsPlugin := tls.NewPlugin(engine, viper.GetString(config.ScanPluginsTLSVersion), nats.SubjectScanRequestedTLS)
	scan.Register(tlsPlugin)

	w := scanner.NewTLSScanner(scan.Get(scan.KindTLS), deps.NATS)
	if err := w.Start(ctx); err != nil {
		stopHeartbeat()
		return nil, nil, err
	}

	shutdown := func() {
		stopHeartbeat()
		left := nats.ScannerPresenceMessage{Event: nats.ScannerPresenceLeft, ScannerID: scannerID, Type: "tls"}
		_ = nats.PublishJSON(deps.NATS, nats.SubjectScannerPresence, left)
		log.Printf("TLS scanner %s announced left", scannerID)
	}
	return []core.HealthChecker{w}, shutdown, nil
}
