package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cafe-discovery/internal/domain"
	redisconn "cafe-discovery/pkg/redis"
)

const (
	// Redis key prefix for anonymous wallet scans
	walletRedisKeyPrefix = "wallet:scan:anonymous:"
	// TTL for anonymous wallet scans (30 minutes)
	anonymousWalletScanTTL = 30 * time.Minute
	// Maximum number of anonymous wallet scans per token
	maxAnonymousWalletScans = 10
)

// RedisWalletScanRepository handles storage of anonymous wallet scan results in Redis
type RedisWalletScanRepository interface {
	Save(ctx context.Context, tokenHash string, address string, result *domain.ScanResult) error
	FindByAddress(ctx context.Context, tokenHash string, address string) (*domain.ScanResult, error)
	ListAll(ctx context.Context, tokenHash string) ([]*domain.ScanResult, error)
	Count(ctx context.Context, tokenHash string) (int, error)
	Delete(ctx context.Context, tokenHash string, address string) error
}

type redisWalletScanRepository struct {
	redis redisconn.Connection
}

// NewRedisWalletScanRepository creates a new Redis wallet scan repository
func NewRedisWalletScanRepository(redis redisconn.Connection) RedisWalletScanRepository {
	return &redisWalletScanRepository{
		redis: redis,
	}
}

// getKey returns the Redis key for a given token hash and address
func (r *redisWalletScanRepository) getKey(tokenHash string, address string) string {
	return walletRedisKeyPrefix + tokenHash + ":" + address
}

// Save saves a wallet scan result in Redis with TTL
func (r *redisWalletScanRepository) Save(ctx context.Context, tokenHash string, address string, result *domain.ScanResult) error {
	key := r.getKey(tokenHash, address)

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal scan result: %w", err)
	}

	if err := r.redis.Set(ctx, key, data, anonymousWalletScanTTL).Err(); err != nil {
		return fmt.Errorf("failed to save scan result to Redis: %w", err)
	}

	return nil
}

// FindByAddress finds a wallet scan result by address for a specific token
func (r *redisWalletScanRepository) FindByAddress(ctx context.Context, tokenHash string, address string) (*domain.ScanResult, error) {
	key := r.getKey(tokenHash, address)

	data, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get scan result from Redis: %w", err)
	}

	var result domain.ScanResult
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal scan result: %w", err)
	}

	return &result, nil
}

// ListAll lists all anonymous wallet scan results for a specific token
func (r *redisWalletScanRepository) ListAll(ctx context.Context, tokenHash string) ([]*domain.ScanResult, error) {
	pattern := walletRedisKeyPrefix + tokenHash + ":*"
	keys, err := r.redis.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to list scan keys from Redis: %w", err)
	}

	var results []*domain.ScanResult
	for _, key := range keys {
		data, err := r.redis.Get(ctx, key).Result()
		if err != nil {
			// Skip keys that no longer exist (expired)
			continue
		}

		var result domain.ScanResult
		if err := json.Unmarshal([]byte(data), &result); err != nil {
			// Skip invalid data
			continue
		}

		results = append(results, &result)
	}

	return results, nil
}

// Count counts the number of anonymous wallet scans for a specific token
func (r *redisWalletScanRepository) Count(ctx context.Context, tokenHash string) (int, error) {
	pattern := walletRedisKeyPrefix + tokenHash + ":*"
	keys, err := r.redis.Keys(ctx, pattern).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to count scan keys from Redis: %w", err)
	}

	// Count only existing keys (not expired)
	count := 0
	for _, key := range keys {
		_, err := r.redis.Get(ctx, key).Result()
		if err != nil {
			// Skip keys that no longer exist (expired)
			continue
		}
		count++
	}

	return count, nil
}

// Delete deletes a wallet scan result from Redis
func (r *redisWalletScanRepository) Delete(ctx context.Context, tokenHash string, address string) error {
	key := r.getKey(tokenHash, address)
	if err := r.redis.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete scan result from Redis: %w", err)
	}
	return nil
}

// GetMaxAnonymousWalletScans returns the maximum number of anonymous wallet scans allowed
func GetMaxAnonymousWalletScans() int {
	return maxAnonymousWalletScans
}
