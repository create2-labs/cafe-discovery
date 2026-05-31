package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"
	"cafe-discovery/pkg/scan"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupIMM6b8HandlerDB(t *testing.T) (*gorm.DB, repository.ScanResultRepository, uuid.UUID) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.New().String()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&domain.ScanResultEntity{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	userID := uuid.New()
	return db, repository.NewScanResultRepository(db), userID
}

// IMM-6b-8 / G4: CBOM 404 unless completed success (real ScanResultRepository).
func TestIntegration_IMM6b8_CBOM404OnNonSuccess(t *testing.T) {
	_, repo, userID := setupIMM6b8HandlerDB(t)

	cases := []struct {
		name   string
		status string
		errMsg string
	}{
		{name: "running", status: scan.StateRUNNING},
		{name: "failed", status: scan.StateFAILED},
		{name: "plan_limit_stub", status: scan.StateFAILED, errMsg: scan.ErrPlanLimitExceeded},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scanID := uuid.New()
			ent := &domain.ScanResultEntity{
				ID: scanID, UserID: userID, Address: "0xcbom",
				Type: domain.AccountTypeEOA, Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: 1,
				Status: tc.status, Error: tc.errMsg,
			}
			if err := repo.Create(ent); err != nil {
				t.Fatalf("seed: %v", err)
			}

			h := &DiscoveryHandler{scanResultRepo: repo}
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
			var out map[string]any
			_ = json.NewDecoder(resp.Body).Decode(&out)
			if out["error"] != "not_found" {
				t.Fatalf("error = %v, want not_found", out["error"])
			}
		})
	}
}
