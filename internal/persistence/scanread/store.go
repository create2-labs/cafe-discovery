// Package scanread defines the Discovery scan read contract backed by cafe-persistence internal/scan/v1 (PERS-D6a-read).
package scanread

import (
	"context"
	"errors"
	"net/url"

	"cafe-discovery/internal/domain"

	"github.com/google/uuid"
)

// ErrUnavailable is returned when cafe-persistence cannot serve a read (network, 5xx, 503).
var ErrUnavailable = errors.New("scan persistence unavailable")

// Store provides owner-scoped scan reads for public GET/list v1 handlers.
// Writes, deletes, pending, authz, and quotas remain on direct repositories until later D6a milestones.
type Store interface {
	ListWalletScans(ctx context.Context, userID uuid.UUID, tenantID string, query url.Values) (items []*domain.ScanResultEntity, total int64, limit, offset int, err error)
	GetWalletScan(ctx context.Context, userID uuid.UUID, tenantID string, scanID uuid.UUID) (*domain.ScanResultEntity, error)

	ListTLSScans(ctx context.Context, userID uuid.UUID, tenantID string, limit, offset int) (items []*domain.TLSScanResultEntity, total int64, err error)
	ListTLSDefaultScans(ctx context.Context, userID uuid.UUID, tenantID string) ([]*domain.TLSScanResultEntity, error)
	GetTLSScan(ctx context.Context, userID uuid.UUID, tenantID string, scanID uuid.UUID) (*domain.TLSScanResultEntity, error)
}
