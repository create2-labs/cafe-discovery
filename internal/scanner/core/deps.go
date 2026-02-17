package core

import (
	"context"

	"cafe-discovery/internal/config"
	"cafe-discovery/pkg/nats"
)

// Deps holds shared dependencies for all scanner runners (NATS, chain config).
// Scanners do not access Postgres; they publish scan.started/completed/failed to NATS for the persistence-service.
type Deps struct {
	NATS        nats.Connection
	ChainConfig *config.ChainConfig
}

// HealthChecker is implemented by each scanner type for the health endpoint.
type HealthChecker interface {
	IsRunning() bool
}

// Runner starts one kind of scanner (TLS or Wallet) and returns health checkers plus a shutdown func.
// The shutdown func must be called on process exit so the scanner can announce "left" via NATS.
type Runner interface {
	// Name returns the scanner kind for health checks and presence (e.g. "tls", "wallet").
	Name() string
	// Start starts the scanner(s). It announces "joined" via NATS before consuming, then returns
	// health checkers and a shutdown func that announces "left" and stops heartbeats.
	Start(ctx context.Context, deps *Deps) ([]HealthChecker, func(), error)
}
