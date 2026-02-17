package wallet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"cafe-discovery/internal/service"
	"cafe-discovery/pkg/nats"
	"cafe-discovery/pkg/scan"

	"github.com/google/uuid"
)

// Plugin implements scan.Plugin for wallet discovery.
type Plugin struct {
	descriptor *scan.PluginDescriptor
	service    *service.DiscoveryService
}

// NewPlugin returns the wallet discovery plugin. version is read from config (e.g. scan.plugins.wallet.version).
// subjectOverride: when non-empty (e.g. nats.SubjectScanRequestedWallet in scanner), use it instead of SubjectWalletScan.
func NewPlugin(svc *service.DiscoveryService, version string, subjectOverride string) *Plugin {
	if version == "" {
		version = "1.0"
	}
	subject := nats.SubjectWalletScan
	if subjectOverride != "" {
		subject = subjectOverride
	}
	return &Plugin{
		descriptor: &scan.PluginDescriptor{
			Kind:         scan.KindWallet,
			Subject:      subject,
			PlanLimitKey: scan.PlanLimitKeyWallet,
			Version:      version,
		},
		service: svc,
	}
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
	normalized, err := p.service.ValidateAndNormalizeAddress(req.Address)
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
	t, ok := target.(*scan.WalletTarget)
	if !ok {
		return nil, errors.New("invalid target type for wallet plugin")
	}
	uid := uuid.Nil
	if userID != nil {
		uid = *userID
	}
	result, err := p.service.ScanWallet(ctx, uid, t.Address, opts.SkipPersist)
	if err != nil {
		return nil, err
	}
	return &walletResultAdapter{ScanResult: result}, nil
}

// Ensure Plugin implements scan.Plugin.
var _ scan.Plugin = (*Plugin)(nil)
