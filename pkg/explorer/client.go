package explorer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ExplorerClient represents the HTTP client for the explorer API
type ExplorerClient struct {
	apiKey     string
	chainID    int
	apiURL     string
	httpClient *http.Client
}

// NewExplorerClient creates a new ExplorerClient
func NewExplorerClient(apiKey, apiURL string, chainID int) *ExplorerClient {
	return &ExplorerClient{
		apiKey:  apiKey,
		apiURL:  apiURL,
		chainID: chainID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetTransactionsByAddress gets the transactions for an address
func (c *ExplorerClient) GetTransactionsByAddress(address string) ([]EtherscanTx, error) {
	url := fmt.Sprintf("%s/v2/api?chainid=%d&module=account&action=txlist&address=%s&startblock=0&endblock=99999999&sort=asc&apikey=%s", c.apiURL, c.chainID, address, c.apiKey)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log but don't fail on close errors
			_ = closeErr
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var etherscanResp EtherscanResp
	if err := json.Unmarshal(body, &etherscanResp); err != nil {
		return nil, err
	}
	if etherscanResp.Status != "1" {
		return nil, fmt.Errorf("failed to get transactions: %s", etherscanResp.Message)
	}
	return etherscanResp.Result, nil
}
