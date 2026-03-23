package wallet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/metrics"
	"cafe-discovery/internal/walletscan"
	"cafe-discovery/pkg/nats"
	"cafe-discovery/pkg/scan"

	"github.com/google/uuid"
)

// Plugin implements scan.Plugin for wallet discovery.
type Plugin struct {
	descriptor *scan.PluginDescriptor
	engine     *walletscan.WalletScanEngine
	// validate is used for DecodeHTTP (same rules as DiscoveryService API path).
	validate func(address string) (string, error)
}

// NewPlugin returns the wallet discovery plugin. version is read from config (e.g. scan.plugins.wallet.version).
// subjectOverride: when non-empty (e.g. nats.SubjectScanRequestedWallet in scanner), use it instead of SubjectWalletScan.
// validate, if non-nil, is used for DecodeHTTP (typically DiscoveryService.ValidateAndNormalizeAddress). If nil, the engine validates.
func NewPlugin(engine *walletscan.WalletScanEngine, version string, subjectOverride string, validate func(address string) (string, error)) *Plugin {
	if version == "" {
		version = "1.0"
	}
	subject := nats.SubjectWalletScan
	if subjectOverride != "" {
		subject = subjectOverride
	}
	p := &Plugin{
		descriptor: &scan.PluginDescriptor{
			Kind:         scan.KindWallet,
			Subject:      subject,
			PlanLimitKey: scan.PlanLimitKeyWallet,
			Version:      version,
		},
		engine:   engine,
		validate: validate,
	}
	if p.validate == nil {
		p.validate = engine.ValidateAndNormalizeAddress
	}
	return p
}

// Descriptor implements scan.Plugin.
func (p *Plugin) Descriptor() *scan.PluginDescriptor { return p.descriptor }

// DecodeHTTP implements scan.Plugin. Body should be {"address": "0x..."}.
func (p *Plugin) DecodeHTTP(body []byte) (scan.ScanTarget, error) {
	var req struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("invalid request body: %w", err)
	}
	if req.Address == "" {
		return nil, errors.New("address is required")
	}
	normalized, err := p.validate(req.Address)
	if err != nil {
		return nil, err
	}
	return &scan.WalletTarget{Address: normalized}, nil
}

// DecodeMessage implements scan.Plugin. msg is *nats.WalletScanMessage.
func (p *Plugin) DecodeMessage(msg any) (scan.ScanTarget, error) {
	m, ok := msg.(*nats.WalletScanMessage)
	if !ok {
		return nil, errors.New("invalid message type for wallet plugin")
	}
	if m.Address == "" {
		return nil, errors.New("address is required")
	}
	return &scan.WalletTarget{Address: m.Address}, nil
}

// Run implements scan.Plugin.
func (p *Plugin) Run(ctx context.Context, userID *uuid.UUID, target scan.ScanTarget, opts scan.RunOptions) (scan.ScanResult, error) {
	_ = userID
	_ = opts.SkipPersist // scanner path never persists; engine has no DB access
	t, ok := target.(*scan.WalletTarget)
	if !ok {
		return nil, errors.New("invalid target type for wallet plugin")
	}
	var err error
	startTime := time.Now()
	m := metrics.Get()
	defer func() {
		m.RecordWalletScan(time.Since(startTime), err == nil)
	}()

	var result *domain.ScanResult
	result, err = p.engine.Execute(ctx, t.Address)
	if err != nil {
		return nil, err
	}
	return &walletResultAdapter{ScanResult: result}, nil
}

// Ensure Plugin implements scan.Plugin.
var _ scan.Plugin = (*Plugin)(nil)
