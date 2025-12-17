package repository

import (
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// baseRepository provides generic CRUD operations for entities with UserID
// T must be a struct type that has a UserID field and implements gorm.Model or has CreatedAt
type baseRepository[T any] struct {
	db *gorm.DB
}

// create creates a new entity in the database
func (r *baseRepository[T]) create(entity *T) error {
	if err := r.db.Create(entity).Error; err != nil {
		return err
	}
	return nil
}

// findByUserID finds all entities for a specific user with pagination
func (r *baseRepository[T]) findByUserID(userID uuid.UUID, limit, offset int, results *[]*T) error {
	query := r.db.Where("user_id = ?", userID).Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	result := query.Find(results)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

// findByUserIDWithResults finds all entities for a specific user with pagination and returns them
func (r *baseRepository[T]) findByUserIDWithResults(userID uuid.UUID, limit, offset int) ([]*T, error) {
	var results []*T
	if err := r.findByUserID(userID, limit, offset, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// findByID finds an entity by ID
func (r *baseRepository[T]) findByID(id uuid.UUID, result *T) (*T, error) {
	res := r.db.Where("id = ?", id).First(result)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, res.Error
	}
	return result, nil
}

// countByUserID counts the total number of entities for a user
func (r *baseRepository[T]) countByUserID(userID uuid.UUID, model *T) (int64, error) {
	var count int64
	result := r.db.Model(model).Where("user_id = ?", userID).Count(&count)
	if result.Error != nil {
		return 0, result.Error
	}
	return count, nil
}

// findByUserIDAndField finds an entity by user ID and a specific field value
func (r *baseRepository[T]) findByUserIDAndField(userID uuid.UUID, fieldName string, fieldValue string, result *T) (*T, error) {
	res := r.db.Where("user_id = ? AND "+fieldName+" = ?", userID, fieldValue).Order("created_at DESC").First(result)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, res.Error
	}
	return result, nil
}
