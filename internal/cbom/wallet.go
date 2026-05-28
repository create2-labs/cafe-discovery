package cbom

import (
	"time"

	"cafe-discovery/internal/domain"
)

// Wallet builds the wallet CBOM JSON envelope from a scan result DTO (W6, on-demand).
// address is the display key (typically canonical lowercase); when empty, sr.Address is used.
func Wallet(sr *domain.ScanResult, address string) map[string]any {
	if sr == nil {
		return nil
	}
	if address == "" {
		address = sr.Address
	}

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

	timestamp := sr.ScannedAt.UTC().Format(time.RFC3339)
	if sr.ScannedAt.IsZero() {
		timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	return map[string]any{
		"address":     address,
		"type":        sr.Type,
		"algorithm":   sr.Algorithm,
		"nist_level":  sr.NISTLevel,
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
			"metadata": map[string]any{
				"timestamp": timestamp,
			},
			"type":       "wallet",
			"components": []map[string]any{component},
		},
	}
}
