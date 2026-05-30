package repository

import (
	"testing"
	"time"

	"cafe-discovery/internal/domain"
	"cafe-discovery/pkg/scan"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestBackfillScanUsageLedgerFromHistoricalSuccesses(t *testing.T) {
	db, repo := setupScanUsageLedgerTestDB(t)
	userID := uuid.New()
	successID := uuid.New()
	deletedSuccessID := uuid.New()
	failedID := uuid.New()
	planLimitID := uuid.New()
	runningID := uuid.New()
	endpointID := uuid.New()
	defaultEndpointID := uuid.New()

	deletedAt := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	seed := []domain.ScanResultEntity{
		{ID: successID, UserID: userID, Address: "0xok", Status: scan.StateSUCCESS, Type: domain.AccountTypeEOA, Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: domain.NISTLevel1},
		{ID: deletedSuccessID, UserID: userID, Address: "0xdel", Status: scan.StateSUCCESS, Type: domain.AccountTypeEOA, Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: domain.NISTLevel1, DeletedAt: gorm.DeletedAt{Time: deletedAt, Valid: true}},
		{ID: failedID, UserID: userID, Address: "0xfail", Status: scan.StateFAILED, Error: "scanner error", Type: domain.AccountTypeEOA, Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: domain.NISTLevel1},
		{ID: planLimitID, UserID: userID, Address: "0xlimit", Status: scan.StateFAILED, Error: scan.ErrPlanLimitExceeded, Type: domain.AccountTypeEOA, Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: domain.NISTLevel1},
		{ID: runningID, UserID: userID, Address: "0xrun", Status: scan.StateRUNNING, Type: domain.AccountTypeEOA, Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: domain.NISTLevel1},
	}
	if err := db.Create(&seed).Error; err != nil {
		t.Fatalf("seed wallet rows: %v", err)
	}

	endpointUser := userID
	tlsRows := []domain.TLSScanResultEntity{
		{ID: endpointID, UserID: &endpointUser, URL: "https://example.com", Host: "example.com", Port: 443, ProtocolVersion: "TLS1.3", NISTLevel: domain.NISTLevel1, RiskScore: 0, PQCRisk: "low", Status: scan.StateSUCCESS},
		{ID: defaultEndpointID, UserID: &endpointUser, URL: "https://default.example", Host: "default.example", Port: 443, ProtocolVersion: "TLS1.3", NISTLevel: domain.NISTLevel1, RiskScore: 0, PQCRisk: "low", Status: scan.StateSUCCESS, Default: true},
	}
	if err := db.Create(&tlsRows).Error; err != nil {
		t.Fatalf("seed tls rows: %v", err)
	}

	// Pre-existing ledger row for one wallet success (idempotent skip).
	if err := repo.RecordSuccessUsage(userID, successID, domain.ScanUsageKindWallet); err != nil {
		t.Fatalf("pre-seed ledger: %v", err)
	}

	first, err := BackfillScanUsageLedgerFromHistoricalSuccesses(db)
	if err != nil {
		t.Fatalf("first backfill: %v", err)
	}
	if first.WalletCandidates != 2 {
		t.Fatalf("wallet candidates = %d, want 2 (success + soft-deleted success)", first.WalletCandidates)
	}
	if first.WalletInserted != 1 {
		t.Fatalf("wallet inserted = %d, want 1 (deleted success only)", first.WalletInserted)
	}
	if first.EndpointCandidates != 1 {
		t.Fatalf("endpoint candidates = %d, want 1 (non-default success)", first.EndpointCandidates)
	}
	if first.EndpointInserted != 1 {
		t.Fatalf("endpoint inserted = %d, want 1", first.EndpointInserted)
	}

	usedWallet, err := repo.CountSuccessUsage(userID, domain.ScanUsageKindWallet)
	if err != nil {
		t.Fatalf("CountSuccessUsage wallet: %v", err)
	}
	if usedWallet != 2 {
		t.Fatalf("wallet used = %d, want 2", usedWallet)
	}
	usedEndpoint, err := repo.CountSuccessUsage(userID, domain.ScanUsageKindEndpoint)
	if err != nil {
		t.Fatalf("CountSuccessUsage endpoint: %v", err)
	}
	if usedEndpoint != 1 {
		t.Fatalf("endpoint used = %d, want 1", usedEndpoint)
	}

	second, err := BackfillScanUsageLedgerFromHistoricalSuccesses(db)
	if err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if second.WalletInserted != 0 || second.EndpointInserted != 0 {
		t.Fatalf("second backfill inserted rows: wallet=%d endpoint=%d, want 0/0", second.WalletInserted, second.EndpointInserted)
	}

	var ledgerCount int64
	if err := db.Model(&domain.ScanUsageEventEntity{}).Count(&ledgerCount).Error; err != nil {
		t.Fatalf("count ledger: %v", err)
	}
	if ledgerCount != 3 {
		t.Fatalf("ledger rows = %d, want 3 (no duplicate scan_id)", ledgerCount)
	}
}

func TestBackfillScanUsageLedger_UsedGtePostgresSuccesses(t *testing.T) {
	db, repo := setupScanUsageLedgerTestDB(t)
	userID := uuid.New()

	for i := 0; i < 3; i++ {
		row := domain.ScanResultEntity{
			ID: uuid.New(), UserID: userID, Address: uuid.New().String(), Status: scan.StateSUCCESS,
			Type: domain.AccountTypeEOA, Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: domain.NISTLevel1,
		}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	var pgSuccesses int64
	if err := db.Unscoped().Model(&domain.ScanResultEntity{}).
		Where("user_id = ? AND status = ?", userID, scan.StateSUCCESS).
		Count(&pgSuccesses).Error; err != nil {
		t.Fatalf("count postgres successes: %v", err)
	}

	if _, err := BackfillScanUsageLedgerFromHistoricalSuccesses(db); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	used, err := repo.CountSuccessUsage(userID, domain.ScanUsageKindWallet)
	if err != nil {
		t.Fatalf("CountSuccessUsage: %v", err)
	}
	if used < pgSuccesses {
		t.Fatalf("used=%d < postgres successes=%d", used, pgSuccesses)
	}
}
