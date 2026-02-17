package storage

import (
	"errors"

	"cafe-discovery/internal/domain"
	"cafe-discovery/pkg/scan"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TLSWriter upserts TLS scan state/results (idempotent by scan_id).
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

// OnStarted creates or updates a row with status RUNNING (idempotent). If row exists and is terminal, no-op (returns nil).
func (w *TLSWriter) OnStarted(scanID uuid.UUID, userID *uuid.UUID, url string) error {
	current, err := w.GetStatus(scanID)
	if err != nil {
		return err
	}
	if scan.IsTerminal(current) {
		return nil // do not downgrade terminal state
	}
	ent := &domain.TLSScanResultEntity{
		ID: scanID, UserID: userID, URL: url, Host: "", Port: 0, Status: scan.StateRUNNING,
	}
	return w.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"status", "updated_at"}),
	}).Create(ent).Error
}

// OnCompleted updates row to SUCCESS and full result data (idempotent).
func (w *TLSWriter) OnCompleted(scanID uuid.UUID, entity *domain.TLSScanResultEntity) error {
	entity.ID = scanID
	entity.Status = scan.StateSUCCESS
	entity.Error = ""
	return w.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"user_id", "url", "host", "port", "protocol_version", "nist_level", "risk_score", "pqc_risk",
			"certificate", "cipher_suites", "supported_pqcs", "recommendations",
			"kex_algorithm", "kex_pqc_ready", "pqc_mode", "pfs", "alpn", "ocsp_stapled", "curve", "nist_levels", "default",
			"status", "error", "updated_at",
		}),
	}).Create(entity).Error
}

// OnFailed updates row to FAILED; if no row exists, creates one (idempotent).
func (w *TLSWriter) OnFailed(scanID uuid.UUID, userID *uuid.UUID, url, errMsg string) error {
	res := w.db.Model(&domain.TLSScanResultEntity{}).Where("id = ?", scanID).
		Updates(map[string]interface{}{
			"status":     scan.StateFAILED,
			"error":      errMsg,
			"user_id":    userID,
			"url":        url,
			"updated_at": gorm.Expr("NOW()"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		ent := &domain.TLSScanResultEntity{
			ID: scanID, UserID: userID, URL: url, Status: scan.StateFAILED, Error: errMsg,
		}
		return w.db.Create(ent).Error
	}
	return nil
}

// WalletWriter upserts wallet scan state/results (idempotent by scan_id).
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

// OnStarted creates or updates a row with status RUNNING (idempotent). If row exists and is terminal, no-op.
func (w *WalletWriter) OnStarted(scanID, userID uuid.UUID, address string) error {
	current, err := w.GetStatus(scanID)
	if err != nil {
		return err
	}
	if scan.IsTerminal(current) {
		return nil
	}
	ent := &domain.ScanResultEntity{
		ID: scanID, UserID: userID, Address: address, Status: scan.StateRUNNING,
	}
	return w.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"status", "updated_at"}),
	}).Create(ent).Error
}

// OnCompleted updates row to SUCCESS and full result data (idempotent).
func (w *WalletWriter) OnCompleted(scanID uuid.UUID, entity *domain.ScanResultEntity) error {
	entity.ID = scanID
	entity.Status = scan.StateSUCCESS
	entity.Error = ""
	return w.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"user_id", "address", "type", "algorithm", "nist_level", "key_exposed",
			"public_key", "transaction_hash", "exposed_network", "is_eoa", "is_erc4337", "risk_score",
			"networks", "connections", "status", "error", "updated_at",
		}),
	}).Create(entity).Error
}

// OnFailed updates row to FAILED; if no row exists, creates one (idempotent).
func (w *WalletWriter) OnFailed(scanID, userID uuid.UUID, address, errMsg string) error {
	res := w.db.Model(&domain.ScanResultEntity{}).Where("id = ?", scanID).
		Updates(map[string]interface{}{
			"status":     scan.StateFAILED,
			"error":      errMsg,
			"user_id":    userID,
			"address":    address,
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
		}
		return w.db.Create(ent).Error
	}
	return nil
}
