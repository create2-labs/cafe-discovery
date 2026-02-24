package wallet

import (
	"cafe-discovery/internal/domain"
	"cafe-discovery/pkg/scan"
	"time"
)

// walletResultAdapter wraps *domain.ScanResult and implements scan.ScanResult.
type walletResultAdapter struct {
	*domain.ScanResult
}

func (a *walletResultAdapter) ScanKind() string { return scan.KindWallet }
func (a *walletResultAdapter) ScannedAt() time.Time { return a.ScanResult.ScannedAt }
func (a *walletResultAdapter) Findings() []scan.Finding {
	return []scan.Finding{{
		Type:        "cryptographic-primitive",
		Name:        string(a.Algorithm),
		NISTLevel:   int(a.NISTLevel),
		QuantumVuln: a.NISTLevel <= 1,
		Details:     map[string]any{"key_exposed": a.KeyExposed},
	}}
}
func (a *walletResultAdapter) Classification() string {
	if a.NISTLevel <= 1 {
		return "legacy"
	}
	return "pq-ready"
}

func (a *walletResultAdapter) ToCBOM() (map[string]any, error) {
	return walletResultToCBOM(a.ScanResult), nil
}

// Raw implements scan.RawResult for persistence (scan.completed payload).
func (a *walletResultAdapter) Raw() interface{} {
	return a.ScanResult
}

// walletResultToCBOM produces the same shape as handler's scanResultToCBOM (API unchanged).
func walletResultToCBOM(sr *domain.ScanResult) map[string]any {
	component := map[string]any{
		"type":               "cryptographic-primitive",
		"name":               sr.Algorithm,
		"nist_level":         sr.NISTLevel,
		"quantum_vulnerable": sr.NISTLevel <= 1,
		"key_exposed":        sr.KeyExposed,
		"assetType":          "related-crypto-material",
		"state":              "active",
	}
	if sr.NISTLevel <= 1 {
		component["customStates"] = []map[string]any{{
			"name":        "quantum-vulnerable",
			"description": "Key relies on cryptographic algorithms considered vulnerable to future cryptographic quantum attacks",
		}}
	}
	timestamp := sr.ScannedAt.Format(time.RFC3339)
	if sr.ScannedAt.IsZero() {
		timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	return map[string]any{
		"address":    sr.Address,
		"type":       sr.Type,
		"algorithm":  sr.Algorithm,
		"nist_level": sr.NISTLevel,
		"key_exposed": sr.KeyExposed,
		"risk_score":  sr.RiskScore,
		"first_seen":  sr.FirstSeen,
		"last_seen":   sr.LastSeen,
		"networks":    sr.Networks,
		"scanned_at":  sr.ScannedAt,
		"cbom": map[string]any{
			"bomFormat":   "CycloneDX",
			"specVersion": "1.7",
			"version":     1,
			"metadata":    map[string]any{"timestamp": timestamp},
			"type":        "wallet",
			"components":  []map[string]any{component},
		},
	}
}
