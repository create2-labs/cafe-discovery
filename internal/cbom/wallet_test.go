package cbom

import (
	"testing"
	"time"

	"cafe-discovery/internal/domain"
)

func TestWallet_CycloneDXEnvelope(t *testing.T) {
	t.Parallel()
	scannedAt := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	sr := &domain.ScanResult{
		Address:    "0xAbC",
		Type:       domain.AccountTypeEOA,
		Algorithm:  domain.AlgorithmECDSAsecp256k1,
		NISTLevel:  domain.NISTLevel1,
		KeyExposed: true,
		RiskScore:  0.85,
		Networks:   []string{"ethereum-mainnet"},
		ScannedAt:  scannedAt,
	}

	out := Wallet(sr, "0xabc")
	if out == nil {
		t.Fatal("expected non-nil CBOM")
	}
	if out["address"] != "0xabc" {
		t.Fatalf("address = %v, want 0xabc", out["address"])
	}
	inner, ok := out["cbom"].(map[string]any)
	if !ok {
		t.Fatalf("cbom type = %T", out["cbom"])
	}
	if inner["bomFormat"] != "CycloneDX" {
		t.Fatalf("bomFormat = %v", inner["bomFormat"])
	}
	if inner["specVersion"] != "1.7" {
		t.Fatalf("specVersion = %v", inner["specVersion"])
	}
}
