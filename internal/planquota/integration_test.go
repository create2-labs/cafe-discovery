// Package planquota holds IMM-6b-8 integration tests for plan quota guards (G1–G4),
// completion race (G3), monotonic DELETE (P1-b), and CBOM success-only.
package planquota

import (
	"sync"
	"testing"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/persistence/handlers"
	"cafe-discovery/internal/persistence/planlimit"
	"cafe-discovery/internal/persistence/storage"
	"cafe-discovery/internal/repository"
	"cafe-discovery/internal/service"
	"cafe-discovery/pkg/nats"
	"cafe-discovery/pkg/scan"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type integrationEnv struct {
	db          *gorm.DB
	userID      uuid.UUID
	planID      uuid.UUID
	ledger      repository.ScanUsageLedgerRepository
	planSvc     *service.PlanService
	scanRepo    repository.ScanResultRepository
	scanHandler *handlers.ScanEventHandler
}

func setupIntegrationEnv(t *testing.T, walletLimit int) *integrationEnv {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.New().String()+"?mode=memory&cache=shared&_txlock=immediate"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&domain.ScanUsageEventEntity{},
		&domain.ScanResultEntity{},
		&domain.TLSScanResultEntity{},
		&domain.User{},
		&domain.Plan{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	planID := uuid.New()
	userID := uuid.New()
	if err := db.Create(&domain.Plan{
		ID: planID, Name: "imm6b8", Type: domain.PlanTypeFree,
		WalletScanLimit: walletLimit, EndpointScanLimit: walletLimit, IsActive: true,
	}).Error; err != nil {
		t.Fatalf("seed plan: %v", err)
	}
	if err := db.Create(&domain.User{
		ID: userID, Email: "imm6b8@test.local", Password: "x", PlanID: planID,
	}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	ledger := repository.NewScanUsageLedgerRepository(db)
	planSvc := service.NewPlanService(
		&integrationPlanRepo{plan: &domain.Plan{
			ID: planID, WalletScanLimit: walletLimit, EndpointScanLimit: walletLimit,
		}},
		&integrationUserRepo{user: &domain.User{ID: userID, PlanID: planID}},
	)
	scanHandler := handlers.NewScanEventHandler(
		storage.NewTLSWriter(db),
		storage.NewWalletWriter(db),
		nil, nil, nil, db, ledger,
		planlimit.NewResolver(repository.NewUserRepository(db), repository.NewPlanRepository(db)),
	)

	return &integrationEnv{
		db:          db,
		userID:      userID,
		planID:      planID,
		ledger:      ledger,
		planSvc:     planSvc,
		scanRepo:    repository.NewScanResultRepository(db),
		scanHandler: scanHandler,
	}
}

type integrationPlanRepo struct {
	plan *domain.Plan
}

func (r *integrationPlanRepo) Create(*domain.Plan) error { return nil }
func (r *integrationPlanRepo) FindByID(uuid.UUID) (*domain.Plan, error) {
	return r.plan, nil
}
func (r *integrationPlanRepo) FindByType(domain.PlanType) (*domain.Plan, error) { return nil, nil }
func (r *integrationPlanRepo) FindAll() ([]*domain.Plan, error)                 { return nil, nil }
func (r *integrationPlanRepo) FindActive() ([]*domain.Plan, error)            { return nil, nil }

type integrationUserRepo struct {
	user *domain.User
}

func (r *integrationUserRepo) Create(*domain.User) error { return nil }
func (r *integrationUserRepo) FindByID(string) (*domain.User, error) {
	return r.user, nil
}
func (r *integrationUserRepo) FindByEmail(string) (*domain.User, error) { return nil, nil }
func (r *integrationUserRepo) ExistsByEmail(string) (bool, error)      { return false, nil }

func seedWalletSuccess(t *testing.T, env *integrationEnv, scanID uuid.UUID, address string) {
	t.Helper()
	row := domain.ScanResultEntity{
		ID: scanID, UserID: env.userID, Address: address,
		Type: domain.AccountTypeEOA, Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: domain.NISTLevel1,
		Status: scan.StateSUCCESS, RiskScore: 1.0, Networks: `["ethereum"]`,
	}
	if err := env.db.Create(&row).Error; err != nil {
		t.Fatalf("seed success row: %v", err)
	}
	if err := env.ledger.RecordSuccessUsage(env.userID, scanID, domain.ScanUsageKindWallet); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}
}

func seedWalletInFlight(t *testing.T, env *integrationEnv, scanID uuid.UUID, address string) {
	t.Helper()
	row := domain.ScanResultEntity{
		ID: scanID, UserID: env.userID, Address: address,
		Type: domain.AccountTypeEOA, Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: domain.NISTLevel1,
		Status: scan.StateRUNNING,
	}
	if err := env.db.Create(&row).Error; err != nil {
		t.Fatalf("seed in-flight row: %v", err)
	}
}

// IMM-6b-8 / G1: POST blocked when successful + in_flight >= limit.
func TestIntegration_IMM6b8_PostBlockedG1(t *testing.T) {
	env := setupIntegrationEnv(t, 5)
	seedWalletSuccess(t, env, uuid.New(), "0xg1a")
	seedWalletSuccess(t, env, uuid.New(), "0xg1b")
	seedWalletSuccess(t, env, uuid.New(), "0xg1c")
	seedWalletSuccess(t, env, uuid.New(), "0xg1d")
	seedWalletInFlight(t, env, uuid.New(), "0xg1e")

	ok, _, deny, err := env.planSvc.CheckPostScanQuota(env.userID, scan.PlanLimitKeyWallet, env.ledger)
	if err != nil {
		t.Fatalf("CheckPostScanQuota: %v", err)
	}
	if ok {
		t.Fatal("expected POST blocked when successful+in_flight >= limit")
	}
	if deny != service.PostScanQuotaDenyQuota {
		t.Fatalf("deny reason = %q, want %q", deny, service.PostScanQuotaDenyQuota)
	}
}

// IMM-6b-8 / G2: POST blocked when in_flight >= min(limit, 3).
func TestIntegration_IMM6b8_PostBlockedG2(t *testing.T) {
	env := setupIntegrationEnv(t, 5)
	seedWalletInFlight(t, env, uuid.New(), "0xg2a")
	seedWalletInFlight(t, env, uuid.New(), "0xg2b")
	seedWalletInFlight(t, env, uuid.New(), "0xg2c")

	ok, _, deny, err := env.planSvc.CheckPostScanQuota(env.userID, scan.PlanLimitKeyWallet, env.ledger)
	if err != nil {
		t.Fatalf("CheckPostScanQuota: %v", err)
	}
	if ok {
		t.Fatal("expected POST blocked when in_flight >= min(limit, 3)")
	}
	if deny != service.PostScanQuotaDenyParallel {
		t.Fatalf("deny reason = %q, want %q", deny, service.PostScanQuotaDenyParallel)
	}
}

func richWalletScanResult(address string) *domain.ScanResult {
	return &domain.ScanResult{
		Address: address, Type: domain.AccountTypeEOA,
		Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: domain.NISTLevel1,
		KeyExposed: true, PublicKey: "0xsecret", RiskScore: 7.5,
		Networks: []string{"ethereum"}, Connections: []string{"peer"},
	}
}

func seedCompletionRaceAtLimit(t *testing.T, env *integrationEnv, address string) (scanA, scanB uuid.UUID) {
	t.Helper()
	seedScan := uuid.New()
	if err := env.ledger.RecordSuccessUsage(env.userID, seedScan, domain.ScanUsageKindWallet); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}
	scanA, scanB = uuid.New(), uuid.New()
	w := storage.NewWalletWriter(env.db)
	for _, id := range []uuid.UUID{scanA, scanB} {
		if err := w.OnStarted(id, env.userID, address); err != nil {
			t.Fatalf("OnStarted: %v", err)
		}
	}
	return scanA, scanB
}

func runConcurrentWalletCompletions(
	t *testing.T,
	env *integrationEnv,
	scanIDs []uuid.UUID,
	address string,
	richResult *domain.ScanResult,
) []bool {
	t.Helper()
	results := make([]bool, len(scanIDs))
	var wg sync.WaitGroup
	for i := range scanIDs {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			msg := &nats.ScanCompletedMessage{
				ScanID: scanIDs[idx], Kind: "wallet", UserID: env.userID, Address: address,
				Result: richResult,
			}
			entity := domain.FromScanResult(env.userID, richResult)
			entity.ID = scanIDs[idx]
			acquired, err := env.scanHandler.CommitWalletCompletionForIntegrationTest(msg, entity, richResult)
			if err != nil {
				t.Errorf("commitWalletCompletion: %v", err)
				return
			}
			results[idx] = acquired
		}(i)
	}
	wg.Wait()
	return results
}

func assertExactlyOneAcquiredSlot(t *testing.T, results []bool) {
	t.Helper()
	var n int
	for _, ok := range results {
		if ok {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("want exactly 1 success slot, got %d (results=%v)", n, results)
	}
}

func assertLedgerSuccessCount(t *testing.T, env *integrationEnv, want int64) {
	t.Helper()
	count, err := env.ledger.CountSuccessUsage(env.userID, domain.ScanUsageKindWallet)
	if err != nil {
		t.Fatalf("ledger count: %v", err)
	}
	if count != want {
		t.Fatalf("want ledger count %d, got %d", want, count)
	}
}

func assertCompletionRaceRows(t *testing.T, env *integrationEnv, scanA, scanB uuid.UUID, address string) {
	t.Helper()
	var rows []domain.ScanResultEntity
	if err := env.db.Where("user_id = ? AND id IN ?", env.userID, []uuid.UUID{scanA, scanB}).Find(&rows).Error; err != nil {
		t.Fatalf("load rows: %v", err)
	}
	var rich, stub int
	for _, row := range rows {
		switch row.Status {
		case scan.StateSUCCESS:
			rich++
			assertRichSuccessRow(t, row)
		case scan.StateFAILED:
			stub++
			assertPlanLimitStubRow(t, row, address)
		default:
			t.Fatalf("unexpected status %s", row.Status)
		}
	}
	if rich != 1 || stub != 1 {
		t.Fatalf("want 1 rich + 1 stub, got rich=%d stub=%d", rich, stub)
	}
}

func assertRichSuccessRow(t *testing.T, row domain.ScanResultEntity) {
	t.Helper()
	if row.PublicKey == "" {
		t.Fatalf("success row must keep rich result: %+v", row)
	}
}

func assertPlanLimitStubRow(t *testing.T, row domain.ScanResultEntity, address string) {
	t.Helper()
	if row.Error != scan.ErrPlanLimitExceeded {
		t.Fatalf("stub error: want %s, got %q", scan.ErrPlanLimitExceeded, row.Error)
	}
	if row.Address != address {
		t.Fatalf("stub must keep address, got %q", row.Address)
	}
	if row.PublicKey != "" || row.KeyExposed {
		t.Fatalf("stub must strip exploitable fields: %+v", row)
	}
}

// IMM-6b-8 / G3: concurrent completion at limit → one rich success, one stub, ledger +1.
func TestIntegration_IMM6b8_CompletionRaceOneRichOneStub(t *testing.T) {
	env := setupIntegrationEnv(t, 2)
	address := "0xrace"
	scanA, scanB := seedCompletionRaceAtLimit(t, env, address)
	richResult := richWalletScanResult(address)

	results := runConcurrentWalletCompletions(t, env, []uuid.UUID{scanA, scanB}, address, richResult)
	assertExactlyOneAcquiredSlot(t, results)
	assertLedgerSuccessCount(t, env, 2)
	assertCompletionRaceRows(t, env, scanA, scanB, address)
}

// IMM-6b-8 / P1-b: DELETE success scan → used unchanged, visible decreases.
func TestIntegration_IMM6b8_DeleteSuccessUsedStableVisibleDown(t *testing.T) {
	env := setupIntegrationEnv(t, 5)
	scanID := uuid.New()
	seedWalletSuccess(t, env, scanID, "0xdelete")

	before, err := env.planSvc.GetPlanUsage(env.userID, env.ledger)
	if err != nil {
		t.Fatalf("GetPlanUsage before: %v", err)
	}
	if before.WalletScansUsed != 1 || before.WalletScansVisible != 1 || before.WalletScansDeletedByUser != 0 {
		t.Fatalf("before delete: used=%d visible=%d deleted=%d, want 1/1/0",
			before.WalletScansUsed, before.WalletScansVisible, before.WalletScansDeletedByUser)
	}

	deleted, err := env.scanRepo.DeleteOwnedWalletScan(env.userID, scanID)
	if err != nil {
		t.Fatalf("DeleteOwnedWalletScan: %v", err)
	}
	if !deleted {
		t.Fatal("expected row deleted")
	}

	after, err := env.planSvc.GetPlanUsage(env.userID, env.ledger)
	if err != nil {
		t.Fatalf("GetPlanUsage after: %v", err)
	}
	if after.WalletScansUsed != before.WalletScansUsed {
		t.Fatalf("used changed on delete: before=%d after=%d", before.WalletScansUsed, after.WalletScansUsed)
	}
	if after.WalletScansVisible != 0 {
		t.Fatalf("visible after delete = %d, want 0", after.WalletScansVisible)
	}
	if after.WalletScansDeletedByUser != 1 {
		t.Fatalf("deleted_by_user after delete = %d, want 1", after.WalletScansDeletedByUser)
	}
}
