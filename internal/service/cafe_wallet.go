package service

import (
	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	ErrWalletNotFound = errors.New("wallet not found")
	ErrWalletExists   = errors.New("wallet already exists")
)

// CafeWalletService handles cafe wallet operations
type CafeWalletService struct {
	walletRepo repository.CafeWalletRepository
}

// NewCafeWalletService creates a new cafe wallet service
func NewCafeWalletService(walletRepo repository.CafeWalletRepository) *CafeWalletService {
	return &CafeWalletService{
		walletRepo: walletRepo,
	}
}

// CreateWalletRequest represents the request to create a wallet
type CreateWalletRequest struct {
	PubKeyHash string `json:"pub_key_hash"`
	UserPubKey string `json:"user_pub_key"`
	Label      string `json:"label"`
}

// UpdateWalletRequest represents the request to update a wallet
type UpdateWalletRequest struct {
	UserPubKey string `json:"user_pub_key"`
	Label      string `json:"label"`
}

// CreateWallet creates a new wallet for a user
func (s *CafeWalletService) CreateWallet(userID uuid.UUID, req CreateWalletRequest) (*domain.CafeWallet, error) {
	// Validate required fields
	if req.PubKeyHash == "" {
		return nil, fmt.Errorf("pub_key_hash is required")
	}
	if req.UserPubKey == "" {
		return nil, fmt.Errorf("user_pub_key is required")
	}

	// Check if wallet already exists for this user
	existing, err := s.walletRepo.FindByID(req.PubKeyHash, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing wallet: %w", err)
	}
	if existing != nil {
		return nil, ErrWalletExists
	}

	// Create wallet
	wallet := &domain.CafeWallet{
		PubKeyHash: req.PubKeyHash,
		UserID:     userID,
		UserPubKey: req.UserPubKey,
		Label:      req.Label,
	}

	if err := s.walletRepo.Create(wallet); err != nil {
		return nil, fmt.Errorf("failed to create wallet: %w", err)
	}

	return wallet, nil
}

// GetWallet retrieves a wallet by pub key hash for a user
func (s *CafeWalletService) GetWallet(userID uuid.UUID, pubKeyHash string) (*domain.CafeWallet, error) {
	wallet, err := s.walletRepo.FindByID(pubKeyHash, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}
	if wallet == nil {
		return nil, ErrWalletNotFound
	}
	return wallet, nil
}

// GetAllWallets retrieves all wallets for a user
func (s *CafeWalletService) GetAllWallets(userID uuid.UUID) ([]*domain.CafeWallet, error) {
	wallets, err := s.walletRepo.FindAllByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallets: %w", err)
	}
	return wallets, nil
}

// UpdateWallet updates an existing wallet
func (s *CafeWalletService) UpdateWallet(userID uuid.UUID, pubKeyHash string, req UpdateWalletRequest) (*domain.CafeWallet, error) {
	// Get existing wallet
	wallet, err := s.walletRepo.FindByID(pubKeyHash, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}
	if wallet == nil {
		return nil, ErrWalletNotFound
	}

	// Update fields
	if req.UserPubKey != "" {
		wallet.UserPubKey = req.UserPubKey
	}
	if req.Label != "" {
		wallet.Label = req.Label
	}

	// Save updates
	if err := s.walletRepo.Update(wallet); err != nil {
		return nil, fmt.Errorf("failed to update wallet: %w", err)
	}

	// Fetch updated wallet
	updatedWallet, err := s.walletRepo.FindByID(pubKeyHash, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated wallet: %w", err)
	}

	return updatedWallet, nil
}

// DeleteWallet deletes a wallet
func (s *CafeWalletService) DeleteWallet(userID uuid.UUID, pubKeyHash string) error {
	// Check if wallet exists
	wallet, err := s.walletRepo.FindByID(pubKeyHash, userID)
	if err != nil {
		return fmt.Errorf("failed to get wallet: %w", err)
	}
	if wallet == nil {
		return ErrWalletNotFound
	}

	if err := s.walletRepo.Delete(pubKeyHash, userID); err != nil {
		return fmt.Errorf("failed to delete wallet: %w", err)
	}

	return nil
}

