package service

import (
	"context"
	"fmt"
	"log"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"
	"cafe-discovery/internal/tlsscan"

	"github.com/google/uuid"
)

// TLSService is the API-facing TLS service (plan checks, persistence, list/get)
// and delegates scan execution to TLSScanEngine.
type TLSService struct {
	engine            *tlsscan.TLSScanEngine
	tlsScanResultRepo repository.TLSScanResultRepository
	planService       *PlanService
}

// NewTLSService creates a new TLS service.
func NewTLSService(tlsScanResultRepo repository.TLSScanResultRepository, planService *PlanService) *TLSService {
	return &TLSService{
		engine:            tlsscan.NewTLSScanEngine(),
		tlsScanResultRepo: tlsScanResultRepo,
		planService:       planService,
	}
}

// ScanTLS scans a URL and optionally persists the result.
func (s *TLSService) ScanTLS(ctx context.Context, userID *uuid.UUID, targetURL string, isDefault bool, skipPersist bool) (result *domain.TLSScanResult, err error) {
	if !skipPersist {
		if err := s.checkPlanLimitForScan(userID, isDefault); err != nil {
			return nil, err
		}
	}

	result, err = s.engine.Execute(ctx, userID, targetURL, isDefault)
	if err != nil {
		return nil, err
	}
	if !skipPersist {
		s.persistTLSScanResult(userID, result, targetURL, isDefault)
	}
	return result, nil
}

// checkPlanLimitForScan returns an error if the user has reached their endpoint scan limit.
func (s *TLSService) checkPlanLimitForScan(userID *uuid.UUID, isDefault bool) error {
	if isDefault || userID == nil || s.planService == nil {
		return nil
	}
	canScan, usage, err := s.planService.CheckScanLimit(*userID, "endpoint", nil, s.tlsScanResultRepo)
	if err != nil {
		return fmt.Errorf("failed to check plan limits: %w", err)
	}
	if !canScan {
		return fmt.Errorf("endpoint scan limit reached (%d/%d). Please upgrade your plan to continue", usage.EndpointScansUsed, usage.EndpointScanLimit)
	}
	return nil
}

func (s *TLSService) persistTLSScanResult(userID *uuid.UUID, result *domain.TLSScanResult, targetURL string, isDefault bool) {
	log.Printf("persistTLSScanResult(userID=%v, result=%v, targetURL=%s, isDefault=%v)", userID, result, targetURL, isDefault)
	if s.tlsScanResultRepo == nil {
		return
	}
	if !isDefault && (userID == nil || *userID == uuid.Nil) {
		log.Printf("TLS scan result not persisted: userID is nil or Nil (url=%s). Sign in and use a valid token so the backend sends user_id in the scan request.", targetURL)
		return
	}
	tlsScanResultEntity := domain.FromTLSScanResult(userID, result, isDefault)
	if err := s.tlsScanResultRepo.Create(tlsScanResultEntity); err != nil {
		log.Printf("Failed to save TLS scan result to database (url=%s): %v", targetURL, err)
	}
}

// GetTLSScanByURL retrieves a TLS scan result by URL for a specific user.
func (s *TLSService) GetTLSScanByURL(ctx context.Context, userID uuid.UUID, url string) (*domain.TLSScanResult, error) {
	entity, err := s.tlsScanResultRepo.FindByUserIDAndURL(userID, url)
	if err == nil && entity != nil {
		return entity.ToTLSScanResult(), nil
	}

	entity, err = s.tlsScanResultRepo.FindDefaultByURL(url)
	if err != nil {
		return nil, fmt.Errorf("TLS scan result not found for URL: %w", err)
	}

	return entity.ToTLSScanResult(), nil
}

// ListTLSScanResults lists TLS scan results for a user with pagination.
func (s *TLSService) ListTLSScanResults(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.TLSScanResult, int64, error) {
	entities, err := s.tlsScanResultRepo.FindByUserIDOrDefault(userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch TLS scan results: %w", err)
	}

	total, err := s.tlsScanResultRepo.CountByUserIDOrDefault(userID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count TLS scan results: %w", err)
	}

	results := make([]*domain.TLSScanResult, len(entities))
	for i, entity := range entities {
		results[i] = entity.ToTLSScanResult()
	}

	return results, total, nil
}
