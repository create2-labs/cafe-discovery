package contract

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cafe-discovery/internal/config"
	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/handler"
	"cafe-discovery/pkg/scan"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type memoryWalletScanRepo struct {
	byOwner map[uuid.UUID][]*domain.ScanResultEntity
}

func (m *memoryWalletScanRepo) Create(*domain.ScanResultEntity) error { return nil }

func (m *memoryWalletScanRepo) FindByUserID(uuid.UUID, int, int) ([]*domain.ScanResultEntity, error) {
	return nil, nil
}

func (m *memoryWalletScanRepo) FindByID(uuid.UUID) (*domain.ScanResultEntity, error) {
	return nil, nil
}

func (m *memoryWalletScanRepo) FindOwnedWalletScanByID(userID, scanID uuid.UUID) (*domain.ScanResultEntity, error) {
	for _, e := range m.byOwner[userID] {
		if e != nil && e.ID == scanID {
			return e, nil
		}
	}
	return nil, nil
}

func (m *memoryWalletScanRepo) ListOwnerWalletScansDiscoveryV1(userID uuid.UUID, address string, limit, offset int) ([]*domain.ScanResultEntity, int64, error) {
	all := m.byOwner[userID]
	if address != "" {
		filtered := make([]*domain.ScanResultEntity, 0)
		for _, e := range all {
			if e != nil && e.Address == address {
				filtered = append(filtered, e)
			}
		}
		all = filtered
	}
	total := int64(len(all))
	if offset >= len(all) {
		return nil, total, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], total, nil
}

func (m *memoryWalletScanRepo) ListOwnerWalletScansByAddress(userID uuid.UUID, address string) ([]*domain.ScanResultEntity, error) {
	all := m.byOwner[userID]
	filtered := make([]*domain.ScanResultEntity, 0)
	for _, e := range all {
		if e != nil && e.Address == address {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

func (m *memoryWalletScanRepo) CountByUserID(uuid.UUID) (int64, error) { return 0, nil }

func (m *memoryWalletScanRepo) DeleteOwnedWalletScan(uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func walletScanEntity(id, owner uuid.UUID, address string) *domain.ScanResultEntity {
	now := time.Date(2026, 5, 11, 10, 27, 10, 0, time.UTC)
	return &domain.ScanResultEntity{
		ID:        id,
		UserID:    owner,
		Address:   address,
		Type:      domain.AccountTypeEOA,
		Algorithm: "ECDSA",
		NISTLevel: 2,
		IsEOA:     true,
		Networks:  `["ethereum"]`,
		Status:    scan.StateSUCCESS,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func walletScanEntityWithNetworks(id, owner uuid.UUID, address string, networks string, status string, createdAt time.Time) *domain.ScanResultEntity {
	ent := walletScanEntity(id, owner, address)
	ent.Networks = networks
	ent.Status = status
	ent.CreatedAt = createdAt
	ent.UpdatedAt = createdAt
	return ent
}

func TestDiscoveryV1WalletScans_listWithoutPrincipalReturns401(t *testing.T) {
	t.Parallel()
	h := handler.NewDiscoveryHandlerForContractTest(&memoryWalletScanRepo{}, nil)
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/wallets/scans", h.ListDiscoveryV1WalletScans)
	req := httptest.NewRequest(http.MethodGet, "/wallets/scans", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestDiscoveryV1WalletScans_listWithPrincipalReturns200Envelope(t *testing.T) {
	t.Parallel()
	owner := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	scanID := uuid.MustParse("705c9704-9428-45e0-882d-fae4cb9d2a0b")
	repo := &memoryWalletScanRepo{
		byOwner: map[uuid.UUID][]*domain.ScanResultEntity{
			owner: {walletScanEntity(scanID, owner, "0x0802b015613ef6701192811e595e085a9c560caf")},
		},
	}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	h := handler.NewDiscoveryHandlerForContractTest(repo, &config.ChainConfig{
		Blockchains: []config.Blockchain{{Name: "ethereum", ChainID: 1}},
	})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", owner)
		return c.Next()
	})
	app.Get("/wallets/scans", h.ListDiscoveryV1WalletScans)
	req := httptest.NewRequest(http.MethodGet, "/wallets/scans", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	raw, _ := io.ReadAll(resp.Body)
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"items", "total", "limit", "offset"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("missing %q in list envelope", key)
		}
	}
	items, ok := body["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("items = %#v", body["items"])
	}
}

func TestDiscoveryV1WalletScans_addressFilterReturnsAllExecutions(t *testing.T) {
	t.Parallel()
	owner := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	address := "0x0802b015613ef6701192811e595e085a9c560caf"
	repo := &memoryWalletScanRepo{
		byOwner: map[uuid.UUID][]*domain.ScanResultEntity{
			owner: {
				walletScanEntityWithNetworks(uuid.MustParse("705c9704-9428-45e0-882d-fae4cb9d2a0c"), owner, address, `["ethereum"]`, scan.StateRUNNING, time.Date(2026, 5, 11, 10, 30, 0, 0, time.UTC)),
				walletScanEntityWithNetworks(uuid.MustParse("705c9704-9428-45e0-882d-fae4cb9d2a0b"), owner, address, `["ethereum"]`, scan.StateSUCCESS, time.Date(2026, 5, 11, 10, 20, 0, 0, time.UTC)),
				walletScanEntityWithNetworks(uuid.MustParse("705c9704-9428-45e0-882d-fae4cb9d2a0a"), owner, address, `["ethereum"]`, scan.StateFAILED, time.Date(2026, 5, 11, 10, 10, 0, 0, time.UTC)),
			},
		},
	}
	h := handler.NewDiscoveryHandlerForContractTest(repo, &config.ChainConfig{
		Blockchains: []config.Blockchain{{Name: "ethereum", ChainID: 1}},
	})
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", owner)
		return c.Next()
	})
	app.Get("/wallets/scans", h.ListDiscoveryV1WalletScans)

	req := httptest.NewRequest(http.MethodGet, "/wallets/scans?address="+address+"&limit=50", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	raw, _ := io.ReadAll(resp.Body)
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body["total"] != float64(3) {
		t.Fatalf("total = %v, want 3", body["total"])
	}
	items := body["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("items len = %d, want 3", len(items))
	}
	first := items[0].(map[string]any)
	if first["status"] != "started" {
		t.Fatalf("first status = %v, want started", first["status"])
	}
}

func TestDiscoveryV1WalletScans_latestReturnsNewestCompletedOnly(t *testing.T) {
	t.Parallel()
	owner := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	address := "0x0802b015613ef6701192811e595e085a9c560caf"
	completedID := uuid.MustParse("705c9704-9428-45e0-882d-fae4cb9d2a0b")
	repo := &memoryWalletScanRepo{
		byOwner: map[uuid.UUID][]*domain.ScanResultEntity{
			owner: {
				walletScanEntityWithNetworks(uuid.MustParse("705c9704-9428-45e0-882d-fae4cb9d2a0c"), owner, address, `["ethereum"]`, scan.StateFAILED, time.Date(2026, 5, 11, 10, 30, 0, 0, time.UTC)),
				walletScanEntityWithNetworks(completedID, owner, address, `["ethereum"]`, scan.StateSUCCESS, time.Date(2026, 5, 11, 10, 20, 0, 0, time.UTC)),
				walletScanEntityWithNetworks(uuid.MustParse("705c9704-9428-45e0-882d-fae4cb9d2a0a"), owner, address, `["ethereum"]`, scan.StateRUNNING, time.Date(2026, 5, 11, 10, 10, 0, 0, time.UTC)),
			},
		},
	}
	h := handler.NewDiscoveryHandlerForContractTest(repo, &config.ChainConfig{
		Blockchains: []config.Blockchain{{Name: "ethereum", ChainID: 1}},
	})
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", owner)
		return c.Next()
	})
	app.Get("/wallets/scans", h.ListDiscoveryV1WalletScans)

	req := httptest.NewRequest(http.MethodGet, "/wallets/scans?address="+address+"&latest=true", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	raw, _ := io.ReadAll(resp.Body)
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body["total"] != float64(1) {
		t.Fatalf("total = %v, want 1", body["total"])
	}
	items := body["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	item := items[0].(map[string]any)
	if item["scan_id"] != completedID.String() {
		t.Fatalf("scan_id = %v, want latest completed %s", item["scan_id"], completedID)
	}
	if item["status"] != "completed" {
		t.Fatalf("status = %v, want completed", item["status"])
	}
}

func TestDiscoveryV1WalletScans_latestReturnsEmptyWhenNoCompletedScanExists(t *testing.T) {
	t.Parallel()
	owner := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	address := "0x0802b015613ef6701192811e595e085a9c560caf"
	repo := &memoryWalletScanRepo{
		byOwner: map[uuid.UUID][]*domain.ScanResultEntity{
			owner: {
				walletScanEntityWithNetworks(uuid.MustParse("705c9704-9428-45e0-882d-fae4cb9d2a0c"), owner, address, `["ethereum"]`, scan.StateFAILED, time.Date(2026, 5, 11, 10, 30, 0, 0, time.UTC)),
				walletScanEntityWithNetworks(uuid.MustParse("705c9704-9428-45e0-882d-fae4cb9d2a0b"), owner, address, `["ethereum"]`, scan.StateRUNNING, time.Date(2026, 5, 11, 10, 20, 0, 0, time.UTC)),
			},
		},
	}
	h := handler.NewDiscoveryHandlerForContractTest(repo, &config.ChainConfig{
		Blockchains: []config.Blockchain{{Name: "ethereum", ChainID: 1}},
	})
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", owner)
		return c.Next()
	})
	app.Get("/wallets/scans", h.ListDiscoveryV1WalletScans)

	req := httptest.NewRequest(http.MethodGet, "/wallets/scans?address="+address+"&latest=true", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	raw, _ := io.ReadAll(resp.Body)
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body["total"] != float64(0) {
		t.Fatalf("total = %v, want 0", body["total"])
	}
	items := body["items"].([]any)
	if len(items) != 0 {
		t.Fatalf("items len = %d, want 0", len(items))
	}
}

func TestDiscoveryV1WalletScans_latestRequiresAddress(t *testing.T) {
	t.Parallel()
	h := handler.NewDiscoveryHandlerForContractTest(&memoryWalletScanRepo{}, nil)
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", uuid.MustParse("11111111-1111-1111-1111-111111111111"))
		return c.Next()
	})
	app.Get("/wallets/scans", h.ListDiscoveryV1WalletScans)

	req := httptest.NewRequest(http.MethodGet, "/wallets/scans?latest=true", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDiscoveryV1WalletScans_latestChainIDReturnsNewestCompletedMatchingChain(t *testing.T) {
	t.Parallel()
	owner := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	address := "0x0802b015613ef6701192811e595e085a9c560caf"
	ethereumID := uuid.MustParse("705c9704-9428-45e0-882d-fae4cb9d2a0b")
	repo := &memoryWalletScanRepo{
		byOwner: map[uuid.UUID][]*domain.ScanResultEntity{
			owner: {
				walletScanEntityWithNetworks(uuid.MustParse("705c9704-9428-45e0-882d-fae4cb9d2a0c"), owner, address, `["polygon"]`, scan.StateSUCCESS, time.Date(2026, 5, 11, 10, 30, 0, 0, time.UTC)),
				walletScanEntityWithNetworks(ethereumID, owner, address, `["ethereum"]`, scan.StateSUCCESS, time.Date(2026, 5, 11, 10, 20, 0, 0, time.UTC)),
			},
		},
	}
	h := handler.NewDiscoveryHandlerForContractTest(repo, &config.ChainConfig{
		Blockchains: []config.Blockchain{
			{Name: "ethereum", ChainID: 1},
			{Name: "polygon", ChainID: 137},
		},
	})
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", owner)
		return c.Next()
	})
	app.Get("/wallets/scans", h.ListDiscoveryV1WalletScans)

	req := httptest.NewRequest(http.MethodGet, "/wallets/scans?address="+address+"&latest=true&chain_id=1", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	raw, _ := io.ReadAll(resp.Body)
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	items := body["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	got := items[0].(map[string]any)["scan_id"]
	if got != ethereumID.String() {
		t.Fatalf("scan_id = %v, want newest completed chain match %s", got, ethereumID)
	}
}

func TestDiscoveryV1WalletScans_chainIDFilterUsesAllAddressRowsBeforePagination(t *testing.T) {
	t.Parallel()
	owner := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	address := "0x0802b015613ef6701192811e595e085a9c560caf"
	ethOnly := uuid.MustParse("705c9704-9428-45e0-882d-fae4cb9d2a0b")
	ethAndPolygon := uuid.MustParse("705c9704-9428-45e0-882d-fae4cb9d2a0a")
	repo := &memoryWalletScanRepo{
		byOwner: map[uuid.UUID][]*domain.ScanResultEntity{
			owner: {
				walletScanEntityWithNetworks(uuid.MustParse("705c9704-9428-45e0-882d-fae4cb9d2a0c"), owner, address, `["polygon"]`, scan.StateSUCCESS, time.Date(2026, 5, 11, 10, 30, 0, 0, time.UTC)),
				walletScanEntityWithNetworks(ethOnly, owner, address, `["ethereum"]`, scan.StateSUCCESS, time.Date(2026, 5, 11, 10, 20, 0, 0, time.UTC)),
				walletScanEntityWithNetworks(ethAndPolygon, owner, address, `["ethereum","polygon"]`, scan.StateSUCCESS, time.Date(2026, 5, 11, 10, 10, 0, 0, time.UTC)),
			},
		},
	}
	h := handler.NewDiscoveryHandlerForContractTest(repo, &config.ChainConfig{
		Blockchains: []config.Blockchain{
			{Name: "ethereum", ChainID: 1},
			{Name: "polygon", ChainID: 137},
		},
	})
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", owner)
		return c.Next()
	})
	app.Get("/wallets/scans", h.ListDiscoveryV1WalletScans)

	req := httptest.NewRequest(http.MethodGet, "/wallets/scans?address="+address+"&chain_id=1&limit=1&offset=1", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	raw, _ := io.ReadAll(resp.Body)
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body["total"] != float64(2) {
		t.Fatalf("total = %v, want 2", body["total"])
	}
	items := body["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	got := items[0].(map[string]any)["scan_id"]
	if got != ethAndPolygon.String() {
		t.Fatalf("scan_id = %v, want second matching chain row %s; first matching row is %s", got, ethAndPolygon, ethOnly)
	}
}

func TestDiscoveryV1WalletScans_detailOwnerIsolation(t *testing.T) {
	t.Parallel()
	ownerA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	ownerB := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	scanID := uuid.MustParse("705c9704-9428-45e0-882d-fae4cb9d2a0b")
	repo := &memoryWalletScanRepo{
		byOwner: map[uuid.UUID][]*domain.ScanResultEntity{
			ownerA: {walletScanEntity(scanID, ownerA, "0x0802b015613ef6701192811e595e085a9c560caf")},
		},
	}
	h := handler.NewDiscoveryHandlerForContractTest(repo, &config.ChainConfig{
		Blockchains: []config.Blockchain{{Name: "ethereum", ChainID: 1}},
	})
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", ownerB)
		return c.Next()
	})
	app.Get("/wallets/scans/:scan_id", h.GetDiscoveryV1WalletScan)

	req := httptest.NewRequest(http.MethodGet, "/wallets/scans/"+scanID.String(), nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("other owner detail status = %d, want 404", resp.StatusCode)
	}
}

func TestDiscoveryV1WalletScans_detailStableFieldsForOwner(t *testing.T) {
	t.Parallel()
	owner := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	scanID := uuid.MustParse("705c9704-9428-45e0-882d-fae4cb9d2a0b")
	repo := &memoryWalletScanRepo{
		byOwner: map[uuid.UUID][]*domain.ScanResultEntity{
			owner: {walletScanEntity(scanID, owner, "0x0802b015613ef6701192811e595e085a9c560caf")},
		},
	}
	h := handler.NewDiscoveryHandlerForContractTest(repo, &config.ChainConfig{
		Blockchains: []config.Blockchain{{Name: "ethereum", ChainID: 1}},
	})
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", owner)
		return c.Next()
	})
	app.Get("/wallets/scans/:scan_id", h.GetDiscoveryV1WalletScan)

	req := httptest.NewRequest(http.MethodGet, "/wallets/scans/"+scanID.String(), nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	raw, _ := io.ReadAll(resp.Body)
	var detail map[string]any
	if err := json.Unmarshal(raw, &detail); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"scan_id", "status", "result"} {
		if _, ok := detail[key]; !ok {
			t.Fatalf("WalletScanDetail missing %q", key)
		}
	}
	result, ok := detail["result"].(map[string]any)
	if !ok {
		t.Fatal("result is not an object")
	}
	for _, key := range []string{"target_address", "chain_ids", "wallet_type", "current_pq_posture"} {
		if _, ok := result[key]; !ok {
			t.Fatalf("WalletScanResult missing %q", key)
		}
	}
}

// IMM-7: two completed executions for the same address must both appear in the list (total >= 2).
func TestDiscoveryV1WalletScans_twoExecutionsSameAddress_listContainsBoth(t *testing.T) {
	t.Parallel()
	owner := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	address := "0x0802b015613ef6701192811e595e085a9c560caf"
	scanA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-000000000001")
	scanB := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-000000000002")
	repo := &memoryWalletScanRepo{
		byOwner: map[uuid.UUID][]*domain.ScanResultEntity{
			owner: {
				walletScanEntityWithNetworks(scanB, owner, address, `["ethereum"]`, scan.StateSUCCESS, time.Date(2026, 5, 11, 11, 0, 0, 0, time.UTC)),
				walletScanEntityWithNetworks(scanA, owner, address, `["ethereum"]`, scan.StateSUCCESS, time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)),
			},
		},
	}
	h := handler.NewDiscoveryHandlerForContractTest(repo, &config.ChainConfig{
		Blockchains: []config.Blockchain{{Name: "ethereum", ChainID: 1}},
	})
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", owner)
		return c.Next()
	})
	app.Get("/wallets/scans", h.ListDiscoveryV1WalletScans)

	req := httptest.NewRequest(http.MethodGet, "/wallets/scans?address="+address+"&limit=50", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	raw, _ := io.ReadAll(resp.Body)
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body["total"].(float64) < 2 {
		t.Fatalf("total = %v, want >= 2 (both executions must be listed)", body["total"])
	}
	items := body["items"].([]any)
	if len(items) < 2 {
		t.Fatalf("items len = %d, want >= 2", len(items))
	}
	scanIDs := make(map[string]bool)
	for _, item := range items {
		scanIDs[item.(map[string]any)["scan_id"].(string)] = true
	}
	if !scanIDs[scanA.String()] {
		t.Fatalf("historical scan_id A %s missing from list", scanA)
	}
	if !scanIDs[scanB.String()] {
		t.Fatalf("newer scan_id B %s missing from list", scanB)
	}
}

// IMM-7: after a second scan on the same address, the older scan_id still returns 200 with an
// immutable result (target_address, wallet_type, current_pq_posture unchanged).
func TestDiscoveryV1WalletScans_historicalScanIDDetailIsStable(t *testing.T) {
	t.Parallel()
	owner := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	address := "0x0802b015613ef6701192811e595e085a9c560caf"
	scanA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-000000000003")
	scanB := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-000000000004")

	entityA := walletScanEntity(scanA, owner, address)
	entityA.Status = scan.StateSUCCESS
	entityA.CreatedAt = time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	entityA.UpdatedAt = entityA.CreatedAt

	entityB := walletScanEntity(scanB, owner, address)
	entityB.Status = scan.StateSUCCESS
	entityB.CreatedAt = time.Date(2026, 5, 11, 11, 0, 0, 0, time.UTC)
	entityB.UpdatedAt = entityB.CreatedAt

	repo := &memoryWalletScanRepo{
		byOwner: map[uuid.UUID][]*domain.ScanResultEntity{
			owner: {entityB, entityA},
		},
	}
	h := handler.NewDiscoveryHandlerForContractTest(repo, &config.ChainConfig{
		Blockchains: []config.Blockchain{{Name: "ethereum", ChainID: 1}},
	})
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", owner)
		return c.Next()
	})
	app.Get("/wallets/scans/:scan_id", h.GetDiscoveryV1WalletScan)

	req := httptest.NewRequest(http.MethodGet, "/wallets/scans/"+scanA.String(), nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("historical scan_id A detail status = %d, want 200", resp.StatusCode)
	}
	raw, _ := io.ReadAll(resp.Body)
	var detail map[string]any
	if err := json.Unmarshal(raw, &detail); err != nil {
		t.Fatal(err)
	}
	if detail["scan_id"] != scanA.String() {
		t.Fatalf("scan_id = %v, want historical %s", detail["scan_id"], scanA)
	}
	if detail["status"] != "completed" {
		t.Fatalf("status = %v, want completed", detail["status"])
	}
	result, ok := detail["result"].(map[string]any)
	if !ok {
		t.Fatal("result is not an object on historical scan detail")
	}
	for _, key := range []string{"target_address", "wallet_type", "current_pq_posture"} {
		if _, ok := result[key]; !ok {
			t.Fatalf("historical WalletScanResult missing %q", key)
		}
	}
	if result["target_address"] != address {
		t.Fatalf("target_address = %v, want %s (result must be immutable for historical scan)", result["target_address"], address)
	}
}

// IMM-D4: ScanListItem is lifecycle synopsis only — never crypto posture or result payload.
func TestDiscoveryV1WalletScans_listItemSynopsisExcludesPostureFields(t *testing.T) {
	t.Parallel()
	owner := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	address := "0x0802b015613ef6701192811e595e085a9c560caf"
	scanID := uuid.MustParse("705c9704-9428-45e0-882d-fae4cb9d2a0f")
	repo := &memoryWalletScanRepo{
		byOwner: map[uuid.UUID][]*domain.ScanResultEntity{
			owner: {
				walletScanEntityWithNetworks(scanID, owner, address, `["ethereum"]`, scan.StateSUCCESS, time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)),
				walletScanEntityWithNetworks(uuid.MustParse("705c9704-9428-45e0-882d-fae4cb9d2a0e"), owner, address, `[]`, scan.StateRUNNING, time.Date(2026, 5, 11, 10, 30, 0, 0, time.UTC)),
			},
		},
	}
	h := handler.NewDiscoveryHandlerForContractTest(repo, &config.ChainConfig{
		Blockchains: []config.Blockchain{{Name: "ethereum", ChainID: 1}},
	})
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", owner)
		return c.Next()
	})
	app.Get("/wallets/scans", h.ListDiscoveryV1WalletScans)

	req := httptest.NewRequest(http.MethodGet, "/wallets/scans?address="+address+"&limit=50", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	raw, _ := io.ReadAll(resp.Body)
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	items := body["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2", len(items))
	}

	allowed := map[string]bool{
		"scan_id": true, "target_address": true, "chain_ids": true,
		"created_at": true, "status": true,
	}
	forbidden := []string{
		"result", "wallet_type", "current_pq_posture", "algorithm", "type",
		"nist_level", "risk_score", "key_exposed", "observations", "networks",
		"first_seen", "last_seen", "scanned_at",
	}
	for _, item := range items {
		row := item.(map[string]any)
		for key := range row {
			if !allowed[key] {
				t.Fatalf("list item contains unexpected key %q (synopsis contract)", key)
			}
		}
		for _, key := range forbidden {
			if _, ok := row[key]; ok {
				t.Fatalf("list item must not expose posture field %q", key)
			}
		}
	}
}

// IMM-D4: WalletScanDetail omits result until terminal lifecycle state.
func TestDiscoveryV1WalletScans_detailOmitsResultUntilTerminal(t *testing.T) {
	t.Parallel()
	owner := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	address := "0x0802b015613ef6701192811e595e085a9c560caf"
	cases := []struct {
		name   string
		status string
		want   string
	}{
		{name: "requested", status: scan.StatePENDING, want: "requested"},
		{name: "started", status: scan.StateRUNNING, want: "started"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			scanID := uuid.New()
			ent := walletScanEntity(scanID, owner, address)
			ent.Status = tc.status
			repo := &memoryWalletScanRepo{
				byOwner: map[uuid.UUID][]*domain.ScanResultEntity{owner: {ent}},
			}
			h := handler.NewDiscoveryHandlerForContractTest(repo, &config.ChainConfig{
				Blockchains: []config.Blockchain{{Name: "ethereum", ChainID: 1}},
			})
			app := fiber.New(fiber.Config{DisableStartupMessage: true})
			app.Use(func(c *fiber.Ctx) error {
				c.Locals("user_id", owner)
				return c.Next()
			})
			app.Get("/wallets/scans/:scan_id", h.GetDiscoveryV1WalletScan)

			req := httptest.NewRequest(http.MethodGet, "/wallets/scans/"+scanID.String(), nil)
			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != fiber.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			raw, _ := io.ReadAll(resp.Body)
			var detail map[string]any
			if err := json.Unmarshal(raw, &detail); err != nil {
				t.Fatal(err)
			}
			if detail["status"] != tc.want {
				t.Fatalf("status = %v, want %s", detail["status"], tc.want)
			}
			if _, ok := detail["result"]; ok {
				t.Fatalf("non-terminal detail must not include result (status=%s)", tc.want)
			}
		})
	}
}

// IMM-D4: terminal detail exposes posture under result only.
func TestDiscoveryV1WalletScans_detailIncludesResultAtTerminal(t *testing.T) {
	t.Parallel()
	owner := uuid.MustParse("10101010-1010-1010-1010-101010101010")
	scanID := uuid.MustParse("20202020-2020-2020-2020-202020202020")
	address := "0x0802b015613ef6701192811e595e085a9c560caf"
	repo := &memoryWalletScanRepo{
		byOwner: map[uuid.UUID][]*domain.ScanResultEntity{
			owner: {walletScanEntity(scanID, owner, address)},
		},
	}
	h := handler.NewDiscoveryHandlerForContractTest(repo, &config.ChainConfig{
		Blockchains: []config.Blockchain{{Name: "ethereum", ChainID: 1}},
	})
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", owner)
		return c.Next()
	})
	app.Get("/wallets/scans/:scan_id", h.GetDiscoveryV1WalletScan)

	req := httptest.NewRequest(http.MethodGet, "/wallets/scans/"+scanID.String(), nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	raw, _ := io.ReadAll(resp.Body)
	var detail map[string]any
	if err := json.Unmarshal(raw, &detail); err != nil {
		t.Fatal(err)
	}
	if detail["status"] != "completed" {
		t.Fatalf("status = %v, want completed", detail["status"])
	}
	result, ok := detail["result"].(map[string]any)
	if !ok {
		t.Fatal("terminal detail must include result object")
	}
	for _, key := range []string{"wallet_type", "current_pq_posture", "target_address", "chain_ids"} {
		if _, ok := result[key]; !ok {
			t.Fatalf("WalletScanResult missing %q on terminal detail", key)
		}
	}
}
