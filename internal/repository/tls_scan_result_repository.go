//nolint:dupl // This repository follows the same pattern as ScanResultRepository but uses different domain types
package repository

import (
	"cafe-discovery/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TLSScanResultRepository defines the interface for TLS scan result data access operations
type TLSScanResultRepository interface {
	Create(tlsScanResult *domain.TLSScanResultEntity) error
	FindByUserID(userID uuid.UUID, limit, offset int) ([]*domain.TLSScanResultEntity, error)
	FindByID(id uuid.UUID) (*domain.TLSScanResultEntity, error)
	FindByUserIDAndURL(userID uuid.UUID, url string) (*domain.TLSScanResultEntity, error)
	CountByUserID(userID uuid.UUID) (int64, error)
}

// tlsScanResultRepository implements TLSScanResultRepository interface
type tlsScanResultRepository struct {
	*baseRepository[domain.TLSScanResultEntity]
}

// NewTLSScanResultRepository creates a new TLS scan result repository
func NewTLSScanResultRepository(db *gorm.DB) TLSScanResultRepository {
	return &tlsScanResultRepository{
		baseRepository: &baseRepository[domain.TLSScanResultEntity]{db: db},
	}
}

// Create creates a new TLS scan result in the database
func (r *tlsScanResultRepository) Create(tlsScanResult *domain.TLSScanResultEntity) error {
	return r.create(tlsScanResult)
}

// FindByUserID finds all TLS scan results for a specific user with pagination
func (r *tlsScanResultRepository) FindByUserID(userID uuid.UUID, limit, offset int) ([]*domain.TLSScanResultEntity, error) {
	return r.findByUserIDWithResults(userID, limit, offset)
}

// FindByID finds a TLS scan result by ID
func (r *tlsScanResultRepository) FindByID(id uuid.UUID) (*domain.TLSScanResultEntity, error) {
	var result domain.TLSScanResultEntity
	return r.findByID(id, &result)
}

// FindByUserIDAndURL finds a TLS scan result by user ID and URL
func (r *tlsScanResultRepository) FindByUserIDAndURL(userID uuid.UUID, url string) (*domain.TLSScanResultEntity, error) {
	var result domain.TLSScanResultEntity
	return r.findByUserIDAndField(userID, "url", url, &result)
}

// CountByUserID counts the total number of TLS scan results for a user
func (r *tlsScanResultRepository) CountByUserID(userID uuid.UUID) (int64, error) {
	return r.countByUserID(userID, &domain.TLSScanResultEntity{})
}
