package moralis

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// MoralisClient represents the HTTP client for the Moralis API
type MoralisClient struct {
	apiKey     string
	apiURL     string
	httpClient *http.Client
}

// NewMoralisClient creates a new MoralisClient
func NewMoralisClient(apiKey, apiURL string) *MoralisClient {
	return &MoralisClient{
		apiKey: apiKey,
		apiURL: apiURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetTransactionsByAddress gets the transactions for an address
func (c *MoralisClient) GetTransactionsByAddress(address string, chainName string) ([]MoralisTxResult, error) {
	url := fmt.Sprintf("%s/api/v2.2/wallets/%s/history?chain=%s&order=DESC&limit=2", c.apiURL, address, chainName)
	log.Println("url", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
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
	var moralisTxResponse MoralisTxResponse
	if err := json.Unmarshal(body, &moralisTxResponse); err != nil {
		return nil, err
	}
	return moralisTxResponse.Result, nil
}
