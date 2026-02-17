package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"cafe-discovery/internal/domain"
	redisconn "cafe-discovery/pkg/redis"
)

const (
	redisKeyPrefix   = "tls:scan:token:"
	tlsUserKeyPrefix = "tls:user:"
	tlsScanTTL       = 30 * time.Minute
)

// RedisTLSScanRepository stores TLS scan results in Redis by token (temporary, TTL).
// User-scoped methods (FindByUserIDAndURL, ListURLsByUserID) read keys written by persistence-service (tls:user:<id>:<url>).
type RedisTLSScanRepository interface {
	Save(ctx context.Context, tokenHash string, url string, result *domain.TLSScanResult) error
	FindByURL(ctx context.Context, tokenHash string, url string) (*domain.TLSScanResult, error)
	ListAll(ctx context.Context, tokenHash string) ([]*domain.TLSScanResult, error)
	Count(ctx context.Context, tokenHash string) (int, error)
	Delete(ctx context.Context, tokenHash string, url string) error
	// User-scoped (persistence-service write-through keys)
	FindByUserIDAndURL(ctx context.Context, userID string, url string) (*domain.TLSScanResult, error)
	ListURLsByUserID(ctx context.Context, userID string) ([]string, error)
	CountByUserID(ctx context.Context, userID string) (int64, error)
}

// HashToken creates a SHA256 hash of the JWT token for use as a unique identifier
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

type redisTLSScanRepository struct {
	redis redisconn.Connection
}

// NewRedisTLSScanRepository creates a new Redis TLS scan repository
func NewRedisTLSScanRepository(redis redisconn.Connection) RedisTLSScanRepository {
	return &redisTLSScanRepository{
		redis: redis,
	}
}

// getKey returns the Redis key for a given token hash and URL
func (r *redisTLSScanRepository) getKey(tokenHash string, url string) string {
	return redisKeyPrefix + tokenHash + ":" + url
}

// Save saves a TLS scan result in Redis with TTL
func (r *redisTLSScanRepository) Save(ctx context.Context, tokenHash string, url string, result *domain.TLSScanResult) error {
	key := r.getKey(tokenHash, url)

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal scan result: %w", err)
	}

	if err := r.redis.Set(ctx, key, data, tlsScanTTL).Err(); err != nil {
		return fmt.Errorf("failed to save scan result to Redis: %w", err)
	}

	return nil
}

// FindByURL finds a TLS scan result by URL for a specific token
func (r *redisTLSScanRepository) FindByURL(ctx context.Context, tokenHash string, url string) (*domain.TLSScanResult, error) {
	key := r.getKey(tokenHash, url)

	data, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get scan result from Redis: %w", err)
	}

	var result domain.TLSScanResult
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal scan result: %w", err)
	}

	return &result, nil
}

// ListAll lists all TLS scan results for a specific token
func (r *redisTLSScanRepository) ListAll(ctx context.Context, tokenHash string) ([]*domain.TLSScanResult, error) {
	pattern := redisKeyPrefix + tokenHash + ":*"
	keys, err := r.redis.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to list scan keys from Redis: %w", err)
	}

	var results []*domain.TLSScanResult
	for _, key := range keys {
		data, err := r.redis.Get(ctx, key).Result()
		if err != nil {
			// Skip keys that no longer exist (expired)
			continue
		}

		var result domain.TLSScanResult
		if err := json.Unmarshal([]byte(data), &result); err != nil {
			// Skip invalid data
			continue
		}

		results = append(results, &result)
	}

	return results, nil
}

// Count counts the number of TLS scan results for a specific token
func (r *redisTLSScanRepository) Count(ctx context.Context, tokenHash string) (int, error) {
	pattern := redisKeyPrefix + tokenHash + ":*"
	keys, err := r.redis.Keys(ctx, pattern).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to count scan keys from Redis: %w", err)
	}
	return len(keys), nil
}

// Delete deletes a TLS scan result from Redis
func (r *redisTLSScanRepository) Delete(ctx context.Context, tokenHash string, url string) error {
	key := r.getKey(tokenHash, url)
	if err := r.redis.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete scan result from Redis: %w", err)
	}
	return nil
}

// getUserKey returns the key used by persistence-service for user-scoped TLS results
func (r *redisTLSScanRepository) getUserKey(userID string, url string) string {
	return tlsUserKeyPrefix + userID + ":" + url
}

// FindByUserIDAndURL finds a TLS scan result by user ID and URL (persistence-service keys)
func (r *redisTLSScanRepository) FindByUserIDAndURL(ctx context.Context, userID string, url string) (*domain.TLSScanResult, error) {
	key := r.getUserKey(userID, url)
	data, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	var result domain.TLSScanResult
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal scan result: %w", err)
	}
	return &result, nil
}

// ListURLsByUserID lists all TLS scan URLs for a user (persistence-service keys)
func (r *redisTLSScanRepository) ListURLsByUserID(ctx context.Context, userID string) ([]string, error) {
	pattern := tlsUserKeyPrefix + userID + ":*"
	keys, err := r.redis.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, err
	}
	prefix := tlsUserKeyPrefix + userID + ":"
	urls := make([]string, 0, len(keys))
	for _, k := range keys {
		if len(k) > len(prefix) {
			urls = append(urls, k[len(prefix):])
		}
	}
	return urls, nil
}

// CountByUserID returns the number of TLS scans for a user (for plan limits; backend Redis-only).
func (r *redisTLSScanRepository) CountByUserID(ctx context.Context, userID string) (int64, error) {
	urls, err := r.ListURLsByUserID(ctx, userID)
	if err != nil {
		return 0, err
	}
	return int64(len(urls)), nil
}
