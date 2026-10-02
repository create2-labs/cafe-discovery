package cbom

import (
	"encoding/json"
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
	components := inner["components"].([]map[string]any)
	if components[0]["quantum_vulnerable"] != true {
		t.Fatalf("quantum_vulnerable = %#v", components[0]["quantum_vulnerable"])
	}
	if _, ok := out["delegations"].([]domain.Delegation); !ok {
		t.Fatalf("delegations = %#v", out["delegations"])
	}
}

func TestWallet_DelegationsAndUnknownWithoutAlgorithm(t *testing.T) {
	t.Parallel()
	target := "0x1111111111111111111111111111111111111111"
	sr := &domain.ScanResult{
		Address:   "0xabc",
		Type:      domain.AccountTypeUnknown,
		Algorithm: "",
		NISTLevel: 0,
		Networks:  []string{},
		Delegations: []domain.Delegation{{
			ChainID:          1,
			DelegatedAddress: target,
		}},
	}
	out := Wallet(sr, "0xabc")
	if out["type"] != domain.AccountTypeUnknown {
		t.Fatalf("type = %#v", out["type"])
	}
	delegations, ok := out["delegations"].([]domain.Delegation)
	if !ok || len(delegations) != 1 || delegations[0].DelegatedAddress != target {
		t.Fatalf("delegations = %#v", out["delegations"])
	}
	inner := out["cbom"].(map[string]any)
	component := inner["components"].([]map[string]any)[0]
	if component["quantum_vulnerable"] != false {
		t.Fatalf("quantum_vulnerable = %#v", component["quantum_vulnerable"])
	}
	if _, ok := component["customStates"]; ok {
		t.Fatal("empty algorithm must not add quantum-vulnerable custom state")
	}
}

func TestWallet_NilDelegationsEncodeAsEmptyArray(t *testing.T) {
	t.Parallel()
	out := Wallet(&domain.ScanResult{Type: domain.AccountTypeEOA, Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: 1}, "0xabc")
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	delegations, ok := doc["delegations"].([]any)
	if !ok || len(delegations) != 0 {
		t.Fatalf("delegations = %#v", doc["delegations"])
	}
}
