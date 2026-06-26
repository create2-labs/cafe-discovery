package scanpending

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// PendingScanRecordWire mirrors cafe-persistence internal scan v1 PendingScanRecord JSON.
type PendingScanRecordWire struct {
	ScanID    string `json:"scan_id"`
	UserID    string `json:"user_id"`
	Family    string `json:"family"`
	Address   string `json:"address,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`
	CreatedAt string `json:"created_at"`
}

// ReserveWalletPendingRequestWire is the POST /pending/wallet body.
type ReserveWalletPendingRequestWire struct {
	ScanID    string `json:"scan_id"`
	Address   string `json:"address"`
	CreatedAt string `json:"created_at,omitempty"`
}

// PutTLSPendingRequestWire is the POST /pending/tls body.
type PutTLSPendingRequestWire struct {
	ScanID    string `json:"scan_id"`
	Endpoint  string `json:"endpoint"`
	CreatedAt string `json:"created_at,omitempty"`
}

// ReserveWalletPendingResponseWire is the POST /pending/wallet 201 body.
type ReserveWalletPendingResponseWire struct {
	Reserved bool                  `json:"reserved"`
	Record   PendingScanRecordWire `json:"record"`
}

// RecordFromWire maps a persistence pending record to a Discovery record.
func RecordFromWire(w PendingScanRecordWire) (*Record, error) {
	scanID, err := uuid.Parse(strings.TrimSpace(w.ScanID))
	if err != nil {
		return nil, err
	}
	userID, err := uuid.Parse(strings.TrimSpace(w.UserID))
	if err != nil {
		return nil, err
	}
	createdAt, err := parseTime(w.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &Record{
		ScanID:    scanID,
		UserID:    userID,
		Family:    w.Family,
		Address:   w.Address,
		Endpoint:  w.Endpoint,
		CreatedAt: createdAt,
	}, nil
}

func parseTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t.UTC(), nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}
