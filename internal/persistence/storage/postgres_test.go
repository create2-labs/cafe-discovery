package storage

import (
	"testing"

	"cafe-discovery/internal/domain"
	"cafe-discovery/pkg/scan"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupWalletWriterTestDB(t *testing.T) *WalletWriter {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&domain.ScanResultEntity{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewWalletWriter(db)
}

func TestWalletWriter_TwoScanIDsSameAddressPreservesTerminalA(t *testing.T) {
	w := setupWalletWriterTestDB(t)
	userID := uuid.New()
	address := "0xabc"
	scanA := uuid.New()
	scanB := uuid.New()

	if err := w.OnStarted(scanA, userID, address); err != nil {
		t.Fatalf("OnStarted A: %v", err)
	}
	entityA := domain.FromScanResult(userID, &domain.ScanResult{
		Address: address, Type: domain.AccountTypeEOA,
		Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: domain.NISTLevel1,
		IsEOA: true, RiskScore: 1.0,
	})
	if err := w.OnCompleted(scanA, entityA); err != nil {
		t.Fatalf("OnCompleted A: %v", err)
	}

	if err := w.OnStarted(scanB, userID, address); err != nil {
		t.Fatalf("OnStarted B: %v", err)
	}
	entityB := domain.FromScanResult(userID, &domain.ScanResult{
		Address: address, Type: domain.AccountTypeEOA,
		Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: domain.NISTLevel1,
		IsEOA: true, RiskScore: 9.0,
	})
	if err := w.OnCompleted(scanB, entityB); err != nil {
		t.Fatalf("OnCompleted B: %v", err)
	}

	var rows []domain.ScanResultEntity
	if err := w.db.Where("user_id = ? AND address = ?", userID, address).Find(&rows).Error; err != nil {
		t.Fatalf("query rows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows for same address, got %d", len(rows))
	}

	statusA, err := w.GetStatus(scanA)
	if err != nil {
		t.Fatalf("GetStatus A: %v", err)
	}
	if statusA != scan.StateSUCCESS {
		t.Fatalf("scan A status: want %s, got %s", scan.StateSUCCESS, statusA)
	}

	var storedA domain.ScanResultEntity
	if err := w.db.Where("id = ?", scanA).First(&storedA).Error; err != nil {
		t.Fatalf("load A: %v", err)
	}
	if storedA.RiskScore != 1.0 {
		t.Fatalf("scan A risk_score: want 1.0, got %v", storedA.RiskScore)
	}

	var storedB domain.ScanResultEntity
	if err := w.db.Where("id = ?", scanB).First(&storedB).Error; err != nil {
		t.Fatalf("load B: %v", err)
	}
	if storedB.RiskScore != 9.0 {
		t.Fatalf("scan B risk_score: want 9.0, got %v", storedB.RiskScore)
	}
}

func TestWalletWriter_OnStartedIdempotentByScanID(t *testing.T) {
	w := setupWalletWriterTestDB(t)
	userID := uuid.New()
	scanID := uuid.New()
	address := "0xdef"

	if err := w.OnStarted(scanID, userID, address); err != nil {
		t.Fatalf("first OnStarted: %v", err)
	}
	if err := w.OnStarted(scanID, userID, address); err != nil {
		t.Fatalf("second OnStarted: %v", err)
	}

	var count int64
	if err := w.db.Model(&domain.ScanResultEntity{}).Where("id = ?", scanID).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("want 1 row after duplicate start, got %d", count)
	}
}
