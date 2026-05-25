package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"cafe-discovery/internal/discoveryroutes"
	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"
	"cafe-discovery/internal/service"
	"cafe-discovery/pkg/nats"
	"cafe-discovery/pkg/scan"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	natsio "github.com/nats-io/nats.go"
)

type mockNATSConn struct {
	lastSubject string
	lastData    []byte
	publishErr  error
}

func (m *mockNATSConn) Publish(subject string, data []byte) error {
	if m.publishErr != nil {
		return m.publishErr
	}
	m.lastSubject = subject
	m.lastData = append([]byte(nil), data...)
	return nil
}

func (m *mockNATSConn) Subscribe(string, func(msg *natsio.Msg)) (*natsio.Subscription, error) {
	return nil, nil
}

func (m *mockNATSConn) QueueSubscribe(string, string, func(msg *natsio.Msg)) (*natsio.Subscription, error) {
	return nil, nil
}

func (m *mockNATSConn) Close() {}

func (m *mockNATSConn) IsConnected() bool { return true }

type alwaysScanners struct{}

func (alwaysScanners) HasScanner(string) bool { return true }

func (alwaysScanners) ListScanners() []service.ScannerInfo { return nil }

// walletScannerAbsent reports no wallet scanner (TLS may still be "present" but unused in this test).
type walletScannerAbsent struct{}

func (walletScannerAbsent) HasScanner(s string) bool { return s != "wallet" }

func (walletScannerAbsent) ListScanners() []service.ScannerInfo { return nil }

type memoryPendingV1Repo struct {
	mu       sync.Mutex
	byID     map[uuid.UUID]*repository.PendingV1ScanRecord
	byWallet map[string]uuid.UUID
}

func newMemoryPendingV1Repo() *memoryPendingV1Repo {
	return &memoryPendingV1Repo{
		byID:     map[uuid.UUID]*repository.PendingV1ScanRecord{},
		byWallet: map[string]uuid.UUID{},
	}
}

func (r *memoryPendingV1Repo) Put(_ context.Context, rec *repository.PendingV1ScanRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *rec
	r.byID[rec.ScanID] = &cp
	return nil
}

func (r *memoryPendingV1Repo) PutWallet(_ context.Context, rec *repository.PendingV1ScanRecord) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := pendingWalletTestKey(rec.UserID, rec.Address)
	if _, ok := r.byWallet[key]; ok {
		return false, nil
	}
	cp := *rec
	cp.Family = "wallet"
	r.byWallet[key] = rec.ScanID
	r.byID[rec.ScanID] = &cp
	return true, nil
}

func (r *memoryPendingV1Repo) Get(_ context.Context, scanID uuid.UUID) (*repository.PendingV1ScanRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := r.byID[scanID]
	if rec == nil {
		return nil, nil
	}
	cp := *rec
	return &cp, nil
}

func (r *memoryPendingV1Repo) GetWalletByOwnerAddress(_ context.Context, userID uuid.UUID, address string) (*repository.PendingV1ScanRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	scanID, ok := r.byWallet[pendingWalletTestKey(userID, address)]
	if !ok {
		return nil, nil
	}
	rec := r.byID[scanID]
	if rec == nil {
		return &repository.PendingV1ScanRecord{ScanID: scanID, UserID: userID, Family: "wallet", Address: address}, nil
	}
	cp := *rec
	return &cp, nil
}

func (r *memoryPendingV1Repo) Delete(_ context.Context, scanID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rec := r.byID[scanID]; rec != nil && rec.Family == "wallet" {
		delete(r.byWallet, pendingWalletTestKey(rec.UserID, rec.Address))
	}
	delete(r.byID, scanID)
	return nil
}

func (r *memoryPendingV1Repo) DeleteWalletReservation(_ context.Context, userID uuid.UUID, address string, scanID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := pendingWalletTestKey(userID, address)
	if r.byWallet[key] == scanID {
		delete(r.byWallet, key)
	}
	return nil
}

func pendingWalletTestKey(userID uuid.UUID, address string) string {
	return userID.String() + ":" + strings.ToLower(strings.TrimSpace(address))
}

type scanResultRepoStub struct {
	byAddress []*domain.ScanResultEntity
	byID      map[uuid.UUID]*domain.ScanResultEntity
}

func (r *scanResultRepoStub) Create(*domain.ScanResultEntity) error { return nil }

func (r *scanResultRepoStub) FindByUserID(uuid.UUID, int, int) ([]*domain.ScanResultEntity, error) {
	return nil, nil
}

func (r *scanResultRepoStub) FindByID(uuid.UUID) (*domain.ScanResultEntity, error) { return nil, nil }

func (r *scanResultRepoStub) FindOwnedWalletScanByID(_ uuid.UUID, scanID uuid.UUID) (*domain.ScanResultEntity, error) {
	if r.byID == nil {
		return nil, nil
	}
	return r.byID[scanID], nil
}

func (r *scanResultRepoStub) DeleteOwnedWalletScan(uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (r *scanResultRepoStub) ListOwnerWalletScansDiscoveryV1(uuid.UUID, string, int, int) ([]*domain.ScanResultEntity, int64, error) {
	return r.byAddress, int64(len(r.byAddress)), nil
}

func (r *scanResultRepoStub) ListOwnerWalletScansByAddress(uuid.UUID, string) ([]*domain.ScanResultEntity, error) {
	return r.byAddress, nil
}

func (r *scanResultRepoStub) CountByUserID(uuid.UUID) (int64, error) { return 0, nil }

func TestPostDiscoveryScanV1_WalletAccepted(t *testing.T) {
	t.Parallel()
	n := &mockNATSConn{}
	h := &DiscoveryHandler{
		discoveryService: service.NewDiscoveryService(nil, nil, nil, nil),
		natsConn:         n,
		scannerPresence:  alwaysScanners{},
	}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Post(discoveryroutes.PostScan, func(c *fiber.Ctx) error {
		c.Locals("user_id", uuid.MustParse("11111111-1111-1111-1111-111111111111"))
		return h.PostDiscoveryScanV1(c)
	})

	body := []byte(`{"address":"0x742d35Cc6634C0532925a3b844Bc454e4438f44e"}`)
	req := httptest.NewRequest(http.MethodPost, discoveryroutes.PostScan, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, b)
	}
	if n.lastSubject != nats.SubjectScanRequestedWallet {
		t.Fatalf("NATS subject = %q, want %q", n.lastSubject, nats.SubjectScanRequestedWallet)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["status"] != "requested" {
		t.Fatalf("status = %v, want requested", out["status"])
	}
	if out["scan_family"] != "wallet" {
		t.Fatalf("scan_family = %v", out["scan_family"])
	}
	scanID, _ := out["scan_id"].(string)
	if scanID == "" {
		t.Fatalf("missing scan_id")
	}
	wantLoc := discoveryroutes.EdgeWalletScans + scanID
	if out["location"] != wantLoc {
		t.Fatalf("location = %q, want %q", out["location"], wantLoc)
	}
	var published struct {
		ScanID  string `json:"scan_id"`
		Address string `json:"address"`
	}
	if err := json.Unmarshal(n.lastData, &published); err != nil {
		t.Fatalf("published json: %v", err)
	}
	if published.ScanID != scanID {
		t.Fatalf("published scan_id %q != response scan_id %q", published.ScanID, scanID)
	}
	wantAddr := "0x742d35cc6634c0532925a3b844bc454e4438f44e"
	if published.Address != wantAddr {
		t.Fatalf("published address = %q, want %q", published.Address, wantAddr)
	}
}

func TestPostDiscoveryScanV1_WalletPendingRequested409(t *testing.T) {
	t.Parallel()
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	address := "0x742d35cc6634c0532925a3b844bc454e4438f44e"
	pending := newMemoryPendingV1Repo()
	_, err := pending.PutWallet(context.Background(), &repository.PendingV1ScanRecord{
		ScanID:    uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		UserID:    userID,
		Family:    "wallet",
		Address:   address,
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	n := &mockNATSConn{}
	h := &DiscoveryHandler{
		discoveryService: service.NewDiscoveryService(nil, nil, nil, nil),
		natsConn:         n,
		scannerPresence:  alwaysScanners{},
		pendingV1:        pending,
		scanResultRepo:   &scanResultRepoStub{},
	}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Post(discoveryroutes.PostScan, func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		return h.PostDiscoveryScanV1(c)
	})

	req := httptest.NewRequest(http.MethodPost, discoveryroutes.PostScan, bytes.NewReader([]byte(`{"address":"0x742d35Cc6634C0532925a3b844Bc454e4438f44e"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusConflict {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, b)
	}
	if n.lastSubject != "" {
		t.Fatalf("unexpected NATS subject = %q", n.lastSubject)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["error"] != "SCAN_IN_PROGRESS" {
		t.Fatalf("error = %v, want SCAN_IN_PROGRESS", out["error"])
	}
}

func TestPostDiscoveryScanV1_WalletRunningRow409(t *testing.T) {
	t.Parallel()
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	n := &mockNATSConn{}
	h := &DiscoveryHandler{
		discoveryService: service.NewDiscoveryService(nil, nil, nil, nil),
		natsConn:         n,
		scannerPresence:  alwaysScanners{},
		pendingV1:        newMemoryPendingV1Repo(),
		scanResultRepo: &scanResultRepoStub{byAddress: []*domain.ScanResultEntity{{
			ID:      uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
			UserID:  userID,
			Address: "0x742d35cc6634c0532925a3b844bc454e4438f44e",
			Status:  scan.StateRUNNING,
		}}},
	}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Post(discoveryroutes.PostScan, func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		return h.PostDiscoveryScanV1(c)
	})

	req := httptest.NewRequest(http.MethodPost, discoveryroutes.PostScan, bytes.NewReader([]byte(`{"address":"0x742d35Cc6634C0532925a3b844Bc454e4438f44e"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusConflict {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, b)
	}
	if n.lastSubject != "" {
		t.Fatalf("unexpected NATS subject = %q", n.lastSubject)
	}
}

func TestPostDiscoveryScanV1_WalletFailedNewestAccepted(t *testing.T) {
	t.Parallel()
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	n := &mockNATSConn{}
	h := &DiscoveryHandler{
		discoveryService: service.NewDiscoveryService(nil, nil, nil, nil),
		natsConn:         n,
		scannerPresence:  alwaysScanners{},
		pendingV1:        newMemoryPendingV1Repo(),
		scanResultRepo: &scanResultRepoStub{byAddress: []*domain.ScanResultEntity{{
			ID:      uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
			UserID:  userID,
			Address: "0x742d35cc6634c0532925a3b844bc454e4438f44e",
			Status:  scan.StateFAILED,
		}}},
	}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Post(discoveryroutes.PostScan, func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		return h.PostDiscoveryScanV1(c)
	})

	req := httptest.NewRequest(http.MethodPost, discoveryroutes.PostScan, bytes.NewReader([]byte(`{"address":"0x742d35Cc6634C0532925a3b844Bc454e4438f44e"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, b)
	}
	if n.lastSubject != nats.SubjectScanRequestedWallet {
		t.Fatalf("NATS subject = %q, want %q", n.lastSubject, nats.SubjectScanRequestedWallet)
	}
}

func TestPostDiscoveryScanV1_TLSAccepted(t *testing.T) {
	t.Parallel()
	n := &mockNATSConn{}
	h := &DiscoveryHandler{
		discoveryService: service.NewDiscoveryService(nil, nil, nil, nil),
		natsConn:         n,
		scannerPresence:  alwaysScanners{},
	}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Post(discoveryroutes.PostScan, func(c *fiber.Ctx) error {
		c.Locals("user_id", uuid.MustParse("22222222-2222-2222-2222-222222222222"))
		return h.PostDiscoveryScanV1(c)
	})
	body := []byte(`{"url":"https://example.com/path"}`)
	req := httptest.NewRequest(http.MethodPost, discoveryroutes.PostScan, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, b)
	}
	if n.lastSubject != nats.SubjectScanRequestedTLS {
		t.Fatalf("NATS subject = %q", n.lastSubject)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["scan_family"] != "tls" || out["status"] != "requested" {
		t.Fatalf("response = %#v", out)
	}
	scanID, _ := out["scan_id"].(string)
	wantLoc := discoveryroutes.EdgeTLSScans + scanID
	if out["location"] != wantLoc {
		t.Fatalf("location = %q, want %q", out["location"], wantLoc)
	}
}

func TestPostDiscoveryScanV1_BothAddressAndURL(t *testing.T) {
	t.Parallel()
	h := &DiscoveryHandler{discoveryService: service.NewDiscoveryService(nil, nil, nil, nil)}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Post("/scan", func(c *fiber.Ctx) error {
		c.Locals("user_id", uuid.New())
		return h.PostDiscoveryScanV1(c)
	})
	req := httptest.NewRequest(http.MethodPost, "/scan", bytes.NewReader([]byte(`{"address":"0x","url":"https://a"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestPostDiscoveryScanV1_NoScanner503(t *testing.T) {
	t.Parallel()
	h := &DiscoveryHandler{
		discoveryService: service.NewDiscoveryService(nil, nil, nil, nil),
		natsConn:         &mockNATSConn{},
		scannerPresence:  walletScannerAbsent{},
	}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Post("/scan", func(c *fiber.Ctx) error {
		c.Locals("user_id", uuid.New())
		return h.PostDiscoveryScanV1(c)
	})
	req := httptest.NewRequest(http.MethodPost, "/scan", bytes.NewReader([]byte(`{"address":"0x742d35Cc6634C0532925a3b844Bc454e4438f44e"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
}
