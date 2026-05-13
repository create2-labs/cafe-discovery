package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	appredis "cafe-discovery/pkg/redis"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

const pendingV1ScanTTL = 48 * time.Hour

// PendingV1ScanRecord is a scan accepted on POST /discovery/v1/scan before persistence rows exist.
type PendingV1ScanRecord struct {
	ScanID    uuid.UUID `json:"scan_id"`
	UserID    uuid.UUID `json:"user_id"`
	Family    string    `json:"family"` // wallet | tls
	Address   string    `json:"address,omitempty"`
	Endpoint  string    `json:"endpoint,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// PendingV1ScanRepository stores short-lived correlation for requested-only scans (Redis).
type PendingV1ScanRepository interface {
	Put(ctx context.Context, rec *PendingV1ScanRecord) error
	Get(ctx context.Context, scanID uuid.UUID) (*PendingV1ScanRecord, error)
}

type redisPendingV1ScanRepository struct {
	redis appredis.Connection
}

func pendingV1RedisKey(scanID uuid.UUID) string {
	return "discovery:v1:pending_scan:" + scanID.String()
}

// NewRedisPendingV1ScanRepository builds a Redis-backed pending scan store.
// redis must be non-nil (container wiring must not call this with a nil connection).
func NewRedisPendingV1ScanRepository(redis appredis.Connection) (PendingV1ScanRepository, error) {
	if redis == nil {
		return nil, fmt.Errorf("redis connection is required for pending v1 scan repository")
	}
	return &redisPendingV1ScanRepository{redis: redis}, nil
}

func (r *redisPendingV1ScanRepository) Put(ctx context.Context, rec *PendingV1ScanRecord) error {
	if rec == nil {
		return fmt.Errorf("pending v1 scan record is required")
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return r.redis.Set(ctx, pendingV1RedisKey(rec.ScanID), string(b), pendingV1ScanTTL).Err()
}

func (r *redisPendingV1ScanRepository) Get(ctx context.Context, scanID uuid.UUID) (*PendingV1ScanRecord, error) {
	s, err := r.redis.Get(ctx, pendingV1RedisKey(scanID)).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("redis get pending v1 scan: %w", err)
	}
	if s == "" {
		return nil, nil
	}
	var rec PendingV1ScanRecord
	if err := json.Unmarshal([]byte(s), &rec); err != nil {
		return nil, fmt.Errorf("decode pending v1 scan: %w", err)
	}
	return &rec, nil
}
