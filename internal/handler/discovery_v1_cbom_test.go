package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"
	"cafe-discovery/pkg/scan"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func TestGetDiscoveryV1WalletScanCBOM_Terminal200(t *testing.T) {
	t.Parallel()
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	scanID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	ent := &domain.ScanResultEntity{
		ID:        scanID,
		UserID:    userID,
		Address:   "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
		Type:      domain.AccountTypeEOA,
		Algorithm: domain.AlgorithmECDSAsecp256k1,
		NISTLevel: domain.NISTLevel1,
		Status:    scan.StateSUCCESS,
		RiskScore: 0.5,
		Networks:  `["ethereum-mainnet"]`,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	h := &DiscoveryHandler{
		scanResultRepo: &scanResultRepoStub{byID: map[uuid.UUID]*domain.ScanResultEntity{scanID: ent}},
	}
	app := fiber.New()
	app.Get("/wallets/scans/:scan_id/cbom", func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		return h.GetDiscoveryV1WalletScanCBOM(c)
	})

	req := httptest.NewRequest("GET", "/wallets/scans/"+scanID.String()+"/cbom", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	inner, ok := out["cbom"].(map[string]any)
	if !ok || inner["bomFormat"] != "CycloneDX" {
		t.Fatalf("cbom envelope missing or invalid: %v", out["cbom"])
	}
	if out["address"] != ent.Address {
		t.Fatalf("address = %v, want %q", out["address"], ent.Address)
	}
}

func TestGetDiscoveryV1WalletScanCBOM_NotTerminal409(t *testing.T) {
	t.Parallel()
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	scanID := uuid.New()
	ent := &domain.ScanResultEntity{
		ID: scanID, UserID: userID, Address: "0xabc", Status: scan.StateRUNNING,
		Type: domain.AccountTypeEOA, Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: 1,
	}
	h := &DiscoveryHandler{
		scanResultRepo: &scanResultRepoStub{byID: map[uuid.UUID]*domain.ScanResultEntity{scanID: ent}},
	}
	app := fiber.New()
	app.Get("/wallets/scans/:scan_id/cbom", func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		return h.GetDiscoveryV1WalletScanCBOM(c)
	})
	req := httptest.NewRequest("GET", "/wallets/scans/"+scanID.String()+"/cbom", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out["error"] != "SCAN_NOT_TERMINAL" {
		t.Fatalf("error = %v, want SCAN_NOT_TERMINAL", out["error"])
	}
}

func TestGetDiscoveryV1WalletScanCBOM_NotFound404(t *testing.T) {
	t.Parallel()
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	scanID := uuid.New()
	h := &DiscoveryHandler{scanResultRepo: &scanResultRepoStub{byID: map[uuid.UUID]*domain.ScanResultEntity{}}}
	app := fiber.New()
	app.Get("/wallets/scans/:scan_id/cbom", func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		return h.GetDiscoveryV1WalletScanCBOM(c)
	})
	req := httptest.NewRequest("GET", "/wallets/scans/"+scanID.String()+"/cbom", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestGetDiscoveryV1WalletScanCBOM_Pending409(t *testing.T) {
	t.Parallel()
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	scanID := uuid.New()
	pending := newMemoryPendingV1Repo()
	_, _ = pending.PutWallet(nil, &repository.PendingV1ScanRecord{
		ScanID: scanID, UserID: userID, Family: "wallet", Address: "0xabc",
	})
	h := &DiscoveryHandler{
		scanResultRepo: &scanResultRepoStub{byID: map[uuid.UUID]*domain.ScanResultEntity{}},
		pendingV1:      pending,
	}
	app := fiber.New()
	app.Get("/wallets/scans/:scan_id/cbom", func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		return h.GetDiscoveryV1WalletScanCBOM(c)
	})
	req := httptest.NewRequest("GET", "/wallets/scans/"+scanID.String()+"/cbom", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
}
