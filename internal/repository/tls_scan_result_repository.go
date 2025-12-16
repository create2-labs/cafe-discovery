package repository

import (
	"cafe-discovery/internal/domain"
	"errors"

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
	db *gorm.DB
}

// NewTLSScanResultRepository creates a new TLS scan result repository
func NewTLSScanResultRepository(db *gorm.DB) TLSScanResultRepository {
	return &tlsScanResultRepository{
		db: db,
	}
}

// Create creates a new TLS scan result in the database
func (r *tlsScanResultRepository) Create(tlsScanResult *domain.TLSScanResultEntity) error {
	if err := r.db.Create(tlsScanResult).Error; err != nil {
		return err
	}
	return nil
}

// FindByUserID finds all TLS scan results for a specific user with pagination
func (r *tlsScanResultRepository) FindByUserID(userID uuid.UUID, limit, offset int) ([]*domain.TLSScanResultEntity, error) {
	var results []*domain.TLSScanResultEntity
	query := r.db.Where("user_id = ?", userID).Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	result := query.Find(&results)
	if result.Error != nil {
		return nil, result.Error
	}
	return results, nil
}

// FindByID finds a TLS scan result by ID
func (r *tlsScanResultRepository) FindByID(id uuid.UUID) (*domain.TLSScanResultEntity, error) {
	var result domain.TLSScanResultEntity
	res := r.db.Where("id = ?", id).First(&result)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, res.Error
	}
	return &result, nil
}

// FindByUserIDAndURL finds a TLS scan result by user ID and URL
func (r *tlsScanResultRepository) FindByUserIDAndURL(userID uuid.UUID, url string) (*domain.TLSScanResultEntity, error) {
	var result domain.TLSScanResultEntity
	res := r.db.Where("user_id = ? AND url = ?", userID, url).Order("created_at DESC").First(&result)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, res.Error
	}
	return &result, nil
}

// CountByUserID counts the total number of TLS scan results for a user
func (r *tlsScanResultRepository) CountByUserID(userID uuid.UUID) (int64, error) {
	var count int64
	result := r.db.Model(&domain.TLSScanResultEntity{}).Where("user_id = ?", userID).Count(&count)
	if result.Error != nil {
		return 0, result.Error
	}
	return count, nil
}

