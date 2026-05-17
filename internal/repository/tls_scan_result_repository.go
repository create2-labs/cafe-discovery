//nolint:dupl // This repository follows the same pattern as ScanResultRepository but uses different domain types
package repository

import (
	"errors"

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
	FindDefaultTLSScanByID(scanID uuid.UUID) (*domain.TLSScanResultEntity, error)
	FindOwnedUserTLSScanByID(userID, scanID uuid.UUID) (*domain.TLSScanResultEntity, error)
	DeleteOwnedUserTLSScan(userID, scanID uuid.UUID) (deleted bool, err error)
	ListOwnerUserTLSScansDiscoveryV1(userID uuid.UUID, limit, offset int) ([]*domain.TLSScanResultEntity, int64, error)
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

// FindDefaultTLSScanByID returns a shared-catalog default TLS scan by scan_id, or nil.
func (r *tlsScanResultRepository) FindDefaultTLSScanByID(scanID uuid.UUID) (*domain.TLSScanResultEntity, error) {
	var ent domain.TLSScanResultEntity
	err := r.db.Where("id = ? AND \"default\" = ?", scanID, true).First(&ent).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &ent, nil
}

// FindOwnedUserTLSScanByID returns a non-default TLS scan owned by userID, or nil.
func (r *tlsScanResultRepository) FindOwnedUserTLSScanByID(userID, scanID uuid.UUID) (*domain.TLSScanResultEntity, error) {
	var ent domain.TLSScanResultEntity
	err := r.db.Where("id = ? AND user_id = ? AND \"default\" = ?", scanID, userID, false).First(&ent).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &ent, nil
}

// DeleteOwnedUserTLSScan soft-deletes a non-default TLS scan row owned by userID.
func (r *tlsScanResultRepository) DeleteOwnedUserTLSScan(userID, scanID uuid.UUID) (bool, error) {
	res := r.db.Where("id = ? AND user_id = ? AND \"default\" = ?", scanID, userID, false).Delete(&domain.TLSScanResultEntity{})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// ListOwnerUserTLSScansDiscoveryV1 lists TLS scans for the owner excluding catalog defaults (WORKPLAN_API §0.1).
func (r *tlsScanResultRepository) ListOwnerUserTLSScansDiscoveryV1(userID uuid.UUID, limit, offset int) ([]*domain.TLSScanResultEntity, int64, error) {
	q := r.db.Model(&domain.TLSScanResultEntity{}).Where("user_id = ? AND \"default\" = ?", userID, false)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []*domain.TLSScanResultEntity
	err := r.db.Where("user_id = ? AND \"default\" = ?", userID, false).
		Order("created_at DESC, id DESC").
		Limit(limit).Offset(offset).
		Find(&out).Error
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
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
