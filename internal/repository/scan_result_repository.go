//nolint:dupl // This repository follows the same pattern as TLSScanResultRepository but uses different domain types
package repository

import (
	"errors"
	"strings"

	"cafe-discovery/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ScanResultRepository defines the interface for scan result data access operations
type ScanResultRepository interface {
	Create(scanResult *domain.ScanResultEntity) error
	FindByUserID(userID uuid.UUID, limit, offset int) ([]*domain.ScanResultEntity, error)
	FindByID(id uuid.UUID) (*domain.ScanResultEntity, error)
	FindByUserIDAndAddress(userID uuid.UUID, address string) (*domain.ScanResultEntity, error)
	FindOwnedWalletScanByID(userID, scanID uuid.UUID) (*domain.ScanResultEntity, error)
	ListOwnerWalletScansDiscoveryV1(userID uuid.UUID, address string, limit, offset int) ([]*domain.ScanResultEntity, int64, error)
	CountByUserID(userID uuid.UUID) (int64, error)
}

// scanResultRepository implements ScanResultRepository interface
type scanResultRepository struct {
	*baseRepository[domain.ScanResultEntity]
}

// NewScanResultRepository creates a new scan result repository
func NewScanResultRepository(db *gorm.DB) ScanResultRepository {
	return &scanResultRepository{
		baseRepository: &baseRepository[domain.ScanResultEntity]{db: db},
	}
}

// Create creates a new scan result in the database
func (r *scanResultRepository) Create(scanResult *domain.ScanResultEntity) error {
	return r.create(scanResult)
}

// FindByUserID finds all scan results for a specific user with pagination
func (r *scanResultRepository) FindByUserID(userID uuid.UUID, limit, offset int) ([]*domain.ScanResultEntity, error) {
	return r.findByUserIDWithResults(userID, limit, offset)
}

// FindByID finds a scan result by ID
func (r *scanResultRepository) FindByID(id uuid.UUID) (*domain.ScanResultEntity, error) {
	var result domain.ScanResultEntity
	return r.findByID(id, &result)
}

// FindByUserIDAndAddress finds a scan result by user ID and address
func (r *scanResultRepository) FindByUserIDAndAddress(userID uuid.UUID, address string) (*domain.ScanResultEntity, error) {
	var result domain.ScanResultEntity
	return r.findByUserIDAndField(userID, "address", address, &result)
}

// FindOwnedWalletScanByID returns a wallet scan row owned by userID, or nil if none.
func (r *scanResultRepository) FindOwnedWalletScanByID(userID, scanID uuid.UUID) (*domain.ScanResultEntity, error) {
	var ent domain.ScanResultEntity
	err := r.db.Where("id = ? AND user_id = ?", scanID, userID).First(&ent).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &ent, nil
}

// ListOwnerWalletScansDiscoveryV1 lists owner wallet scans ordered by created_at DESC, id DESC (WORKPLAN_API §2.2).
// address, when non-empty, filters by case-insensitive hex address match.
func (r *scanResultRepository) ListOwnerWalletScansDiscoveryV1(userID uuid.UUID, address string, limit, offset int) ([]*domain.ScanResultEntity, int64, error) {
	q := r.db.Model(&domain.ScanResultEntity{}).Where("user_id = ?", userID)
	if strings.TrimSpace(address) != "" {
		q = q.Where("LOWER(address) = ?", strings.ToLower(strings.TrimSpace(address)))
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []*domain.ScanResultEntity
	tx := r.db.Where("user_id = ?", userID)
	if strings.TrimSpace(address) != "" {
		tx = tx.Where("LOWER(address) = ?", strings.ToLower(strings.TrimSpace(address)))
	}
	err := tx.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&out).Error
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// CountByUserID counts the total number of scan results for a user
func (r *scanResultRepository) CountByUserID(userID uuid.UUID) (int64, error) {
	return r.countByUserID(userID, &domain.ScanResultEntity{})
}
