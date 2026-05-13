package cpmpolicyref

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cafe-discovery/internal/policyref"

	"github.com/google/uuid"
)

// HTTPClient calls CPM internal policy reference check (Bearer service token).
type HTTPClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

var _ policyref.Checker = (*HTTPClient)(nil)

// NewHTTPClient returns a client for POST {baseURL}/internal/policies/references/scan.
// baseURL must be the CPM service root (e.g. http://cafe-cpm:8080) without trailing slash issues.
func NewHTTPClient(baseURL, bearerToken string, httpClient *http.Client) *HTTPClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	return &HTTPClient{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:      strings.TrimSpace(bearerToken),
		httpClient: httpClient,
	}
}

type requestWire struct {
	ScanID   string `json:"scan_id"`
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
}

type responseWire struct {
	Referenced bool `json:"referenced"`
}

// PersistedPoliciesReferenceScan implements policyref.Checker.
func (c *HTTPClient) PersistedPoliciesReferenceScan(ctx context.Context, userID uuid.UUID, tenantID string, scanID uuid.UUID) (bool, error) {
	if c.baseURL == "" || c.token == "" {
		return false, fmt.Errorf("cpm policy reference client: base URL or token not configured")
	}
	u := c.baseURL + "/internal/policies/references/scan"
	body := requestWire{
		ScanID:   strings.ToLower(scanID.String()),
		UserID:   userID.String(),
		TenantID: strings.TrimSpace(tenantID),
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(raw))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	closeErr := resp.Body.Close()
	if readErr != nil {
		return false, errors.Join(readErr, closeErr)
	}
	if closeErr != nil {
		return false, fmt.Errorf("cpm policy reference: close response body: %w", closeErr)
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("cpm policy reference: unexpected status %d", resp.StatusCode)
	}
	var rw responseWire
	if err := json.Unmarshal(respBody, &rw); err != nil {
		return false, fmt.Errorf("cpm policy reference: invalid response json: %w", err)
	}
	return rw.Referenced, nil
}
