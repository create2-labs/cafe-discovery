package scanhttp

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type walletTransitionResponse struct {
	Outcome         string `json:"outcome"`
	Status          string `json:"status,omitempty"`
	ProjectionReady bool   `json:"projection_ready"`
}

// ApplyWalletTransition posts one command. One call is one physical request:
// doJSONOnce does not retry, so two calls are two requests, not four.
func (c *Client) ApplyWalletTransition(ctx context.Context, userID, scanID uuid.UUID, body []byte) (string, string, bool, error) {
	rel := strings.ReplaceAll(WalletScanTransition, "{scan_id}", scanID.String())
	var out walletTransitionResponse
	if err := c.doJSONOnce(ctx, http.MethodPost, c.baseURL+V1Base+rel, userID, "", body, &out); err != nil {
		return "", "", false, err
	}
	return out.Outcome, out.Status, out.ProjectionReady, nil
}
