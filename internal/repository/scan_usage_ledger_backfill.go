package repository

import (
	"time"

	"cafe-discovery/internal/domain"
	"cafe-discovery/pkg/scan"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ScanUsageBackfillResult summarizes a historical ledger backfill run (IMM-6b-6).
type ScanUsageBackfillResult struct {
	WalletCandidates   int64
	WalletInserted     int64
	EndpointCandidates int64
	EndpointInserted   int64
}

// BackfillScanUsageLedgerFromHistoricalSuccesses inserts ledger events for existing
// completed-success scan rows that predate IMM-6b-4. Includes soft-deleted successes;
// excludes failed, plan_limit_exceeded, in-flight, default TLS endpoints, and nil user_id.
// Idempotent: safe to run on every persistence boot (ON CONFLICT scan_id DO NOTHING).
func BackfillScanUsageLedgerFromHistoricalSuccesses(db *gorm.DB) (ScanUsageBackfillResult, error) {
	var result ScanUsageBackfillResult

	walletBefore, err := countLedgerEvents(db, domain.ScanUsageKindWallet)
	if err != nil {
		return result, err
	}
	endpointBefore, err := countLedgerEvents(db, domain.ScanUsageKindEndpoint)
	if err != nil {
		return result, err
	}

	walletRows, err := listHistoricalWalletSuccesses(db)
	if err != nil {
		return result, err
	}
	result.WalletCandidates = int64(len(walletRows))
	for _, row := range walletRows {
		if err := insertLedgerEvent(db, row.UserID, row.ID, domain.ScanUsageKindWallet, row.ConsumedAt); err != nil {
			return result, err
		}
	}

	endpointRows, err := listHistoricalEndpointSuccesses(db)
	if err != nil {
		return result, err
	}
	result.EndpointCandidates = int64(len(endpointRows))
	for _, row := range endpointRows {
		if err := insertLedgerEvent(db, row.UserID, row.ID, domain.ScanUsageKindEndpoint, row.ConsumedAt); err != nil {
			return result, err
		}
	}

	walletAfter, err := countLedgerEvents(db, domain.ScanUsageKindWallet)
	if err != nil {
		return result, err
	}
	endpointAfter, err := countLedgerEvents(db, domain.ScanUsageKindEndpoint)
	if err != nil {
		return result, err
	}
	result.WalletInserted = walletAfter - walletBefore
	result.EndpointInserted = endpointAfter - endpointBefore
	return result, nil
}

type historicalSuccessRow struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	ConsumedAt time.Time
}

func countLedgerEvents(db *gorm.DB, kind domain.ScanUsageKind) (int64, error) {
	var count int64
	err := db.Model(&domain.ScanUsageEventEntity{}).
		Where("scan_kind = ?", kind).
		Count(&count).Error
	return count, err
}

func listHistoricalWalletSuccesses(db *gorm.DB) ([]historicalSuccessRow, error) {
	var entities []domain.ScanResultEntity
	err := db.Unscoped().
		Select("id", "user_id", "updated_at", "created_at").
		Where("status = ?", scan.StateSUCCESS).
		Find(&entities).Error
	if err != nil {
		return nil, err
	}
	rows := make([]historicalSuccessRow, 0, len(entities))
	for _, e := range entities {
		if e.UserID == uuid.Nil {
			continue
		}
		rows = append(rows, historicalSuccessRow{
			ID:         e.ID,
			UserID:     e.UserID,
			ConsumedAt: consumedAtFromScanTimestamps(e.UpdatedAt, e.CreatedAt),
		})
	}
	return rows, nil
}

func listHistoricalEndpointSuccesses(db *gorm.DB) ([]historicalSuccessRow, error) {
	var entities []domain.TLSScanResultEntity
	err := db.Unscoped().
		Select("id", "user_id", "updated_at", "created_at").
		Where("status = ? AND \"default\" = ?", scan.StateSUCCESS, false).
		Where("user_id IS NOT NULL").
		Find(&entities).Error
	if err != nil {
		return nil, err
	}
	rows := make([]historicalSuccessRow, 0, len(entities))
	for _, e := range entities {
		if e.UserID == nil || *e.UserID == uuid.Nil {
			continue
		}
		rows = append(rows, historicalSuccessRow{
			ID:         e.ID,
			UserID:     *e.UserID,
			ConsumedAt: consumedAtFromScanTimestamps(e.UpdatedAt, e.CreatedAt),
		})
	}
	return rows, nil
}

func consumedAtFromScanTimestamps(updatedAt, createdAt time.Time) time.Time {
	if !updatedAt.IsZero() {
		return updatedAt.UTC()
	}
	if !createdAt.IsZero() {
		return createdAt.UTC()
	}
	return time.Now().UTC()
}

func insertLedgerEvent(db *gorm.DB, userID, scanID uuid.UUID, kind domain.ScanUsageKind, consumedAt time.Time) error {
	if err := validateScanUsageKind(kind); err != nil {
		return err
	}
	event := &domain.ScanUsageEventEntity{
		UserID:     userID,
		ScanID:     scanID,
		ScanKind:   kind,
		ConsumedAt: consumedAt,
	}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "scan_id"}},
		DoNothing: true,
	}).Create(event).Error
}
