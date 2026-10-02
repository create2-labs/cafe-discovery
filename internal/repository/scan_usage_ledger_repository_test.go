package repository

import (
	"sync"
	"testing"

	"cafe-discovery/internal/domain"
	"cafe-discovery/pkg/scan"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupScanUsageLedgerTestDB(t *testing.T) (*gorm.DB, ScanUsageLedgerRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.New().String()+"?mode=memory&cache=shared&_txlock=immediate"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&domain.ScanUsageEventEntity{},
		&domain.ScanResultEntity{},
		&domain.TLSScanResultEntity{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	return db, NewScanUsageLedgerRepository(db)
}

func TestScanUsageLedger_RecordSuccessUsage_Idempotent(t *testing.T) {
	_, repo := setupScanUsageLedgerTestDB(t)
	userID := uuid.New()
	scanID := uuid.New()

	if err := repo.RecordSuccessUsage(userID, scanID, domain.ScanUsageKindWallet); err != nil {
		t.Fatalf("first record: %v", err)
	}
	if err := repo.RecordSuccessUsage(userID, scanID, domain.ScanUsageKindWallet); err != nil {
		t.Fatalf("second record: %v", err)
	}

	count, err := repo.CountSuccessUsage(userID, domain.ScanUsageKindWallet)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("want 1 ledger row, got %d", count)
	}
}

func TestScanUsageLedger_CountInFlightScans(t *testing.T) {
	db, repo := setupScanUsageLedgerTestDB(t)
	userID := uuid.New()

	rows := []domain.ScanResultEntity{
		{ID: uuid.New(), UserID: userID, Address: "0x1", Status: scan.StateRUNNING, Type: domain.AccountTypeEOA, Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: domain.NISTLevel1},
		{ID: uuid.New(), UserID: userID, Address: "0x2", Status: scan.StateSUCCESS, Type: domain.AccountTypeEOA, Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: domain.NISTLevel1},
		{ID: uuid.New(), UserID: userID, Address: "0x3", Status: scan.StateFAILED, Type: domain.AccountTypeEOA, Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: domain.NISTLevel1},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	inFlight, err := repo.CountInFlightScans(userID, domain.ScanUsageKindWallet)
	if err != nil {
		t.Fatalf("CountInFlightScans: %v", err)
	}
	if inFlight != 1 {
		t.Fatalf("want 1 in-flight, got %d", inFlight)
	}

	visible, err := repo.CountVisibleSuccessScans(userID, domain.ScanUsageKindWallet)
	if err != nil {
		t.Fatalf("CountVisibleSuccessScans: %v", err)
	}
	if visible != 1 {
		t.Fatalf("want 1 visible success, got %d", visible)
	}
}

func TestScanUsageLedger_TryAcquireSuccessSlot_AtLimit(t *testing.T) {
	_, repo := setupScanUsageLedgerTestDB(t)
	userID := uuid.New()
	limit := 2

	for i := 0; i < limit; i++ {
		if err := repo.RecordSuccessUsage(userID, uuid.New(), domain.ScanUsageKindWallet); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
	}

	ok, err := repo.TryAcquireSuccessSlot(userID, domain.ScanUsageKindWallet, limit)
	if err != nil {
		t.Fatalf("TryAcquireSuccessSlot: %v", err)
	}
	if ok {
		t.Fatal("expected no slot when count == limit")
	}

	ok, err = repo.TryAcquireSuccessSlot(userID, domain.ScanUsageKindWallet, 0)
	if err != nil || !ok {
		t.Fatalf("unlimited limit: ok=%v err=%v", ok, err)
	}
}

func TestScanUsageLedger_CountVisibleSuccessScans_ExcludesSoftDeleted(t *testing.T) {
	db, repo := setupScanUsageLedgerTestDB(t)
	userID := uuid.New()
	scanID := uuid.New()

	row := domain.ScanResultEntity{
		ID: scanID, UserID: userID, Address: "0xdel", Status: scan.StateSUCCESS,
		Type: domain.AccountTypeEOA, Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: domain.NISTLevel1,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := repo.RecordSuccessUsage(userID, scanID, domain.ScanUsageKindWallet); err != nil {
		t.Fatalf("ledger: %v", err)
	}

	used, err := repo.CountSuccessUsage(userID, domain.ScanUsageKindWallet)
	if err != nil {
		t.Fatalf("CountSuccessUsage: %v", err)
	}
	visible, err := repo.CountVisibleSuccessScans(userID, domain.ScanUsageKindWallet)
	if err != nil {
		t.Fatalf("CountVisibleSuccessScans before delete: %v", err)
	}
	if used != 1 || visible != 1 {
		t.Fatalf("before delete: used=%d visible=%d, want 1/1", used, visible)
	}

	if err := db.Delete(&domain.ScanResultEntity{}, "id = ?", scanID).Error; err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	usedAfter, err := repo.CountSuccessUsage(userID, domain.ScanUsageKindWallet)
	if err != nil {
		t.Fatalf("CountSuccessUsage after delete: %v", err)
	}
	visibleAfter, err := repo.CountVisibleSuccessScans(userID, domain.ScanUsageKindWallet)
	if err != nil {
		t.Fatalf("CountVisibleSuccessScans after delete: %v", err)
	}
	if usedAfter != 1 {
		t.Fatalf("used after delete = %d, want 1 (ledger unchanged)", usedAfter)
	}
	if visibleAfter != 0 {
		t.Fatalf("visible after delete = %d, want 0", visibleAfter)
	}
}

func TestScanUsageLedger_RecordSuccessUsageIfUnderLimit_Concurrent(t *testing.T) {
	db, repo := setupScanUsageLedgerTestDB(t)
	userID := uuid.New()
	limit := 2

	if err := repo.RecordSuccessUsage(userID, uuid.New(), domain.ScanUsageKindWallet); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}

	const workers = 8
	var wg sync.WaitGroup
	var recorded int64
	var mu sync.Mutex

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			scanID := uuid.New()
			var inserted bool
			err := db.Transaction(func(tx *gorm.DB) error {
				var err error
				inserted, err = repo.RecordSuccessUsageIfUnderLimitInTx(tx, userID, scanID, domain.ScanUsageKindWallet, limit)
				return err
			})
			if err != nil {
				t.Errorf("transaction: %v", err)
				return
			}
			if inserted {
				mu.Lock()
				recorded++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	final, err := repo.CountSuccessUsage(userID, domain.ScanUsageKindWallet)
	if err != nil {
		t.Fatalf("final count: %v", err)
	}
	if final != int64(limit) {
		t.Fatalf("want ledger count %d after concurrent insert, got %d (recorded goroutines=%d)", limit, final, recorded)
	}
	if recorded != 1 {
		t.Fatalf("want exactly 1 concurrent slot taker, got %d", recorded)
	}
}

func TestScanUsageLedger_Reserve_ConcurrentOneCredit(t *testing.T) {
	_, repo := setupScanUsageLedgerTestDB(t)
	userID := uuid.New()

	const workers = 8
	var wg sync.WaitGroup
	var reserved int64
	var mu sync.Mutex

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := repo.ReserveScanUsage(userID, uuid.New(), domain.ScanUsageKindWallet, 1, 3)
			if err != nil {
				t.Errorf("reserve: %v", err)
				return
			}
			if res.Reserved {
				mu.Lock()
				reserved++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	final, err := repo.CountSuccessUsage(userID, domain.ScanUsageKindWallet)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if final != 1 || reserved != 1 {
		t.Fatalf("want 1 reservation, ledger=%d reserved=%d", final, reserved)
	}
}

func TestScanUsageLedger_Reserve_QuotaFullLeavesLedgerUnchanged(t *testing.T) {
	_, repo := setupScanUsageLedgerTestDB(t)
	userID := uuid.New()
	if err := repo.RecordSuccessUsage(userID, uuid.New(), domain.ScanUsageKindWallet); err != nil {
		t.Fatalf("seed: %v", err)
	}

	res, err := repo.ReserveScanUsage(userID, uuid.New(), domain.ScanUsageKindWallet, 1, 3)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if res.Reserved || res.Deny != ScanUsageDenyQuota {
		t.Fatalf("reserved=%v deny=%q", res.Reserved, res.Deny)
	}
	count, err := repo.CountSuccessUsage(userID, domain.ScanUsageKindWallet)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("ledger count = %d, want 1", count)
	}
}

func TestScanUsageLedger_Reserve_ParallelCapOnOpenReservations(t *testing.T) {
	_, repo := setupScanUsageLedgerTestDB(t)
	userID := uuid.New()
	for i := 0; i < 3; i++ {
		res, err := repo.ReserveScanUsage(userID, uuid.New(), domain.ScanUsageKindWallet, 100, 3)
		if err != nil || !res.Reserved {
			t.Fatalf("seed reserve %d: reserved=%v err=%v deny=%q", i, res.Reserved, err, res.Deny)
		}
	}

	res, err := repo.ReserveScanUsage(userID, uuid.New(), domain.ScanUsageKindWallet, 100, 3)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if res.Reserved || res.Deny != ScanUsageDenyParallel {
		t.Fatalf("reserved=%v deny=%q parallel=%d", res.Reserved, res.Deny, res.ParallelInFlight)
	}
	count, err := repo.CountSuccessUsage(userID, domain.ScanUsageKindWallet)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Fatalf("ledger count = %d, want 3", count)
	}
}

func TestScanUsageLedger_Reserve_DoesNotDoubleCountInFlightWithLedger(t *testing.T) {
	db, repo := setupScanUsageLedgerTestDB(t)
	userID := uuid.New()
	scanID := uuid.New()
	if err := repo.RecordSuccessUsage(userID, scanID, domain.ScanUsageKindWallet); err != nil {
		t.Fatalf("ledger: %v", err)
	}
	row := domain.ScanResultEntity{
		ID: scanID, UserID: userID, Address: "0xrun", Status: scan.StateRUNNING,
		Type: domain.AccountTypeEOA, Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: domain.NISTLevel1,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed scan: %v", err)
	}

	res, err := repo.ReserveScanUsage(userID, uuid.New(), domain.ScanUsageKindWallet, 2, 3)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if !res.Reserved {
		t.Fatalf("expected reservation under limit 2 with one in-flight ledger row, deny=%q ledger=%d extra=%d", res.Deny, res.LedgerCount, res.ExtraInFlight)
	}
}

func TestScanUsageLedger_Reserve_LegacyInFlightWithoutLedgerCountsTowardQuota(t *testing.T) {
	db, repo := setupScanUsageLedgerTestDB(t)
	userID := uuid.New()
	row := domain.ScanResultEntity{
		ID: uuid.New(), UserID: userID, Address: "0xlegacy", Status: scan.StatePENDING,
		Type: domain.AccountTypeEOA, Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: domain.NISTLevel1,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed scan: %v", err)
	}

	res, err := repo.ReserveScanUsage(userID, uuid.New(), domain.ScanUsageKindWallet, 1, 3)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if res.Reserved || res.Deny != ScanUsageDenyQuota {
		t.Fatalf("reserved=%v deny=%q", res.Reserved, res.Deny)
	}
	count, err := repo.CountSuccessUsage(userID, domain.ScanUsageKindWallet)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("ledger count = %d, want 0", count)
	}
}

func TestScanUsageLedger_Reserve_CompletedScanDoesNotUseParallelSlot(t *testing.T) {
	db, repo := setupScanUsageLedgerTestDB(t)
	userID := uuid.New()
	scanID := uuid.New()
	if err := repo.RecordSuccessUsage(userID, scanID, domain.ScanUsageKindWallet); err != nil {
		t.Fatalf("ledger: %v", err)
	}
	row := domain.ScanResultEntity{
		ID: scanID, UserID: userID, Address: "0xdone", Status: scan.StateSUCCESS,
		Type: domain.AccountTypeEOA, Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: domain.NISTLevel1,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed scan: %v", err)
	}

	res, err := repo.ReserveScanUsage(userID, uuid.New(), domain.ScanUsageKindWallet, 10, 1)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if !res.Reserved {
		t.Fatalf("completed scan must not fill the parallel cap, deny=%q parallel=%d", res.Deny, res.ParallelInFlight)
	}
}

func TestScanUsageLedger_Reserve_SoftDeletedResultIsNotAnOpenReservation(t *testing.T) {
	db, repo := setupScanUsageLedgerTestDB(t)
	userID := uuid.New()
	scanID := uuid.New()
	if err := repo.RecordSuccessUsage(userID, scanID, domain.ScanUsageKindWallet); err != nil {
		t.Fatalf("ledger: %v", err)
	}
	row := domain.ScanResultEntity{
		ID: scanID, UserID: userID, Address: "0xdel", Status: scan.StateSUCCESS,
		Type: domain.AccountTypeEOA, Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: domain.NISTLevel1,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed scan: %v", err)
	}
	if err := db.Delete(&domain.ScanResultEntity{}, "id = ?", scanID).Error; err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	res, err := repo.ReserveScanUsage(userID, uuid.New(), domain.ScanUsageKindWallet, 10, 1)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if !res.Reserved {
		t.Fatalf("soft-deleted result must not count as in progress, deny=%q parallel=%d", res.Deny, res.ParallelInFlight)
	}
}

func TestScanUsageLedger_ReleaseSuccessUsageByScanID(t *testing.T) {
	_, repo := setupScanUsageLedgerTestDB(t)
	userID := uuid.New()
	scanID := uuid.New()
	res, err := repo.ReserveScanUsage(userID, scanID, domain.ScanUsageKindEndpoint, 1, 3)
	if err != nil || !res.Reserved {
		t.Fatalf("reserve: reserved=%v err=%v", res.Reserved, err)
	}
	if err := repo.ReleaseSuccessUsageByScanID(scanID); err != nil {
		t.Fatalf("release: %v", err)
	}
	count, err := repo.CountSuccessUsage(userID, domain.ScanUsageKindEndpoint)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("ledger count = %d, want 0", count)
	}
}

func TestScanUsageLedger_Reserve_EndpointLegacyInFlight(t *testing.T) {
	db, repo := setupScanUsageLedgerTestDB(t)
	userID := uuid.New()
	row := domain.TLSScanResultEntity{
		ID: uuid.New(), UserID: &userID, URL: "https://example.com", Host: "example.com", Port: 443,
		ProtocolVersion: "TLS1.3", NISTLevel: domain.NISTLevel1, PQCRisk: "unknown",
		Status: scan.StateRUNNING, Default: false,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed tls: %v", err)
	}

	res, err := repo.ReserveScanUsage(userID, uuid.New(), domain.ScanUsageKindEndpoint, 1, 3)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if res.Reserved || res.Deny != ScanUsageDenyQuota {
		t.Fatalf("reserved=%v deny=%q", res.Reserved, res.Deny)
	}
}

func TestListOpenScanReservations_TerminalResultIsExcluded(t *testing.T) {
	db, repo := setupScanUsageLedgerTestDB(t)
	userID := uuid.New()
	openID := uuid.New()
	doneID := uuid.New()
	runningID := uuid.New()
	for _, id := range []uuid.UUID{openID, doneID, runningID} {
		if _, err := repo.ReserveScanUsage(userID, id, domain.ScanUsageKindWallet, 10, 10); err != nil {
			t.Fatalf("reserve %s: %v", id, err)
		}
	}
	if err := db.Create(&domain.ScanResultEntity{
		ID: doneID, UserID: userID, Address: "0x1111111111111111111111111111111111111111",
		Type: domain.AccountTypeEOA, Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: domain.NISTLevel1,
		Status: scan.StateSUCCESS,
	}).Error; err != nil {
		t.Fatalf("seed success: %v", err)
	}
	if err := db.Create(&domain.ScanResultEntity{
		ID: runningID, UserID: userID, Address: "0x2222222222222222222222222222222222222222",
		Type: domain.AccountTypeEOA, Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: domain.NISTLevel1,
		Status: scan.StateRUNNING,
	}).Error; err != nil {
		t.Fatalf("seed running: %v", err)
	}

	rows, err := repo.ListOpenScanReservations()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	got := map[uuid.UUID]struct{}{}
	for _, row := range rows {
		got[row.ScanID] = struct{}{}
	}
	if _, ok := got[doneID]; ok {
		t.Fatal("completed scan is still open")
	}
	if _, ok := got[openID]; !ok {
		t.Fatal("reservation without a result was not listed")
	}
	if _, ok := got[runningID]; !ok {
		t.Fatal("running scan was not listed")
	}
}
