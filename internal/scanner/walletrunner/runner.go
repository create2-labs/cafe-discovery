package walletrunner

import (
	"context"
	"log"
	"time"

	"cafe-discovery/internal/config"
	"cafe-discovery/internal/scan/wallet"
	scanner "cafe-discovery/internal/scanner"
	"cafe-discovery/internal/scanner/core"
	"cafe-discovery/internal/service"
	"cafe-discovery/pkg/evm"
	"cafe-discovery/pkg/moralis"
	"cafe-discovery/pkg/nats"
	"cafe-discovery/pkg/scan"

	"github.com/google/uuid"
	"github.com/spf13/viper"
)

const heartbeatInterval = 30 * time.Second

// Runner starts the Wallet scan scanner (consumes NATS Wallet subject).
type Runner struct{}

// Name implements core.Runner.
func (Runner) Name() string { return "wallet" }

// Start implements core.Runner: announces joined via NATS, starts heartbeat, wires plugin and scanner, returns health checkers and shutdown func.
func (Runner) Start(ctx context.Context, deps *core.Deps) ([]core.HealthChecker, func(), error) {
	scannerID := uuid.New().String()
	presence := nats.ScannerPresenceMessage{Event: nats.ScannerPresenceJoined, ScannerID: scannerID, Type: "wallet"}
	if err := nats.PublishJSON(deps.NATS, nats.SubjectScannerPresence, presence); err != nil {
		return nil, nil, err
	}
	log.Printf("Wallet scanner %s announced joined", scannerID)

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
				_ = nats.PublishJSON(deps.NATS, nats.SubjectScannerHeartbeatWallet, nats.ScannerHeartbeatMessage{
					ScannerID: scannerID, Kind: "wallet", Timestamp: time.Now().UTC().Format(time.RFC3339),
				})
			}
		}
	}()

	clients := make(map[string]*evm.Client)
	for _, blockchain := range deps.ChainConfig.Blockchains {
		clients[blockchain.Name] = evm.NewClient(blockchain.RPC, blockchain.MoralisChainName)
	}
	moralisClient := moralis.NewMoralisClient(viper.GetString(config.MoralisAPIKey), viper.GetString(config.MoralisAPIURL))
	// Scanners do not use Postgres; persistence-service writes from scan.completed/failed
	discoveryService := service.NewDiscoveryService(clients, moralisClient, nil, nil)

	walletPlugin := wallet.NewPlugin(discoveryService, viper.GetString(config.ScanPluginsWalletVersion), nats.SubjectScanRequestedWallet)
	scan.Register(walletPlugin)

	w := scanner.NewWalletScanner(scan.Get(scan.KindWallet), deps.NATS)
	if err := w.Start(ctx); err != nil {
		stopHeartbeat()
		return nil, nil, err
	}

	shutdown := func() {
		stopHeartbeat()
		left := nats.ScannerPresenceMessage{Event: nats.ScannerPresenceLeft, ScannerID: scannerID, Type: "wallet"}
		_ = nats.PublishJSON(deps.NATS, nats.SubjectScannerPresence, left)
		log.Printf("Wallet scanner %s announced left", scannerID)
	}
	return []core.HealthChecker{w}, shutdown, nil
}
