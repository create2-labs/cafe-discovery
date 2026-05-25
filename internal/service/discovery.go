package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/metrics"
	"cafe-discovery/internal/repository"
	"cafe-discovery/internal/walletscan"
	"cafe-discovery/pkg/evm"
	"cafe-discovery/pkg/moralis"

	"github.com/google/uuid"
)

// DiscoveryService orchestrates wallet discovery for the API: plan limits, persistence, and delegation
// to WalletScanEngine for the actual scan.
type DiscoveryService struct {
	engine         *walletscan.WalletScanEngine
	scanResultRepo repository.ScanResultRepository
	planService    *PlanService
}

// NewDiscoveryService creates a new discovery service.
func NewDiscoveryService(clients map[string]*evm.Client, moralisClient *moralis.MoralisClient, scanResultRepo repository.ScanResultRepository, planService *PlanService) *DiscoveryService {
	return &DiscoveryService{
		engine:         walletscan.NewWalletScanEngine(clients, moralisClient),
		scanResultRepo: scanResultRepo,
		planService:    planService,
	}
}

// ScanWallet scans a wallet address across all configured networks and optionally saves the result.
// When skipPersist is true (scanner path), the result is not written to DB; the scanner publishes scan.completed/failed.
func (s *DiscoveryService) ScanWallet(ctx context.Context, userID uuid.UUID, address string, skipPersist bool) (result *domain.ScanResult, err error) {
	startTime := time.Now()
	m := metrics.Get()
	defer func() {
		m.RecordWalletScan(time.Since(startTime), err == nil)
	}()

	normalizedAddress, err := s.engine.ValidateAndNormalizeAddress(address)
	if err != nil {
		return nil, err
	}

	if !skipPersist {
		if err := s.checkPlanLimits(userID); err != nil {
			return nil, err
		}
	}

	result, err = s.engine.Execute(ctx, normalizedAddress)
	if err != nil {
		return nil, err
	}

	if !skipPersist {
		s.saveScanResult(userID, result)
	}

	return result, nil
}

// ValidateAndNormalizeAddress validates and normalizes the Ethereum address.
func (s *DiscoveryService) ValidateAndNormalizeAddress(address string) (string, error) {
	return s.engine.ValidateAndNormalizeAddress(address)
}

// RecoverPublicKeyFromTransactionData recovers the public key from raw transaction JSON (CLI / tooling).
func (s *DiscoveryService) RecoverPublicKeyFromTransactionData(ctx context.Context, client *evm.Client, txData json.RawMessage, txHash string) (string, string, error) {
	return s.engine.RecoverPublicKeyFromTransactionData(ctx, client, txData, txHash)
}

func (s *DiscoveryService) checkPlanLimits(userID uuid.UUID) error {
	if s.planService == nil {
		return nil
	}

	canScan, usage, err := s.planService.CheckScanLimit(userID, "wallet", s.scanResultRepo, nil)
	if err != nil {
		return fmt.Errorf("failed to check plan limits: %w", err)
	}
	if !canScan {
		return fmt.Errorf("wallet scan limit reached (%d/%d). Please upgrade your plan to continue", usage.WalletScansUsed, usage.WalletScanLimit)
	}
	return nil
}

func (s *DiscoveryService) saveScanResult(userID uuid.UUID, result *domain.ScanResult) {
	if s.scanResultRepo == nil {
		return
	}
	scanResultEntity := domain.FromScanResult(userID, result)
	if err := s.scanResultRepo.Create(scanResultEntity); err != nil {
		log.Printf("Failed to save wallet scan result to database (address=%s): %v", result.Address, err)
	}
}

// ListScanResults lists scan results for a user with pagination.
func (s *DiscoveryService) ListScanResults(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.ScanResult, int64, error) {
	entities, err := s.scanResultRepo.FindByUserID(userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch scan results: %w", err)
	}

	total, err := s.scanResultRepo.CountByUserID(userID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count scan results: %w", err)
	}

	results := make([]*domain.ScanResult, len(entities))
	for i, entity := range entities {
		results[i] = entity.ToScanResult()
	}

	return results, total, nil
}
