package cphttp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cafe-discovery/internal/policyref"

	"github.com/google/uuid"
)

// Config wires the cafe-persistence internal CP HTTP client (openapi/internal/cp/v1).
type Config struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// Client implements policyref.Checker over GET /references/* (existence only, ADR §9.3).
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

var _ policyref.Checker = (*Client)(nil)

// NewClient returns a persistence CP reference client backed by internal/cp/v1.
func NewClient(cfg Config) *Client {
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 5 * time.Second}
	}
	return &Client{
		baseURL:    strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		token:      strings.TrimSpace(cfg.Token),
		httpClient: hc,
	}
}

type scanReferenceWire struct {
	Referenced bool `json:"referenced"`
	Count      int  `json:"count"`
}

type walletReferenceWire struct {
	Exists       bool `json:"exists"`
	PolicyCount  int  `json:"policy_count"`
	DraftCount   int  `json:"draft_count"`
}

// PersistedPoliciesReferenceScan implements policyref.Checker (W3).
func (c *Client) PersistedPoliciesReferenceScan(ctx context.Context, userID uuid.UUID, tenantID string, scanID uuid.UUID) (bool, error) {
	if c.baseURL == "" || c.token == "" {
		return false, fmt.Errorf("cp persistence reference client: base URL or token not configured")
	}
	q := url.Values{}
	q.Set("scan_id", strings.ToLower(scanID.String()))
	u := c.baseURL + V1Base + ReferenceScan + "?" + q.Encode()
	var wire scanReferenceWire
	if err := c.doJSON(ctx, http.MethodGet, u, userID, tenantID, &wire); err != nil {
		return false, err
	}
	return wire.Referenced, nil
}

// ActiveWalletCPMContextForTarget implements policyref.Checker (W1).
func (c *Client) ActiveWalletCPMContextForTarget(ctx context.Context, userID uuid.UUID, tenantID string, normalizedTargetAddress string) (policyref.WalletTargetContext, error) {
	if c.baseURL == "" || c.token == "" {
		return policyref.WalletTargetContext{}, fmt.Errorf("cp persistence reference client: base URL or token not configured")
	}
	q := url.Values{}
	q.Set("wallet_address", strings.TrimSpace(normalizedTargetAddress))
	u := c.baseURL + V1Base + ReferenceWallet + "?" + q.Encode()
	var wire walletReferenceWire
	if err := c.doJSON(ctx, http.MethodGet, u, userID, tenantID, &wire); err != nil {
		return policyref.WalletTargetContext{}, err
	}
	return policyref.WalletTargetContext{
		Exists:      wire.Exists,
		PolicyCount: wire.PolicyCount,
		DraftCount:  wire.DraftCount,
	}, nil
}

func (c *Client) doJSON(ctx context.Context, method, requestURL string, userID uuid.UUID, tenantID string, dst any) error {
	return c.doJSONOnce(ctx, method, requestURL, userID, tenantID, dst)
}

func (c *Client) doJSONOnce(ctx context.Context, method, requestURL string, userID uuid.UUID, tenantID string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, method, requestURL, http.NoBody)
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
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cp persistence reference: unexpected status %d", resp.StatusCode)
	}
	if dst == nil {
		return nil
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("cp persistence reference: invalid response json: %w", err)
	}
	return nil
}
