package scanread

import (
	"testing"
	"time"

	"cafe-discovery/internal/domain"

	"github.com/google/uuid"
)

func TestWalletRowToEntity_CopiesDelegations(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	target := "0x1111111111111111111111111111111111111111"
	raw := `[{"chain_id":1,"delegated_address":"` + target + `"}]`
	ent, err := WalletRowToEntity(WalletScanRowWire{
		ID:                uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa").String(),
		UserID:            uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb").String(),
		Address:           "0xabc",
		Type:              string(domain.AccountTypeEOA),
		Algorithm:         string(domain.AlgorithmECDSAsecp256k1),
		NISTLevel:         1,
		IsEOA:             true,
		Networks:          `["ethereum"]`,
		Delegations:       raw,
		Status:            "SUCCESS",
		PublicKey:         "0x04abcd",
		TransactionHash:   "0xhash",
		ExposedNetwork:    "ethereum",
		PublicKeyRecovery: "recovered",
		ScannedAt:         "2026-10-05T08:47:00Z",
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if ent.Delegations != raw {
		t.Fatalf("stored delegations = %q", ent.Delegations)
	}
	if ent.PublicKeyRecovery != domain.PublicKeyRecoveryRecovered {
		t.Fatalf("public_key_recovery = %q", ent.PublicKeyRecovery)
	}
	if ent.ScannedAt == nil || !ent.ScannedAt.Equal(time.Date(2026, 10, 5, 8, 47, 0, 0, time.UTC)) {
		t.Fatalf("scanned_at = %v", ent.ScannedAt)
	}
	if ent.Type != domain.AccountTypeEOA {
		t.Fatalf("type = %q", ent.Type)
	}
	out := ent.ToScanResult()
	if len(out.Delegations) != 1 || out.Delegations[0].ChainID != 1 || out.Delegations[0].DelegatedAddress != target {
		t.Fatalf("parsed delegations = %#v", out.Delegations)
	}
	if out.PublicKey != "0x04abcd" || out.TransactionHash != "0xhash" || out.ExposedNetwork != "ethereum" {
		t.Fatalf("recovery proof = %#v", out)
	}
	if out.PublicKeyRecovery != domain.PublicKeyRecoveryRecovered {
		t.Fatalf("public_key_recovery = %q", out.PublicKeyRecovery)
	}
	if !out.ScannedAt.Equal(time.Date(2026, 10, 5, 8, 47, 0, 0, time.UTC)) {
		t.Fatalf("scanned_at = %s", out.ScannedAt)
	}
}

func TestWalletRowToEntity_UnknownTypeAndEmptyDelegations(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	for _, raw := range []string{"", "null", "[]", "not-json"} {
		ent, err := WalletRowToEntity(WalletScanRowWire{
			ID:          uuid.NewString(),
			UserID:      uuid.NewString(),
			Address:     "0xabc",
			Type:        string(domain.AccountTypeUnknown),
			Delegations: raw,
			Networks:    "[]",
			Status:      "SUCCESS",
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		if err != nil {
			t.Fatalf("raw %q: %v", raw, err)
		}
		if ent.Type != domain.AccountTypeUnknown {
			t.Fatalf("type = %q", ent.Type)
		}
		out := ent.ToScanResult()
		if out.Type != domain.AccountTypeUnknown {
			t.Fatalf("dto type = %q", out.Type)
		}
		if out.Delegations == nil || len(out.Delegations) != 0 {
			t.Fatalf("delegations for %q = %#v", raw, out.Delegations)
		}
	}
}
