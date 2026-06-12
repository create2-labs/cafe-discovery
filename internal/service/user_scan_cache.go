package service

import (
	"context"
	"log"

	"cafe-discovery/internal/repository"

	"github.com/google/uuid"
)

const maxItemsForWarmOrReadThrough = 10000

// UserScanCacheService provides TLS read-through from Postgres to Redis and warm cache on sign-in.
type UserScanCacheService struct {
	tlsScanResultRepo repository.TLSScanResultRepository
	redisTLSRepo      repository.RedisTLSScanRepository
}

// NewUserScanCacheService creates a UserScanCacheService.
func NewUserScanCacheService(
	tlsScanResultRepo repository.TLSScanResultRepository,
	redisTLSRepo repository.RedisTLSScanRepository,
) *UserScanCacheService {
	return &UserScanCacheService{
		tlsScanResultRepo: tlsScanResultRepo,
		redisTLSRepo:      redisTLSRepo,
	}
}

// WarmForUser loads TLS scan results for the user from Postgres into Redis (e.g. after sign-in).
// Wallet scans are intentionally not warmed by address: wallet history is keyed by scan_id in Postgres.
func (s *UserScanCacheService) WarmForUser(ctx context.Context, userID uuid.UUID) error {
	uid := userID.String()

	// Warm TLS scans (user's own only; defaults are warmed separately at startup or on first list read-through).
	tlsEntities, err := s.tlsScanResultRepo.FindByUserID(userID, maxItemsForWarmOrReadThrough, 0)
	if err != nil {
		log.Printf("user_scan_cache: warm TLS FindByUserID: %v", err)
		return err
	}
	for _, e := range tlsEntities {
		dto := e.ToTLSScanResult()
		if err := s.redisTLSRepo.SaveByUserIDAndURL(ctx, uid, e.URL, dto); err != nil {
			log.Printf("user_scan_cache: warm TLS %s: %v", e.URL, err)
		}
	}

	return nil
}
