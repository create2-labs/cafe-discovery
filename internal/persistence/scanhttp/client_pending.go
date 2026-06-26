package scanhttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cafe-discovery/internal/persistence/scanpending"
	"cafe-discovery/internal/persistence/scanread"

	"github.com/google/uuid"
)

var _ scanpending.Store = (*Client)(nil)

var errConflict = errors.New("scan reservation conflict")

func pendingUnavailable(err error) error {
	if errors.Is(err, scanread.ErrUnavailable) {
		return scanpending.ErrUnavailable
	}
	return err
}

func (c *Client) PutTLS(ctx context.Context, userID uuid.UUID, tenantID string, rec *scanpending.Record) error {
	if rec == nil {
		return fmt.Errorf("pending scan record is required")
	}
	body := scanpending.PutTLSPendingRequestWire{
		ScanID:   rec.ScanID.String(),
		Endpoint: strings.TrimSpace(rec.Endpoint),
	}
	if !rec.CreatedAt.IsZero() {
		body.CreatedAt = rec.CreatedAt.UTC().Format(time.RFC3339Nano)
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	u := c.baseURL + V1Base + PendingTLS
	var wire scanpending.PendingScanRecordWire
	if err := c.doJSON(ctx, http.MethodPost, u, userID, tenantID, false, payload, &wire); err != nil {
		return pendingUnavailable(err)
	}
	return nil
}

func (c *Client) ReserveWallet(ctx context.Context, userID uuid.UUID, tenantID string, rec *scanpending.Record) (bool, error) {
	if rec == nil {
		return false, fmt.Errorf("pending scan record is required")
	}
	body := scanpending.ReserveWalletPendingRequestWire{
		ScanID:  rec.ScanID.String(),
		Address: strings.TrimSpace(rec.Address),
	}
	if !rec.CreatedAt.IsZero() {
		body.CreatedAt = rec.CreatedAt.UTC().Format(time.RFC3339Nano)
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return false, err
	}
	u := c.baseURL + V1Base + PendingWallet
	var wire scanpending.ReserveWalletPendingResponseWire
	if err := c.doJSON(ctx, http.MethodPost, u, userID, tenantID, false, payload, &wire); err != nil {
		if errors.Is(err, errConflict) {
			return false, nil
		}
		return false, pendingUnavailable(err)
	}
	return wire.Reserved, nil
}

func (c *Client) Get(ctx context.Context, userID uuid.UUID, tenantID string, scanID uuid.UUID) (*scanpending.Record, error) {
	rel := strings.ReplaceAll(PendingByScanID, "{scan_id}", scanID.String())
	u := c.baseURL + V1Base + rel
	var wire scanpending.PendingScanRecordWire
	if err := c.doJSON(ctx, http.MethodGet, u, userID, tenantID, true, nil, &wire); err != nil {
		if errors.Is(err, errNotFound) {
			return nil, nil
		}
		return nil, pendingUnavailable(err)
	}
	return scanpending.RecordFromWire(wire)
}

func (c *Client) GetWalletByOwnerAddress(ctx context.Context, userID uuid.UUID, tenantID string, address string) (*scanpending.Record, error) {
	q := url.Values{}
	q.Set("address", strings.TrimSpace(address))
	u := c.baseURL + V1Base + PendingWallet + "?" + q.Encode()
	var wire scanpending.PendingScanRecordWire
	if err := c.doJSON(ctx, http.MethodGet, u, userID, tenantID, true, nil, &wire); err != nil {
		if errors.Is(err, errNotFound) {
			return nil, nil
		}
		return nil, pendingUnavailable(err)
	}
	return scanpending.RecordFromWire(wire)
}

func (c *Client) Delete(ctx context.Context, userID uuid.UUID, tenantID string, scanID uuid.UUID) error {
	rel := strings.ReplaceAll(PendingByScanID, "{scan_id}", scanID.String())
	u := c.baseURL + V1Base + rel
	if err := c.doJSON(ctx, http.MethodDelete, u, userID, tenantID, false, nil, nil); err != nil {
		return pendingUnavailable(err)
	}
	return nil
}

func (c *Client) DeleteWalletReservation(ctx context.Context, userID uuid.UUID, tenantID string, address string, scanID uuid.UUID) error {
	q := url.Values{}
	q.Set("address", strings.TrimSpace(address))
	q.Set("scan_id", scanID.String())
	u := c.baseURL + V1Base + PendingWallet + "?" + q.Encode()
	if err := c.doJSON(ctx, http.MethodDelete, u, userID, tenantID, false, nil, nil); err != nil {
		return pendingUnavailable(err)
	}
	return nil
}
