package repository

import (
	"cafe-discovery/internal/domain"
	"errors"

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
	db *gorm.DB
}

// NewScanResultRepository creates a new scan result repository
func NewScanResultRepository(db *gorm.DB) ScanResultRepository {
	return &scanResultRepository{
		db: db,
	}
}

// Create creates a new scan result in the database
func (r *scanResultRepository) Create(scanResult *domain.ScanResultEntity) error {
	if err := r.db.Create(scanResult).Error; err != nil {
		return err
	}
	return nil
}

// FindByUserID finds all scan results for a specific user with pagination
func (r *scanResultRepository) FindByUserID(userID uuid.UUID, limit, offset int) ([]*domain.ScanResultEntity, error) {
	var results []*domain.ScanResultEntity
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

// FindByID finds a scan result by ID
func (r *scanResultRepository) FindByID(id uuid.UUID) (*domain.ScanResultEntity, error) {
	var result domain.ScanResultEntity
	res := r.db.Where("id = ?", id).First(&result)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, res.Error
	}
	return &result, nil
}

// FindByUserIDAndAddress finds a scan result by user ID and address
func (r *scanResultRepository) FindByUserIDAndAddress(userID uuid.UUID, address string) (*domain.ScanResultEntity, error) {
	var result domain.ScanResultEntity
	res := r.db.Where("user_id = ? AND address = ?", userID, address).Order("created_at DESC").First(&result)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, res.Error
	}
	return &result, nil
}

// CountByUserID counts the total number of scan results for a user
func (r *scanResultRepository) CountByUserID(userID uuid.UUID) (int64, error) {
	var count int64
	result := r.db.Model(&domain.ScanResultEntity{}).Where("user_id = ?", userID).Count(&count)
	if result.Error != nil {
		return 0, result.Error
	}
	return count, nil
}

