package scanhttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/persistence/scanread"

	"github.com/google/uuid"
)

// Config wires the cafe-persistence internal scan HTTP client (openapi/internal/scan/v1).
type Config struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// Client implements scanread.Store over HTTP.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

var _ scanread.Store = (*Client)(nil)

// NewClient returns a scanread.Store backed by cafe-persistence internal/scan/v1.
func NewClient(cfg Config) *Client {
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{
		baseURL:    strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		token:      strings.TrimSpace(cfg.Token),
		httpClient: hc,
	}
}

func (c *Client) ListWalletScans(ctx context.Context, userID uuid.UUID, tenantID string, query url.Values) ([]*domain.ScanResultEntity, int64, int, int, error) {
	u := c.baseURL + V1Base + WalletScans
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var wire scanread.ListWalletScansWire
	if err := c.doJSON(ctx, http.MethodGet, u, userID, tenantID, true, &wire); err != nil {
		return nil, 0, 0, 0, err
	}
	out := make([]*domain.ScanResultEntity, 0, len(wire.Items))
	for _, row := range wire.Items {
		ent, err := scanread.WalletRowToEntity(row)
		if err != nil {
			return nil, 0, 0, 0, scanread.ErrUnavailable
		}
		out = append(out, ent)
	}
	return out, wire.Total, wire.Limit, wire.Offset, nil
}

func (c *Client) GetWalletScan(ctx context.Context, userID uuid.UUID, tenantID string, scanID uuid.UUID) (*domain.ScanResultEntity, error) {
	rel := strings.ReplaceAll(WalletScanByID, "{scan_id}", scanID.String())
	u := c.baseURL + V1Base + rel
	var wire scanread.WalletScanRowWire
	if err := c.doJSON(ctx, http.MethodGet, u, userID, tenantID, true, &wire); err != nil {
		if errors.Is(err, errNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return scanread.WalletRowToEntity(wire)
}

func (c *Client) ListTLSScans(ctx context.Context, userID uuid.UUID, tenantID string, limit, offset int) ([]*domain.TLSScanResultEntity, int64, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	u := c.baseURL + V1Base + TLSScans
	if enc := q.Encode(); enc != "" {
		u += "?" + enc
	}
	var wire scanread.ListTLSScansWire
	if err := c.doJSON(ctx, http.MethodGet, u, userID, tenantID, true, &wire); err != nil {
		return nil, 0, err
	}
	out := make([]*domain.TLSScanResultEntity, 0, len(wire.Items))
	for _, row := range wire.Items {
		ent, err := scanread.TLSRowToEntity(row)
		if err != nil {
			return nil, 0, scanread.ErrUnavailable
		}
		out = append(out, ent)
	}
	return out, wire.Total, nil
}

func (c *Client) ListTLSDefaultScans(ctx context.Context, userID uuid.UUID, tenantID string) ([]*domain.TLSScanResultEntity, error) {
	u := c.baseURL + V1Base + TLSScansDefaults
	var wire scanread.ListTLSDefaultsWire
	if err := c.doJSON(ctx, http.MethodGet, u, userID, tenantID, true, &wire); err != nil {
		return nil, err
	}
	out := make([]*domain.TLSScanResultEntity, 0, len(wire.Items))
	for _, row := range wire.Items {
		ent, err := scanread.TLSRowToEntity(row)
		if err != nil {
			return nil, scanread.ErrUnavailable
		}
		out = append(out, ent)
	}
	return out, nil
}

func (c *Client) GetTLSScan(ctx context.Context, userID uuid.UUID, tenantID string, scanID uuid.UUID) (*domain.TLSScanResultEntity, error) {
	rel := strings.ReplaceAll(TLSScanByID, "{scan_id}", scanID.String())
	u := c.baseURL + V1Base + rel
	var wire scanread.TLSScanRowWire
	if err := c.doJSON(ctx, http.MethodGet, u, userID, tenantID, true, &wire); err != nil {
		if errors.Is(err, errNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return scanread.TLSRowToEntity(wire)
}

func (c *Client) DeleteWalletScan(ctx context.Context, userID uuid.UUID, tenantID string, scanID uuid.UUID) (bool, error) {
	rel := strings.ReplaceAll(WalletScanByID, "{scan_id}", scanID.String())
	u := c.baseURL + V1Base + rel
	if err := c.doJSON(ctx, http.MethodDelete, u, userID, tenantID, true, nil); err != nil {
		if errors.Is(err, errNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *Client) DeleteTLSScan(ctx context.Context, userID uuid.UUID, tenantID string, scanID uuid.UUID) (bool, error) {
	rel := strings.ReplaceAll(TLSScanByID, "{scan_id}", scanID.String())
	u := c.baseURL + V1Base + rel
	if err := c.doJSON(ctx, http.MethodDelete, u, userID, tenantID, true, nil); err != nil {
		if errors.Is(err, errNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

var errNotFound = errors.New("scan not found")

func (c *Client) doJSON(ctx context.Context, method, requestURL string, userID uuid.UUID, tenantID string, idempotent bool, dst any) error {
	var lastErr error
	attempts := 1
	if idempotent {
		attempts = 2
	}
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			time.Sleep(50 * time.Millisecond)
		}
		lastErr = c.doJSONOnce(ctx, method, requestURL, userID, tenantID, dst)
		if lastErr == nil {
			return nil
		}
		if !idempotent || !errors.Is(lastErr, scanread.ErrUnavailable) {
			return lastErr
		}
	}
	return lastErr
}

func (c *Client) doJSONOnce(ctx context.Context, method, requestURL string, userID uuid.UUID, tenantID string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, method, requestURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set(headerAuthorization, "Bearer "+c.token)
	req.Header.Set(headerUserID, userID.String())
	if tenant := strings.TrimSpace(tenantID); tenant != "" {
		req.Header.Set(headerTenantID, tenant)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return scanread.ErrUnavailable
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return scanread.ErrUnavailable
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if dst == nil || len(raw) == 0 {
			return nil
		}
		if err := json.Unmarshal(raw, dst); err != nil {
			return scanread.ErrUnavailable
		}
		return nil
	}
	return mapHTTPError(resp.StatusCode, raw)
}

func mapHTTPError(status int, raw []byte) error {
	_ = raw
	if status == http.StatusNotFound {
		return errNotFound
	}
	if status == http.StatusServiceUnavailable || status >= 500 {
		return scanread.ErrUnavailable
	}
	if status == http.StatusUnauthorized || status == http.StatusBadRequest {
		return scanread.ErrUnavailable
	}
	return fmt.Errorf("scan persistence request failed with status %d", status)
}
