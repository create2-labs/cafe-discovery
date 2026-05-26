package repository

import (
	"context"
	"fmt"

	redisconn "cafe-discovery/pkg/redis"
)

const walletUserKeyPrefix = "wallet:user:"

// RedisWalletScanRepository supports plan usage counts and DELETE cache cleanup
// for persistence write-through keys wallet:user:<user_id>:<address>.
// Wallet v1 list/get/delete use Postgres as source of truth (IMM-4a).
type RedisWalletScanRepository interface {
	CountByUserID(ctx context.Context, userID string) (int64, error)
	DeleteByUserIDAndAddress(ctx context.Context, userID string, address string) error
}

type redisWalletScanRepository struct {
	redis redisconn.Connection
}

// NewRedisWalletScanRepository creates a new Redis wallet scan repository.
func NewRedisWalletScanRepository(redis redisconn.Connection) RedisWalletScanRepository {
	return &redisWalletScanRepository{redis: redis}
}

func (r *redisWalletScanRepository) userKey(userID, address string) string {
	return walletUserKeyPrefix + userID + ":" + address
}

// DeleteByUserIDAndAddress removes the persistence write-through key for user+address.
func (r *redisWalletScanRepository) DeleteByUserIDAndAddress(ctx context.Context, userID, address string) error {
	if err := r.redis.Del(ctx, r.userKey(userID, address)).Err(); err != nil {
		return fmt.Errorf("redis del wallet user scan: %w", err)
	}
	return nil
}

// CountByUserID returns the number of wallet:user:* keys for a user (plan limits; IMM-6 will move to Postgres).
func (r *redisWalletScanRepository) CountByUserID(ctx context.Context, userID string) (int64, error) {
	pattern := walletUserKeyPrefix + userID + ":*"
	keys, err := r.redis.Keys(ctx, pattern).Result()
	if err != nil {
		return 0, err
	}
	return int64(len(keys)), nil
}
