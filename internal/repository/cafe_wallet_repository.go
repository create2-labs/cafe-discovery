package repository

import (
	"cafe-discovery/internal/domain"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CafeWalletRepository defines the interface for cafe wallet data access operations
type CafeWalletRepository interface {
	Create(wallet *domain.CafeWallet) error
	FindByID(pubKeyHash string, userID uuid.UUID) (*domain.CafeWallet, error)
	FindAllByUserID(userID uuid.UUID) ([]*domain.CafeWallet, error)
	Update(wallet *domain.CafeWallet) error
	Delete(pubKeyHash string, userID uuid.UUID) error
}

// cafeWalletRepository implements CafeWalletRepository interface
type cafeWalletRepository struct {
	db *gorm.DB
}

// NewCafeWalletRepository creates a new cafe wallet repository
func NewCafeWalletRepository(db *gorm.DB) CafeWalletRepository {
	return &cafeWalletRepository{
		db: db,
	}
}

// Create creates a new cafe wallet in the database
func (r *cafeWalletRepository) Create(wallet *domain.CafeWallet) error {
	if err := r.db.Create(wallet).Error; err != nil {
		return err
	}
	return nil
}

// FindByID finds a cafe wallet by pub key hash and user ID
func (r *cafeWalletRepository) FindByID(pubKeyHash string, userID uuid.UUID) (*domain.CafeWallet, error) {
	var wallet domain.CafeWallet
	result := r.db.Where("pub_key_hash = ? AND user_id = ?", pubKeyHash, userID).First(&wallet)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &wallet, nil
}

// FindAllByUserID finds all cafe wallets for a specific user
func (r *cafeWalletRepository) FindAllByUserID(userID uuid.UUID) ([]*domain.CafeWallet, error) {
	var wallets []*domain.CafeWallet
	result := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&wallets)
	if result.Error != nil {
		return nil, result.Error
	}
	return wallets, nil
}

// Update updates an existing cafe wallet
func (r *cafeWalletRepository) Update(wallet *domain.CafeWallet) error {
	result := r.db.Model(wallet).
		Where("pub_key_hash = ? AND user_id = ?", wallet.PubKeyHash, wallet.UserID).
		Updates(map[string]interface{}{
			"user_pub_key": wallet.UserPubKey,
			"label":        wallet.Label,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Delete deletes a cafe wallet
func (r *cafeWalletRepository) Delete(pubKeyHash string, userID uuid.UUID) error {
	result := r.db.Where("pub_key_hash = ? AND user_id = ?", pubKeyHash, userID).Delete(&domain.CafeWallet{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

