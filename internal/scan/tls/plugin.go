package tls

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"cafe-discovery/internal/service"
	"cafe-discovery/pkg/nats"
	"cafe-discovery/pkg/scan"

	"github.com/google/uuid"
)

const (
	schemeHTTP  = "http://"
	schemeHTTPS = "https://"
)

// Plugin implements scan.Plugin for TLS endpoint discovery.
type Plugin struct {
	descriptor *scan.PluginDescriptor
	service    *service.TLSService
}

// NewPlugin returns the TLS discovery plugin. version is read from config (e.g. scan.plugins.tls.version).
// subjectOverride: when non-empty (e.g. nats.SubjectScanRequestedTLS in scanner), use it instead of SubjectTLSScan.
func NewPlugin(svc *service.TLSService, version string, subjectOverride string) *Plugin {
	if version == "" {
		version = "1.0"
	}
	subject := nats.SubjectTLSScan
	if subjectOverride != "" {
		subject = subjectOverride
	}
	return &Plugin{
		descriptor: &scan.PluginDescriptor{
			Kind:         scan.KindTLS,
			Subject:      subject,
			PlanLimitKey: scan.PlanLimitKeyEndpoint,
			Version:      version,
		},
		service: svc,
	}
}

// Descriptor implements scan.Plugin.
func (p *Plugin) Descriptor() *scan.PluginDescriptor { return p.descriptor }

// DecodeHTTP implements scan.Plugin. Body should be {"url": "https://..."}.
func (p *Plugin) DecodeHTTP(body []byte) (scan.ScanTarget, error) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("invalid request body: %w", err)
	}
	if req.URL == "" {
		return nil, errors.New("url is required")
	}
	if !strings.HasPrefix(req.URL, schemeHTTPS) && !strings.HasPrefix(req.URL, schemeHTTP) {
		return nil, errors.New("url must use http:// or https:// protocol")
	}
	parsed, err := url.Parse(req.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL format: %w", err)
	}
	if parsed.Host == "" {
		return nil, errors.New("url must include a valid hostname")
	}
	if parsed.Hostname() == "" {
		return nil, errors.New("url must include a valid hostname")
	}
	return &scan.TLSTarget{Endpoint: req.URL}, nil
}

// DecodeMessage implements scan.Plugin. msg is *nats.TLSScanMessage.
func (p *Plugin) DecodeMessage(msg any) (scan.ScanTarget, error) {
	m, ok := msg.(*nats.TLSScanMessage)
	if !ok {
		return nil, errors.New("invalid message type for TLS plugin")
	}
	if m.Endpoint == "" {
		return nil, errors.New("endpoint is required")
	}
	return &scan.TLSTarget{Endpoint: m.Endpoint}, nil
}

// Run implements scan.Plugin.
func (p *Plugin) Run(ctx context.Context, userID *uuid.UUID, target scan.ScanTarget, opts scan.RunOptions) (scan.ScanResult, error) {
	t, ok := target.(*scan.TLSTarget)
	if !ok {
		return nil, errors.New("invalid target type for TLS plugin")
	}
	result, err := p.service.ScanTLS(ctx, userID, t.Endpoint, opts.IsDefault, opts.SkipPersist)
	if err != nil {
		return nil, err
	}
	return &tlsResultAdapter{TLSScanResult: result}, nil
}

// Ensure Plugin implements scan.Plugin.
var _ scan.Plugin = (*Plugin)(nil)
