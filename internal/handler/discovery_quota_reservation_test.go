package handler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"cafe-discovery/internal/discoveryroutes"
	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"
	"cafe-discovery/internal/service"
	"cafe-discovery/pkg/nats"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	natsio "github.com/nats-io/nats.go"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type countingNATS struct {
	publishes atomic.Int32
	err       error
}

func (m *countingNATS) Publish(string, []byte) error {
	return nil
}

func (m *countingNATS) PublishJetStream(string, []byte) error {
	if m.err != nil {
		return m.err
	}
	m.publishes.Add(1)
	return nil
}

func (m *countingNATS) ListStreamPayloads(context.Context) ([][]byte, error) {
	return nil, nil
}

func (m *countingNATS) Subscribe(string, func(msg *natsio.Msg)) (*natsio.Subscription, error) {
	return nil, nil
}

func (m *countingNATS) QueueSubscribe(string, string, func(msg *natsio.Msg)) (*natsio.Subscription, error) {
	return nil, nil
}

func (m *countingNATS) Close() {}

func (m *countingNATS) IsConnected() bool { return true }

func setupQuotaReservationDB(t *testing.T) (*gorm.DB, repository.ScanUsageLedgerRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.New().String()+"?mode=memory&cache=shared&_txlock=immediate"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&domain.ScanUsageEventEntity{}, &domain.ScanResultEntity{}, &domain.TLSScanResultEntity{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	return db, repository.NewScanUsageLedgerRepository(db)
}

func newQuotaReservationHandler(t *testing.T, walletLimit, endpointLimit int, conn nats.Connection) (*DiscoveryHandler, repository.ScanUsageLedgerRepository, *memoryPendingV1Repo) {
	t.Helper()
	_, ledger := setupQuotaReservationDB(t)
	planID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	plan := &domain.Plan{ID: planID, WalletScanLimit: walletLimit, EndpointScanLimit: endpointLimit}
	user := &domain.User{ID: userID, PlanID: planID}
	pending := newMemoryPendingV1Repo()
	h := &DiscoveryHandler{
		natsConn:        conn,
		planService:     service.NewPlanService(&handlerPlanPlanRepo{plan: plan}, &handlerPlanUserRepo{user: user}),
		scannerPresence: alwaysScanners{},
		scanUsageLedger: ledger,
		scanPending:     pending,
		policyRef:       policyRefStub{},
	}
	return h, ledger, pending
}

func postScan(t *testing.T, app *fiber.App, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, discoveryroutes.PostScan, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

func quotaScanApp(h *DiscoveryHandler) *fiber.App {
	app := fiber.New()
	app.Post(discoveryroutes.PostScan, func(c fiber.Ctx) error {
		c.Locals("user_id", uuid.MustParse("11111111-1111-1111-1111-111111111111"))
		return h.PostDiscoveryScanV1(c)
	})
	return app
}

func TestPostDiscoveryScanV1_QuotaFullDoesNotPublish(t *testing.T) {
	t.Parallel()
	n := &countingNATS{}
	h, ledger, pending := newQuotaReservationHandler(t, 1, 1, n)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	if err := ledger.RecordSuccessUsage(userID, uuid.New(), domain.ScanUsageKindWallet); err != nil {
		t.Fatalf("seed: %v", err)
	}
	app := quotaScanApp(h)

	status, body := postScan(t, app, `{"address":"0x1111111111111111111111111111111111111111"}`)
	if status != fiber.StatusForbidden {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	if n.publishes.Load() != 0 {
		t.Fatalf("publishes = %d, want 0", n.publishes.Load())
	}
	count, err := ledger.CountSuccessUsage(userID, domain.ScanUsageKindWallet)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("ledger count = %d, want 1", count)
	}
	if got := pendingRecordCount(pending); got != 0 {
		t.Fatalf("pending records = %d, want 0", got)
	}
}

func TestPostDiscoveryScanV1_PublishFailureReleasesQuota(t *testing.T) {
	t.Parallel()
	n := &countingNATS{err: errors.New("nats down")}
	h, ledger, pending := newQuotaReservationHandler(t, 5, 5, n)
	app := quotaScanApp(h)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	status, body := postScan(t, app, `{"address":"0x1111111111111111111111111111111111111111"}`)
	if status != fiber.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	count, err := ledger.CountSuccessUsage(userID, domain.ScanUsageKindWallet)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("ledger count = %d, want 0 after publish failure", count)
	}
	if got := pendingRecordCount(pending); got != 0 {
		t.Fatalf("pending records = %d, want 0", got)
	}
}

func TestPostDiscoveryScanV1_TLSPublishFailureReleasesQuota(t *testing.T) {
	t.Parallel()
	n := &countingNATS{err: errors.New("nats down")}
	h, ledger, pending := newQuotaReservationHandler(t, 5, 5, n)
	app := quotaScanApp(h)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	status, body := postScan(t, app, `{"url":"https://example.com/path"}`)
	if status != fiber.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	count, err := ledger.CountSuccessUsage(userID, domain.ScanUsageKindEndpoint)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("ledger count = %d, want 0 after TLS publish failure", count)
	}
	if got := pendingRecordCount(pending); got != 0 {
		t.Fatalf("pending records = %d, want 0", got)
	}
}

func TestPostDiscoveryScanV1_ConcurrentOneCreditOnePublish(t *testing.T) {
	t.Parallel()
	n := &countingNATS{}
	h, ledger, _ := newQuotaReservationHandler(t, 1, 1, n)
	app := quotaScanApp(h)
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	bodies := []string{
		`{"address":"0x1111111111111111111111111111111111111111"}`,
		`{"address":"0x2222222222222222222222222222222222222222"}`,
	}

	var wg sync.WaitGroup
	codes := make([]int, len(bodies))
	errs := make([]error, len(bodies))
	for i, body := range bodies {
		wg.Add(1)
		go func(i int, body string) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, discoveryroutes.PostScan, bytes.NewReader([]byte(body)))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
			if err != nil {
				errs[i] = err
				return
			}
			defer resp.Body.Close()
			_, _ = io.ReadAll(resp.Body)
			codes[i] = resp.StatusCode
		}(i, body)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}

	var accepted, refused int
	for _, code := range codes {
		switch code {
		case fiber.StatusOK:
			accepted++
		case fiber.StatusForbidden:
			refused++
		default:
			t.Fatalf("unexpected status %d among %v", code, codes)
		}
	}
	if accepted != 1 || refused != 1 {
		t.Fatalf("accepted=%d refused=%d codes=%v", accepted, refused, codes)
	}
	if n.publishes.Load() != 1 {
		t.Fatalf("publishes = %d, want 1", n.publishes.Load())
	}
	count, err := ledger.CountSuccessUsage(userID, domain.ScanUsageKindWallet)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("ledger count = %d, want 1", count)
	}
}

func pendingRecordCount(p *memoryPendingV1Repo) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.byID)
}
