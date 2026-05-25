package service

import (
	"context"
	"log"

	"cafe-discovery/internal/domain"
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

// ListTLSURLs returns TLS URLs for the user plus default endpoints, with read-through for both user and defaults.
func (s *UserScanCacheService) ListTLSURLs(ctx context.Context, userID uuid.UUID, limit, offset int) ([]string, int64, error) {
	uid := userID.String()

	userURLs, err := s.redisTLSRepo.ListURLsByUserID(ctx, uid)
	if err != nil {
		userURLs = nil
	}
	if len(userURLs) == 0 {
		entities, loadErr := s.tlsScanResultRepo.FindByUserID(userID, maxItemsForWarmOrReadThrough, 0)
		if loadErr != nil {
			return nil, 0, loadErr
		}
		userURLs = make([]string, 0, len(entities))
		for _, e := range entities {
			dto := e.ToTLSScanResult()
			_ = s.redisTLSRepo.SaveByUserIDAndURL(ctx, uid, e.URL, dto)
			userURLs = append(userURLs, e.URL)
		}
	}

	defaultURLs, err := s.redisTLSRepo.ListURLsByUserID(ctx, repository.DefaultUserIDForRedis)
	if err != nil {
		defaultURLs = nil
	}
	if len(defaultURLs) == 0 {
		defaultEntities, loadErr := s.tlsScanResultRepo.FindAllDefault()
		if loadErr != nil {
			// continue without defaults
		} else {
			defaultURLs = make([]string, 0, len(defaultEntities))
			for _, e := range defaultEntities {
				dto := e.ToTLSScanResult()
				_ = s.redisTLSRepo.SaveByUserIDAndURL(ctx, repository.DefaultUserIDForRedis, e.URL, dto)
				defaultURLs = append(defaultURLs, e.URL)
			}
		}
	}

	seen := make(map[string]struct{}, len(userURLs))
	for _, u := range userURLs {
		seen[u] = struct{}{}
	}
	merged := make([]string, len(userURLs), len(userURLs)+len(defaultURLs))
	copy(merged, userURLs)
	for _, u := range defaultURLs {
		if _, ok := seen[u]; !ok {
			seen[u] = struct{}{}
			merged = append(merged, u)
		}
	}
	total := int64(len(merged))
	return paginateStrings(merged, limit, offset), total, nil
}

// GetTLSScan returns a TLS scan by URL (user then default) with read-through.
func (s *UserScanCacheService) GetTLSScan(ctx context.Context, userID uuid.UUID, url string) (*domain.TLSScanResult, error) {
	uid := userID.String()
	res, err := s.redisTLSRepo.FindByUserIDAndURL(ctx, uid, url)
	if err == nil && res != nil {
		return res, nil
	}
	res, err = s.redisTLSRepo.FindByUserIDAndURL(ctx, repository.DefaultUserIDForRedis, url)
	if err == nil && res != nil {
		return res, nil
	}
	entity, err := s.tlsScanResultRepo.FindByUserIDAndURL(userID, url)
	if err == nil && entity != nil {
		dto := entity.ToTLSScanResult()
		_ = s.redisTLSRepo.SaveByUserIDAndURL(ctx, uid, url, dto)
		return dto, nil
	}
	entity, err = s.tlsScanResultRepo.FindDefaultByURL(url)
	if err != nil || entity == nil {
		return nil, err
	}
	dto := entity.ToTLSScanResult()
	_ = s.redisTLSRepo.SaveByUserIDAndURL(ctx, repository.DefaultUserIDForRedis, url, dto)
	return dto, nil
}

func paginateStrings(slice []string, limit, offset int) []string {
	if offset >= len(slice) {
		return nil
	}
	slice = slice[offset:]
	if limit > 0 && len(slice) > limit {
		slice = slice[:limit]
	}
	return slice
}
