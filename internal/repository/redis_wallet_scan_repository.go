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
	walletRedisKeyPrefix   = "wallet:scan:token:"
	walletUserKeyPrefix    = "wallet:user:"
	walletScanTTL          = 30 * time.Minute
)

// RedisWalletScanRepository stores wallet scan results in Redis by token (temporary, TTL).
// User-scoped methods (FindByUserIDAndAddress, ListAddressesByUserID) read keys written by persistence-service (wallet:user:<id>:<address>).
type RedisWalletScanRepository interface {
	Save(ctx context.Context, tokenHash string, address string, result *domain.ScanResult) error
	FindByAddress(ctx context.Context, tokenHash string, address string) (*domain.ScanResult, error)
	ListAll(ctx context.Context, tokenHash string) ([]*domain.ScanResult, error)
	Count(ctx context.Context, tokenHash string) (int, error)
	Delete(ctx context.Context, tokenHash string, address string) error
	// User-scoped (persistence-service write-through keys)
	SaveByUserIDAndAddress(ctx context.Context, userID string, address string, result *domain.ScanResult) error
	FindByUserIDAndAddress(ctx context.Context, userID string, address string) (*domain.ScanResult, error)
	ListAddressesByUserID(ctx context.Context, userID string) ([]string, error)
	CountByUserID(ctx context.Context, userID string) (int64, error)
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

	if err := r.redis.Set(ctx, key, data, walletScanTTL).Err(); err != nil {
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

// ListAll lists all wallet scan results for a specific token
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

// Count counts the number of wallet scans for a specific token
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

func (r *redisWalletScanRepository) getUserKey(userID string, address string) string {
	return walletUserKeyPrefix + userID + ":" + address
}

// SaveByUserIDAndAddress writes a wallet scan result for user+address (read-through / warm cache). Same key format as persistence.
func (r *redisWalletScanRepository) SaveByUserIDAndAddress(ctx context.Context, userID string, address string, result *domain.ScanResult) error {
	key := r.getUserKey(userID, address)
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal scan result: %w", err)
	}
	if err := r.redis.Set(ctx, key, data, walletScanTTL).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}
	return nil
}

// FindByUserIDAndAddress finds a wallet scan result by user ID and address (persistence-service keys)
func (r *redisWalletScanRepository) FindByUserIDAndAddress(ctx context.Context, userID string, address string) (*domain.ScanResult, error) {
	key := r.getUserKey(userID, address)
	data, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	var result domain.ScanResult
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal scan result: %w", err)
	}
	return &result, nil
}

// ListAddressesByUserID lists all wallet scan addresses for a user (persistence-service keys)
func (r *redisWalletScanRepository) ListAddressesByUserID(ctx context.Context, userID string) ([]string, error) {
	pattern := walletUserKeyPrefix + userID + ":*"
	keys, err := r.redis.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, err
	}
	prefix := walletUserKeyPrefix + userID + ":"
	addresses := make([]string, 0, len(keys))
	for _, k := range keys {
		if len(k) > len(prefix) {
			addresses = append(addresses, k[len(prefix):])
		}
	}
	return addresses, nil
}

// CountByUserID returns the number of wallet scans for a user (for plan limits; backend Redis-only).
func (r *redisWalletScanRepository) CountByUserID(ctx context.Context, userID string) (int64, error) {
	addresses, err := r.ListAddressesByUserID(ctx, userID)
	if err != nil {
		return 0, err
	}
	return int64(len(addresses)), nil
}
