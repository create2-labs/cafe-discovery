package tlsscan

import (
	"context"
	"fmt"
	"time"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/metrics"
	tlspkg "cafe-discovery/pkg/tls"

	"github.com/google/uuid"
)

// TLSScanEngine runs TLS/PQC scans and builds domain results without plan checks or persistence.
type TLSScanEngine struct {
	scanner  *tlspkg.Scanner
	pqcRules *tlspkg.PQCRules
}

// NewTLSScanEngine creates a new TLS scan engine.
func NewTLSScanEngine() *TLSScanEngine {
	return &TLSScanEngine{
		scanner:  tlspkg.NewScanner(),
		pqcRules: tlspkg.NewPQCRules(),
	}
}

// Execute performs a TLS scan and returns a domain result.
// userID is currently unused by scan logic but preserved for call-site compatibility.
func (e *TLSScanEngine) Execute(ctx context.Context, userID *uuid.UUID, targetURL string, isDefault bool) (result *domain.TLSScanResult, err error) {
	_ = userID

	startTime := time.Now()
	m := metrics.Get()
	defer func() {
		m.RecordTLSScan(time.Since(startTime), err == nil)
	}()

	info, err := e.scanner.ScanURL(ctx, targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to scan TLS: %w", err)
	}

	protocolVersionStr := tlspkg.GetProtocolVersion(info.ProtocolVersion)

	if info.ProtocolVersion < 0x0304 { // TLS 1.3 = 0x0304
		res := e.buildResultForTLSBelow13(info, targetURL, protocolVersionStr)
		res.Default = isDefault
		return res, nil
	}

	state := e.buildTLS13BaseState(info, targetURL, protocolVersionStr)
	e.applyPQCInfoToResult(state, info, protocolVersionStr)
	e.setPFSAndALPNOnResult(state, info, protocolVersionStr)
	e.updateRiskScoreIfNeeded(state, info, protocolVersionStr)
	e.updateNISTLevelAndRiskFromPQC(state, info, protocolVersionStr)
	e.updatePQCRiskAndFinalScores(state, info, protocolVersionStr)

	state.result.Default = isDefault
	return state.result, nil
}
