//nolint:dupl // This repository follows the same pattern as TLSScanResultRepository but uses different domain types
package repository

import (
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

// CountByUserID counts the total number of scan results for a user
func (r *scanResultRepository) CountByUserID(userID uuid.UUID) (int64, error) {
	return r.countByUserID(userID, &domain.ScanResultEntity{})
}
