package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/middleware"
	"cafe-discovery/internal/service"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// Manual checks before merge (local / edge):
//
//curl -si "http://localhost:8080/discovery/wallet-policy-contexts" | head -1    # expect 401
//
// TOKEN=$(curl -sS -X POST http://localhost:8080/auth/signin -H 'Content-Type: application/json' \
//   -d '{"email":"...","password":"...","turnstile_token":"dev-pass"}' | jq -r .token)
// curl -si "http://localhost:8080/discovery/wallet-policy-contexts" -H "Authorization: Bearer $TOKEN"
//
// Via nginx HTTPS edge (prefix /api stripped by proxy):
//
//curl -si "https://<host>/api/discovery/wallet-policy-contexts" -H "Authorization: Bearer $TOKEN"

func injectTestUser(userID uuid.UUID) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		return c.Next()
	}
}

type memoryWalletListRepo struct {
	byUser map[uuid.UUID][]*domain.ScanResultEntity
}

func (m *memoryWalletListRepo) Create(*domain.ScanResultEntity) error { return errStub }

func (m *memoryWalletListRepo) FindByUserID(uid uuid.UUID, limit, offset int) ([]*domain.ScanResultEntity, error) {
	rows := m.byUser[uid]
	if offset >= len(rows) {
		return nil, nil
	}
	end := offset + limit
	if limit <= 0 || end > len(rows) {
		end = len(rows)
	}
	out := append([]*domain.ScanResultEntity(nil), rows[offset:end]...)
	return out, nil
}

func (m *memoryWalletListRepo) FindByID(uuid.UUID) (*domain.ScanResultEntity, error) { return nil, nil }

func (m *memoryWalletListRepo) FindByUserIDAndAddress(uuid.UUID, string) (*domain.ScanResultEntity, error) {
	return nil, nil
}

func (m *memoryWalletListRepo) CountByUserID(uid uuid.UUID) (int64, error) {
	return int64(len(m.byUser[uid])), nil
}

type noopTLSListRepo struct{}

func (noopTLSListRepo) Create(*domain.TLSScanResultEntity) error { return errStub }
func (noopTLSListRepo) FindByUserID(uuid.UUID, int, int) ([]*domain.TLSScanResultEntity, error) {
	return nil, nil
}
func (noopTLSListRepo) FindByUserIDOrDefault(uuid.UUID, int, int) ([]*domain.TLSScanResultEntity, error) {
	return nil, nil
}
func (noopTLSListRepo) FindByID(uuid.UUID) (*domain.TLSScanResultEntity, error) { return nil, nil }
func (noopTLSListRepo) FindByUserIDAndURL(uuid.UUID, string) (*domain.TLSScanResultEntity, error) {
	return nil, nil
}
func (noopTLSListRepo) FindByURL(string) (*domain.TLSScanResultEntity, error) { return nil, nil }
func (noopTLSListRepo) FindDefaultByURL(string) (*domain.TLSScanResultEntity, error) { return nil, nil }
func (noopTLSListRepo) FindAllDefault() ([]*domain.TLSScanResultEntity, error) { return nil, nil }
func (noopTLSListRepo) CountByUserID(uuid.UUID) (int64, error) { return 0, nil }
func (noopTLSListRepo) CountByUserIDOrDefault(uuid.UUID) (int64, error) { return 0, nil }

// errStub is returned from repository stub methods not exercised by these tests.
var errStub = errors.New("repository stub: not implemented")

func TestListWalletPolicyContexts_MissingJWT_Unauthorized(t *testing.T) {
	t.Parallel()

	authSvc, err := service.NewAuthService(nil, nil, "unused", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	api := app.Group("/discovery", middleware.JWTMiddleware(authSvc))
	h := &DiscoveryHandler{}
	api.Get("/wallet-policy-contexts", h.ListWalletPolicyContexts)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/discovery/wallet-policy-contexts", nil), -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("missing JWT: status %d, want %d", resp.StatusCode, fiber.StatusUnauthorized)
	}
}

func TestListWalletPolicyContexts_InvalidJWT_Unauthorized(t *testing.T) {
	t.Parallel()

	authSvc, err := service.NewAuthService(nil, nil, "unused", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	api := app.Group("/discovery", middleware.JWTMiddleware(authSvc))
	h := &DiscoveryHandler{}
	api.Get("/wallet-policy-contexts", h.ListWalletPolicyContexts)

	req := httptest.NewRequest(http.MethodGet, "/discovery/wallet-policy-contexts", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-token")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("invalid JWT: status %d, want %d", resp.StatusCode, fiber.StatusUnauthorized)
	}
}

func TestListWalletPolicyContexts_AuthenticatedJSONShape(t *testing.T) {
	t.Parallel()

	userA := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	scanID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	networks, _ := json.Marshal([]string{"ethereum-mainnet"})

	repo := &memoryWalletListRepo{
		byUser: map[uuid.UUID][]*domain.ScanResultEntity{
			userA: {{
				ID:        scanID,
				UserID:    userA,
				Address:   "0xdddddddddddddddddddddddddddddddddddddddd",
				Type:      domain.AccountTypeEOA,
				NISTLevel: domain.NISTLevel1,
				Networks:  string(networks),
				Status:    "SUCCESS",
				UpdatedAt: time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC),
			}},
		},
	}
	cache := service.NewUserScanCacheService(repo, noopTLSListRepo{}, nil, nil)
	h := &DiscoveryHandler{userScanCache: cache}

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/discovery/wallet-policy-contexts", injectTestUser(userA), h.ListWalletPolicyContexts)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/discovery/wallet-policy-contexts", nil), -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, string(body))
	}

	var envelope struct {
		Contexts []map[string]any `json:"contexts"`
		Total    int64            `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Total != 1 || len(envelope.Contexts) != 1 {
		t.Fatalf("want 1 context, got total=%v len=%d", envelope.Total, len(envelope.Contexts))
	}
	row := envelope.Contexts[0]
	if row["scan_id"] != scanID.String() {
		t.Fatalf("scan_id = %v", row["scan_id"])
	}
	if row["status"] != "completed" {
		t.Fatalf("API must expose normalized status completed, got %v", row["status"])
	}
}
