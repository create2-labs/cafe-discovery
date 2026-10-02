package handler

import (
	"encoding/json"
	"testing"
	"time"

	"cafe-discovery/internal/config"
	"cafe-discovery/internal/domain"
	"cafe-discovery/pkg/scan"

	"github.com/google/uuid"
)

func TestWalletScanResultV1_UIFields(t *testing.T) {
	t.Parallel()
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	updated := created.Add(2 * time.Hour)
	ent := &domain.ScanResultEntity{
		ID:         uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		Address:    "0xAbCdEf0123456789abcdef0123456789AbCdEf01",
		Type:       domain.AccountTypeEOA,
		Algorithm:  "ECDSA",
		NISTLevel:  2,
		KeyExposed: false,
		IsEOA:      true,
		RiskScore:  0.42,
		Networks:   `["ethereum","8453"]`,
		Status:     scan.StateSUCCESS,
		CreatedAt:  created,
		UpdatedAt:  updated,
	}
	body := walletScanResultV1(ent, &config.ChainConfig{
		Blockchains: []config.Blockchain{
			{Name: "ethereum", ChainID: 1},
			{Name: "base", ChainID: 8453},
		},
	})
	if body["algorithm"] != "ECDSA" {
		t.Fatalf("algorithm = %v", body["algorithm"])
	}
	if body["nist_level"] != 2 {
		t.Fatalf("nist_level = %v", body["nist_level"])
	}
	if body["risk_score"] != 0.42 {
		t.Fatalf("risk_score = %v", body["risk_score"])
	}
	if body["wallet_type"] != "eoa" {
		t.Fatalf("wallet_type = %v", body["wallet_type"])
	}
	chainIDs, ok := body["chain_ids"].([]int64)
	if !ok || len(chainIDs) != 2 {
		t.Fatalf("chain_ids = %v", body["chain_ids"])
	}
}

func TestWalletScanResultV1_typeEOAWithFalseIsEOA_alignsWalletType(t *testing.T) {
	t.Parallel()
	ent := &domain.ScanResultEntity{
		ID:        uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
		Address:   "0x742d35cc6634c0532925a3b844bc454e4438f44e",
		Type:      domain.AccountTypeEOA,
		Algorithm: domain.AlgorithmECDSAsecp256k1,
		NISTLevel: domain.NISTLevel1,
		IsEOA:     false,
		Status:    scan.StateSUCCESS,
	}
	body := walletScanResultV1(ent, nil)
	if body["wallet_type"] != "eoa" {
		t.Fatalf("wallet_type = %v, want eoa", body["wallet_type"])
	}
	if body["type"] != "EOA" {
		t.Fatalf("type = %v, want EOA", body["type"])
	}
}

func TestWalletScanResultV1_unknownAndDelegations(t *testing.T) {
	t.Parallel()
	target := "0x1111111111111111111111111111111111111111"
	ent := &domain.ScanResultEntity{
		ID:          uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd"),
		Address:     "0x2222222222222222222222222222222222222222",
		Type:        domain.AccountTypeUnknown,
		Algorithm:   "",
		NISTLevel:   0,
		IsEOA:       false,
		RiskScore:   0,
		Networks:    "[]",
		Delegations: `[{"chain_id":1,"delegated_address":"` + target + `"}]`,
		Status:      scan.StateSUCCESS,
	}
	body := walletScanResultV1(ent, nil)
	if body["wallet_type"] != domain.WalletTypeUnknown {
		t.Fatalf("wallet_type = %v, want unknown", body["wallet_type"])
	}
	if body["type"] != string(domain.AccountTypeUnknown) {
		t.Fatalf("type = %v, want unknown", body["type"])
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	delegations, ok := doc["delegations"].([]any)
	if !ok || len(delegations) != 1 {
		t.Fatalf("delegations = %#v", doc["delegations"])
	}
	row := delegations[0].(map[string]any)
	if row["chain_id"] != float64(1) || row["delegated_address"] != target {
		t.Fatalf("delegation = %#v", row)
	}
}

func TestWalletScanResultV1_emptyInvalidDelegationsAreArray(t *testing.T) {
	t.Parallel()
	for _, rawDelegations := range []string{"", "null", "[]", "not-json", "   "} {
		ent := &domain.ScanResultEntity{
			Type:        domain.AccountTypeEOA,
			IsEOA:       true,
			Algorithm:   domain.AlgorithmECDSAsecp256k1,
			Delegations: rawDelegations,
			Status:      scan.StateSUCCESS,
		}
		raw, err := json.Marshal(walletScanResultV1(ent, nil))
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		delegations, ok := doc["delegations"].([]any)
		if !ok || len(delegations) != 0 {
			t.Fatalf("delegations for %q = %#v", rawDelegations, doc["delegations"])
		}
	}
}

func TestWalletScanResultV1_legacyAA(t *testing.T) {
	t.Parallel()
	ent := &domain.ScanResultEntity{
		Type:      domain.AccountTypeAA,
		IsEOA:     false,
		IsERC4337: true,
		Algorithm: domain.AlgorithmECDSAsecp256k1,
		Status:    scan.StateSUCCESS,
	}
	body := walletScanResultV1(ent, nil)
	if body["wallet_type"] != domain.WalletTypeSmartAccount {
		t.Fatalf("wallet_type = %v, want smart_account", body["wallet_type"])
	}
	if body["type"] != string(domain.AccountTypeAA) {
		t.Fatalf("type = %v, want AA", body["type"])
	}
}

func TestTlsScanResultBodyV1_UIFields(t *testing.T) {
	t.Parallel()
	ent := &domain.TLSScanResultEntity{
		ID:              uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
		URL:             "https://example.com",
		Host:            "example.com",
		Port:            443,
		ProtocolVersion: "TLS1.3",
		NISTLevel:       3,
		RiskScore:       0.15,
		PQCRisk:         "safe",
		Certificate:     `{"subject":"CN=example.com","issuer":"CN=CA","signature_algorithm":"sha256WithRSA","not_after":"2027-01-01T00:00:00Z"}`,
		CipherSuites:    `[{"name":"TLS_AES_128_GCM_SHA256","key_exchange":"x25519","nist_level":3}]`,
		Default:         true,
		Status:          scan.StateSUCCESS,
		CreatedAt:       time.Now().UTC(),
	}
	body := tlsScanResultBodyV1(ent)
	if body["risk_score"] != 0.15 {
		t.Fatalf("risk_score = %v", body["risk_score"])
	}
	if body["default"] != true {
		t.Fatalf("default = %v", body["default"])
	}
	suites, ok := body["cipher_suites"].([]domain.CipherSuiteInfo)
	if !ok || len(suites) != 1 {
		t.Fatalf("cipher_suites = %T %#v", body["cipher_suites"], body["cipher_suites"])
	}
	if body["endpoint"] != "https://example.com" {
		t.Fatalf("endpoint = %v", body["endpoint"])
	}
}

func TestTlsScanDetailV1_DefaultFlag(t *testing.T) {
	t.Parallel()
	ent := &domain.TLSScanResultEntity{
		ID:      uuid.New(),
		URL:     "https://catalog.example",
		Status:  scan.StateSUCCESS,
		Default: true,
	}
	raw, err := json.Marshal(tlsScanDetailV1(ent))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["is_default"] != true {
		t.Fatalf("is_default = %v", doc["is_default"])
	}
	if _, ok := doc["result"]; !ok {
		t.Fatal("expected result on terminal default scan")
	}
}
