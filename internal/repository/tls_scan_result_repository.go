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
	FindByUserIDOrDefault(userID uuid.UUID, limit, offset int) ([]*domain.TLSScanResultEntity, error)
	FindByID(id uuid.UUID) (*domain.TLSScanResultEntity, error)
	FindByUserIDAndURL(userID uuid.UUID, url string) (*domain.TLSScanResultEntity, error)
	FindByURL(url string) (*domain.TLSScanResultEntity, error)
	FindDefaultByURL(url string) (*domain.TLSScanResultEntity, error)
	FindAllDefault() ([]*domain.TLSScanResultEntity, error)
	CountByUserID(userID uuid.UUID) (int64, error)
	CountByUserIDOrDefault(userID uuid.UUID) (int64, error)
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

// FindByURL finds a TLS scan result by URL (regardless of user)
func (r *tlsScanResultRepository) FindByURL(url string) (*domain.TLSScanResultEntity, error) {
	var result domain.TLSScanResultEntity
	if err := r.db.Where("url = ?", url).Order("created_at DESC").First(&result).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}

// FindDefaultByURL finds a default TLS scan result by URL (default=true)
func (r *tlsScanResultRepository) FindDefaultByURL(url string) (*domain.TLSScanResultEntity, error) {
	var result domain.TLSScanResultEntity
	if err := r.db.Where("url = ? AND \"default\" = ?", url, true).Order("created_at DESC").First(&result).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}

// FindByUserIDOrDefault finds all TLS scan results for a user OR default endpoints (default=true)
// This allows users to see both their own scans and default endpoints
func (r *tlsScanResultRepository) FindByUserIDOrDefault(userID uuid.UUID, limit, offset int) ([]*domain.TLSScanResultEntity, error) {
	var results []*domain.TLSScanResultEntity
	query := r.db.Where("user_id = ? OR \"default\" = ?", userID, true).Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

// FindAllDefault finds all default TLS scan results (default=true)
func (r *tlsScanResultRepository) FindAllDefault() ([]*domain.TLSScanResultEntity, error) {
	var results []*domain.TLSScanResultEntity
	if err := r.db.Where("\"default\" = ?", true).Order("created_at DESC").Find(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

// CountByUserID counts the total number of TLS scan results for a user
func (r *tlsScanResultRepository) CountByUserID(userID uuid.UUID) (int64, error) {
	return r.countByUserID(userID, &domain.TLSScanResultEntity{})
}

// CountByUserIDOrDefault counts the total number of TLS scan results for a user OR default endpoints
func (r *tlsScanResultRepository) CountByUserIDOrDefault(userID uuid.UUID) (int64, error) {
	var count int64
	result := r.db.Model(&domain.TLSScanResultEntity{}).Where("user_id = ? OR \"default\" = ?", userID, true).Count(&count)
	if result.Error != nil {
		return 0, result.Error
	}
	return count, nil
}
