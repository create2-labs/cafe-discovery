package storage

import (
	"errors"

	"cafe-discovery/internal/domain"
	"cafe-discovery/pkg/scan"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TLSWriter persists TLS scan state/results (one Postgres row per scan_id).
type TLSWriter struct {
	db *gorm.DB
}

func NewTLSWriter(db *gorm.DB) *TLSWriter {
	return &TLSWriter{db: db}
}

// GetStatus returns the current status for the scan, or "" if not found.
func (w *TLSWriter) GetStatus(scanID uuid.UUID) (string, error) {
	var ent domain.TLSScanResultEntity
	err := w.db.Select("status").Where("id = ?", scanID).First(&ent).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return ent.Status, nil
}

// OnStarted inserts a row for scan_id (internal status RUNNING; API maps to started).
// Idempotent for the same scan_id: no downgrade from terminal; duplicate start is a no-op.
func (w *TLSWriter) OnStarted(scanID uuid.UUID, userID *uuid.UUID, url string) error {
	current, err := w.GetStatus(scanID)
	if err != nil {
		return err
	}
	if current != "" {
		return nil
	}
	ent := &domain.TLSScanResultEntity{
		ID: scanID, UserID: userID, URL: url, Host: "", Port: 0,
		ProtocolVersion: "unknown", NISTLevel: domain.NISTLevel1,
		RiskScore: 0, PQCRisk: "unknown", Status: scan.StateRUNNING,
	}
	return w.db.Create(ent).Error
}

// OnCompleted updates the row by scan_id; inserts on replay when the row is missing.
func (w *TLSWriter) OnCompleted(scanID uuid.UUID, entity *domain.TLSScanResultEntity) error {
	entity.ID = scanID
	entity.Status = scan.StateSUCCESS
	entity.Error = ""
	res := w.db.Model(entity).Where("id = ?", scanID).Omit("created_at").Select("*").Updates(entity)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return w.db.Create(entity).Error
	}
	return nil
}

// OnFailed updates the row by scan_id; inserts on replay when the row is missing.
func (w *TLSWriter) OnFailed(scanID uuid.UUID, userID *uuid.UUID, url, errMsg string) error {
	res := w.db.Model(&domain.TLSScanResultEntity{}).Where("id = ?", scanID).
		Updates(map[string]interface{}{
			"status":     scan.StateFAILED,
			"error":      errMsg,
			"updated_at": gorm.Expr("NOW()"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		ent := &domain.TLSScanResultEntity{
			ID: scanID, UserID: userID, URL: url, Host: "", Port: 0,
			ProtocolVersion: "unknown", NISTLevel: domain.NISTLevel1,
			RiskScore: 0, PQCRisk: "unknown", Status: scan.StateFAILED, Error: errMsg,
		}
		return w.db.Create(ent).Error
	}
	return nil
}

// WalletWriter persists wallet scan state/results (one Postgres row per scan_id).
type WalletWriter struct {
	db *gorm.DB
}

func NewWalletWriter(db *gorm.DB) *WalletWriter {
	return &WalletWriter{db: db}
}

// GetStatus returns the current status for the scan, or "" if not found.
func (w *WalletWriter) GetStatus(scanID uuid.UUID) (string, error) {
	var ent domain.ScanResultEntity
	err := w.db.Select("status").Where("id = ?", scanID).First(&ent).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return ent.Status, nil
}

// OnStarted inserts a row for scan_id (internal status RUNNING; API maps to started).
// A new scan_id for the same address always inserts a new row (no target-level upsert).
func (w *WalletWriter) OnStarted(scanID, userID uuid.UUID, address string) error {
	current, err := w.GetStatus(scanID)
	if err != nil {
		return err
	}
	if current != "" {
		return nil
	}
	ent := &domain.ScanResultEntity{
		ID: scanID, UserID: userID, Address: address, Status: scan.StateRUNNING,
		Type: domain.AccountTypeEOA, Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: domain.NISTLevel1,
		KeyExposed: false, IsEOA: true, IsERC4337: false, RiskScore: 0,
		Networks: "[]", Connections: "[]",
	}
	return w.db.Create(ent).Error
}

// OnCompleted updates the row by scan_id; inserts on replay when the row is missing.
func (w *WalletWriter) OnCompleted(scanID uuid.UUID, entity *domain.ScanResultEntity) error {
	entity.ID = scanID
	entity.Status = scan.StateSUCCESS
	entity.Error = ""
	res := w.db.Model(entity).Where("id = ?", scanID).Omit("created_at").Select("*").Updates(entity)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return w.db.Create(entity).Error
	}
	return nil
}

// OnFailed updates the row by scan_id; inserts on replay when the row is missing.
func (w *WalletWriter) OnFailed(scanID, userID uuid.UUID, address, errMsg string) error {
	res := w.db.Model(&domain.ScanResultEntity{}).Where("id = ?", scanID).
		Updates(map[string]interface{}{
			"status":     scan.StateFAILED,
			"error":      errMsg,
			"updated_at": gorm.Expr("NOW()"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		ent := &domain.ScanResultEntity{
			ID: scanID, UserID: userID, Address: address, Status: scan.StateFAILED, Error: errMsg,
			Type: domain.AccountTypeEOA, Algorithm: domain.AlgorithmECDSAsecp256k1, NISTLevel: domain.NISTLevel1,
			KeyExposed: false, IsEOA: true, IsERC4337: false, RiskScore: 0,
			Networks: "[]", Connections: "[]",
		}
		return w.db.Create(ent).Error
	}
	return nil
}
