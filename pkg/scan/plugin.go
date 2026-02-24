package scan

import (
	"context"

	"github.com/google/uuid"
)

// PluginDescriptor identifies the plugin and its limits/capabilities.
type PluginDescriptor struct {
	Kind         string   // "tls" | "wallet"
	Subject      string   // NATS subject
	PlanLimitKey string   // for billing: "tls" -> endpoint limit, "wallet" -> wallet limit
	Capabilities []string // optional
	Version      string   // e.g. "1.0"
}

// RunOptions carries options for Run (e.g. IsDefault for TLS).
type RunOptions struct {
	IsDefault  bool // for TLS: default endpoint vs user-scanned
	SkipPersist bool // when true, service does not write to DB; scanner will publish scan.completed/failed instead
}

// Plugin is the full contract: descriptor, decode (HTTP vs NATS), run.
// For TLS/Wallet: handler uses DecodeHTTP; worker uses DecodeMessage then Run.
type Plugin interface {
	Descriptor() *PluginDescriptor
	DecodeHTTP(body []byte) (ScanTarget, error)   // HTTP request body -> ScanTarget (handler only)
	DecodeMessage(msg any) (ScanTarget, error)    // unmarshaled NATS message -> ScanTarget (worker only)
	Run(ctx context.Context, userID *uuid.UUID, target ScanTarget, opts RunOptions) (ScanResult, error)
}
