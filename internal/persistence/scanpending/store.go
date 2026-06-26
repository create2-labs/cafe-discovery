// Package scanpending defines the Discovery scan pending contract backed by cafe-persistence internal/scan/v1 (PERS-D6a-pending).
package scanpending

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrUnavailable is returned when cafe-persistence cannot serve a pending operation (network, 5xx, 503).
var ErrUnavailable = errors.New("scan pending persistence unavailable")

// Record is a scan accepted on POST /discovery/v1/scan before persistence rows exist.
type Record struct {
	ScanID    uuid.UUID
	UserID    uuid.UUID
	Family    string // wallet | tls
	Address   string
	Endpoint  string
	CreatedAt time.Time
}

// Store provides owner-scoped pending scan reservations via cafe-persistence internal/scan/v1.
type Store interface {
	PutTLS(ctx context.Context, userID uuid.UUID, tenantID string, rec *Record) error
	ReserveWallet(ctx context.Context, userID uuid.UUID, tenantID string, rec *Record) (reserved bool, err error)
	Get(ctx context.Context, userID uuid.UUID, tenantID string, scanID uuid.UUID) (*Record, error)
	GetWalletByOwnerAddress(ctx context.Context, userID uuid.UUID, tenantID string, address string) (*Record, error)
	Delete(ctx context.Context, userID uuid.UUID, tenantID string, scanID uuid.UUID) error
	DeleteWalletReservation(ctx context.Context, userID uuid.UUID, tenantID string, address string, scanID uuid.UUID) error
}
