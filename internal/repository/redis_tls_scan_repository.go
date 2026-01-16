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
	// Redis key prefix for anonymous TLS scans
	redisKeyPrefix = "tls:scan:anonymous:"
	// TTL for anonymous scans (30 minutes)
	anonymousScanTTL = 30 * time.Minute
)

// RedisTLSScanRepository handles storage of anonymous TLS scan results in Redis
type RedisTLSScanRepository interface {
	Save(ctx context.Context, tokenHash string, url string, result *domain.TLSScanResult) error
	FindByURL(ctx context.Context, tokenHash string, url string) (*domain.TLSScanResult, error)
	ListAll(ctx context.Context, tokenHash string) ([]*domain.TLSScanResult, error)
	Count(ctx context.Context, tokenHash string) (int, error)
	Delete(ctx context.Context, tokenHash string, url string) error
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

	if err := r.redis.Set(ctx, key, data, anonymousScanTTL).Err(); err != nil {
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

// ListAll lists all anonymous TLS scan results for a specific token
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

// Count counts the number of anonymous TLS scan results for a specific token
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
