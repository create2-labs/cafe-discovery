package service

import (
	"context"
	"fmt"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"

	"github.com/google/uuid"
)

// TLSService is the API-facing TLS service for TLS result list/get.
type TLSService struct {
	tlsScanResultRepo repository.TLSScanResultRepository
}

// NewTLSService creates a new TLS service.
func NewTLSService(tlsScanResultRepo repository.TLSScanResultRepository, planService *PlanService) *TLSService {
	return &TLSService{
		tlsScanResultRepo: tlsScanResultRepo,
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
