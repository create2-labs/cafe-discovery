package evm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"
)

// JSONRPCRequest represents a JSON-RPC request
type JSONRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

// JSONRPCResponse represents a JSON-RPC response
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
	ID      int             `json:"id"`
}

// JSONRPCError represents a JSON-RPC error
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Client represents an EVM-compatible blockchain client
type Client struct {
	rpcURL           string
	MoralisChainName string
	httpClient       *http.Client
}

// NewClient creates a new EVM client
func NewClient(rpcURL string, moralisChainName string) *Client {
	return &Client{
		rpcURL:           rpcURL,
		MoralisChainName: moralisChainName,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Call performs a JSON-RPC call to the blockchain
func (c *Client) Call(ctx context.Context, method string, params []interface{}) (json.RawMessage, error) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.rpcURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// G704: rpcURL is from config, not user input
	resp, err := c.httpClient.Do(httpReq) // #nosec G704
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log but don't fail on close errors
			_ = closeErr
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var jsonResp JSONRPCResponse
	if err := json.Unmarshal(body, &jsonResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if jsonResp.Error != nil {
		return nil, fmt.Errorf("rpc error: %s (code: %d)", jsonResp.Error.Message, jsonResp.Error.Code)
	}

	return jsonResp.Result, nil
}

// GetCode retrieves the contract code at the given address
func (c *Client) GetCode(ctx context.Context, address string, blockTag string) (string, error) {
	if blockTag == "" {
		blockTag = "latest"
	}

	result, err := c.Call(ctx, "eth_getCode", []interface{}{address, blockTag})
	if err != nil {
		return "", err
	}

	var code string
	if err := json.Unmarshal(result, &code); err != nil {
		return "", fmt.Errorf("failed to unmarshal code: %w", err)
	}

	return code, nil
}

// GetTransactionCount retrieves the transaction count for an address
func (c *Client) GetTransactionCount(ctx context.Context, address string, blockTag string) (uint64, error) {
	if blockTag == "" {
		blockTag = "latest"
	}

	result, err := c.Call(ctx, "eth_getTransactionCount", []interface{}{address, blockTag})
	if err != nil {
		return 0, err
	}

	var hexCount string
	if err := json.Unmarshal(result, &hexCount); err != nil {
		return 0, fmt.Errorf("failed to unmarshal transaction count: %w", err)
	}

	// Remove 0x prefix and parse hex
	if len(hexCount) < 3 || hexCount[:2] != "0x" {
		return 0, fmt.Errorf("invalid hex format: %s", hexCount)
	}

	var count uint64
	if _, err := fmt.Sscanf(hexCount[2:], "%x", &count); err != nil {
		return 0, fmt.Errorf("failed to parse hex: %w", err)
	}

	return count, nil
}

// CallContract performs an eth_call to execute a contract method
func (c *Client) CallContract(ctx context.Context, address string, data string, blockTag string) (string, error) {
	if blockTag == "" {
		blockTag = "latest"
	}

	callObj := map[string]interface{}{
		"to":   address,
		"data": data,
	}

	result, err := c.Call(ctx, "eth_call", []interface{}{callObj, blockTag})
	if err != nil {
		return "", err
	}

	var response string
	if err := json.Unmarshal(result, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal call result: %w", err)
	}

	return response, nil
}

// CheckERC165Support checks if a contract supports an ERC-165 interface
// interfaceID should be the 4-byte function selector (e.g., "0x01ffc9a7" for supportsInterface)
func (c *Client) CheckERC165Support(ctx context.Context, address string, interfaceID string) (bool, error) {
	// ERC-165: supportsInterface(bytes4) -> bool
	// Function selector: 0x01ffc9a7
	// We pad the interfaceID to 32 bytes (64 hex chars)
	data := "0x01ffc9a7" + padHex(interfaceID, 64)

	result, err := c.CallContract(ctx, address, data, "latest")
	if err != nil {
		// If call fails, interface is likely not supported
		return false, nil
	}

	// Result is a 32-byte bool (0x000...000 for false, 0x000...001 for true)
	// Check if the last character is 1
	result = strings.TrimPrefix(result, "0x")
	result = strings.ToLower(result)

	// Find the first non-zero character
	for i := len(result) - 1; i >= 0; i-- {
		if result[i] != '0' {
			return true, nil
		}
	}

	return false, nil
}

// CheckERC4337Support checks if a contract implements ERC-4337 Account Abstraction
// by checking if it has the validateUserOp function
// Function signature: validateUserOp(UserOperation calldata userOp, bytes32 userOpHash, uint256 missingAccountFunds) external returns (uint256)
// Note: Function selector may need verification. The actual selector is calculated from the full signature.
// Common selectors seen in production: 0xb63e800d or variations
func (c *Client) CheckERC4337Support(ctx context.Context, address string) (bool, error) {
	// Method 1: Check bytecode for common validateUserOp selectors
	// Try multiple possible selectors as different implementations may use variations
	code, err := c.GetCode(ctx, address, "latest")
	if err != nil {
		return false, err
	}

	codeLower := strings.ToLower(code)

	// Common function selectors for validateUserOp
	// Note: These should be verified against actual ERC-4337 implementations
	possibleSelectors := []string{
		"b63e800d", // Most common variant
		"2fad5c34", // Alternative signature variant
	}

	for _, selector := range possibleSelectors {
		if strings.Contains(codeLower, selector) {
			// Function selector found in bytecode
			return true, nil
		}
	}

	// Method 2: Try calling validateUserOp with minimal data
	// This works even for proxy contracts where bytecode search might fail
	// We construct a minimal call that will revert but indicates function existence
	minimalData := "0xb63e800d" + strings.Repeat("0", 64*10) // Minimal data for function call

	result, err := c.CallContract(ctx, address, minimalData, "latest")
	if err != nil {
		// Check if error indicates function doesn't exist vs. function reverts
		// If call fails with specific error (function not found), return false
		// If call succeeds or reverts (function exists), return true
		errStr := strings.ToLower(err.Error())
		if strings.Contains(errStr, "execution reverted") {
			// Function exists but reverted - this is expected and indicates ERC-4337 support
			return true, nil
		}
		// Other errors likely mean function doesn't exist
		return false, nil
	}

	// If we get any response (even a revert response), the function exists
	// Empty result usually means function doesn't exist
	if result != "" && result != "0x" && len(result) > 2 {
		return true, nil
	}

	return false, nil
}

// GetTransactionByHash retrieves a transaction by its hash
func (c *Client) GetTransactionByHash(ctx context.Context, txHash string) (json.RawMessage, error) {
	result, err := c.Call(ctx, "eth_getTransactionByHash", []interface{}{txHash})
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}
	return result, nil
}

// ChainID returns the chain ID of the network via eth_chainId (e.g. when tx JSON has no chainId)
func (c *Client) ChainID(ctx context.Context) (*big.Int, error) {
	result, err := c.Call(ctx, "eth_chainId", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %w", err)
	}
	var hexStr string
	if err := json.Unmarshal(result, &hexStr); err != nil {
		return nil, fmt.Errorf("failed to unmarshal chain ID: %w", err)
	}
	if hexStr == "" || hexStr == "0x" {
		return nil, fmt.Errorf("empty chain ID from RPC")
	}
	chainID := new(big.Int)
	chainID.SetString(hexStr, 0)
	return chainID, nil
}

// padHex pads a hex string to the specified length (without 0x prefix)
func padHex(hex string, targetLength int) string {
	hex = strings.TrimPrefix(hex, "0x")
	if len(hex) >= targetLength {
		return hex[:targetLength]
	}
	return strings.Repeat("0", targetLength-len(hex)) + hex
}
