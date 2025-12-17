package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strings"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"
	"cafe-discovery/pkg/evm"
	"cafe-discovery/pkg/moralis"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/google/uuid"
)

// DiscoveryService handles wallet discovery and vulnerability scanning
type DiscoveryService struct {
	clients        map[string]*evm.Client
	moralisClient  *moralis.MoralisClient
	scanResultRepo repository.ScanResultRepository
	planService    *PlanService
}

// NewDiscoveryService creates a new discovery service
func NewDiscoveryService(clients map[string]*evm.Client, moralisClient *moralis.MoralisClient, scanResultRepo repository.ScanResultRepository, planService *PlanService) *DiscoveryService {
	return &DiscoveryService{
		clients:        clients,
		moralisClient:  moralisClient,
		scanResultRepo: scanResultRepo,
		planService:    planService,
	}
}

// ScanWallet scans a wallet address across all configured networks and saves the result for the user
func (s *DiscoveryService) ScanWallet(ctx context.Context, userID uuid.UUID, address string) (*domain.ScanResult, error) {
	// Normalize address (ensure 0x prefix)
	address = normalizeAddress(address)

	if !isValidAddress(address) {
		return nil, fmt.Errorf("invalid Ethereum address: %s", address)
	}

	// Check if scan already exists in database
	existingEntity, err := s.scanResultRepo.FindByUserIDAndAddress(userID, address)
	if err == nil && existingEntity != nil {
		// Return existing scan result
		return existingEntity.ToScanResult(), nil
	}

	// Check plan limits
	if s.planService != nil {
		canScan, usage, err := s.planService.CheckScanLimit(userID, "wallet", s.scanResultRepo, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to check plan limits: %w", err)
		}
		if !canScan {
			return nil, fmt.Errorf("wallet scan limit reached (%d/%d). Please upgrade your plan to continue", usage.WalletScansUsed, usage.WalletScanLimit)
		}
	}

	var networks []string
	var networkResults []domain.NetworkResult
	var recoveredPublicKey string // Store the first recovered public key
	//now := time.Now()

	for networkName, client := range s.clients {
		result, err := s.scanNetwork(ctx, client, networkName, address)
		if err != nil {
			// Log error but continue with other networks
			continue
		}

		networkResults = append(networkResults, *result)

		// Use the first recovered public key we find
		if recoveredPublicKey == "" && result.PublicKey != "" {
			recoveredPublicKey = result.PublicKey
		}

		if result.IsKeyExposed {
			networks = append(networks, networkName)
		}
	}

	// Determine account type and algorithm
	accountType, algorithm, nistLevel := s.determineAccountType(networkResults)

	// Check if key is exposed on any network
	keyExposed := false
	isERC4337 := false
	for _, nr := range networkResults {
		if nr.IsKeyExposed {
			keyExposed = true
		}
		if nr.IsERC4337 {
			isERC4337 = true
		}
	}

	// Calculate risk score
	riskScore := s.calculateRiskScore(networkResults, accountType, nistLevel)

	result := &domain.ScanResult{
		Address:    address,
		Type:       accountType,
		Algorithm:  algorithm,
		NISTLevel:  nistLevel,
		KeyExposed: keyExposed,
		PublicKey:  recoveredPublicKey, // Add recovered public key if available
		IsERC4337:  isERC4337,
		RiskScore:  riskScore,
		//	FirstSeen:   &now, // In production, would query blockchain for actual first transaction date
		//	LastSeen:    &now, // In production, would query blockchain for actual last transaction date
		Networks:    networks,
		Connections: []string{}, // Would be populated from transaction analysis to show connected addresses
	}

	// Save scan result to database
	scanResultEntity := domain.FromScanResult(userID, result)
	if err := s.scanResultRepo.Create(scanResultEntity); err != nil {
		// Log error but don't fail the request - scan was successful
		// In production, you might want to use a logger here
		_ = err
	}

	return result, nil
}

// ListScanResults lists scan results for a user with pagination
func (s *DiscoveryService) ListScanResults(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.ScanResult, int64, error) {
	// Get scan results from repository
	entities, err := s.scanResultRepo.FindByUserID(userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch scan results: %w", err)
	}

	// Get total count for pagination
	total, err := s.scanResultRepo.CountByUserID(userID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count scan results: %w", err)
	}

	// Convert entities to domain ScanResult DTOs
	results := make([]*domain.ScanResult, len(entities))
	for i, entity := range entities {
		results[i] = entity.ToScanResult()
	}

	return results, total, nil
}

// scanNetwork scans a single network for the given address
func (s *DiscoveryService) scanNetwork(ctx context.Context, client *evm.Client, networkName string, address string) (*domain.NetworkResult, error) {
	var publicKey string

	// First, try to get transaction from Moralis if chain name is available
	if s.moralisClient != nil && client.MoralisChainName != "" {
		moralisTxs, err := s.moralisClient.GetTransactionsByAddress(address, client.MoralisChainName)
		if err == nil && len(moralisTxs) > 0 {
			// Get the first transaction hash
			txHash := moralisTxs[0].Hash
			if txHash != "" {
				log.Println("txHash", txHash, "address", address, "networkName", networkName)
				// Retrieve the transaction from RPC
				txData, err := client.GetTransactionByHash(ctx, txHash)
				if err == nil && txData != nil {
					log.Println("txData", string(txData))
					// Try to recover public key from transaction
					recoveredKey, _, err := s.RecoverPublicKeyFromTransactionData(ctx, client, txData, txHash)
					if err == nil && recoveredKey != "" {
						publicKey = recoveredKey
					}
				}
			}
		}
	}

	// Check if address has code (smart contract)
	// eth_getCode returns "0x" for EOA and "0x<bytecode>" for contracts
	code, err := client.GetCode(ctx, address, "latest")
	if err != nil {
		return nil, fmt.Errorf("failed to get code: %w", err)
	}

	// EOA has no code: eth_getCode returns "0x" for EOA
	// Contract has bytecode: eth_getCode returns "0x" + hex bytecode (length > 2)
	isEOA := isEOAAddress(code)

	// Check if contract implements ERC-4337 (only if it's a contract, not EOA)
	var isERC4337 bool
	if !isEOA {
		isERC4337, err = client.CheckERC4337Support(ctx, address)
		if err != nil {
			// Log error but don't fail - assume not ERC-4337 if check fails
			isERC4337 = false
		}
	}

	// Check transaction count to determine if key is exposed
	txCount, err := client.GetTransactionCount(ctx, address, "latest")
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction count: %w", err)
	}

	// Key is exposed if address has sent at least one transaction
	isKeyExposed := txCount > 0

	return &domain.NetworkResult{
		Network:          networkName,
		IsEOA:            isEOA,
		IsERC4337:        isERC4337,
		IsKeyExposed:     isKeyExposed,
		TransactionCount: txCount,
		PublicKey:        publicKey,
	}, nil
}

// determineAccountType determines the account type, algorithm, and NIST level
func (s *DiscoveryService) determineAccountType(results []domain.NetworkResult) (domain.AccountType, domain.Algorithm, domain.NISTLevel) {
	// Check if all results show EOA
	allEOA := true
	for _, result := range results {
		if !result.IsEOA {
			allEOA = false
			break
		}
	}

	if allEOA {
		// EOA uses ECDSA-secp256k1, which is quantum-broken (NIST Level 1)
		return domain.AccountTypeEOA, domain.AlgorithmECDSAsecp256k1, domain.NISTLevel1
	}

	// If any network shows it's a contract, check if it's ERC-4337 compliant
	// ERC-4337 (Account Abstraction) is more flexible for PQC migration
	isERC4337 := false
	for _, result := range results {
		if result.IsERC4337 {
			isERC4337 = true
			break
		}
	}

	if isERC4337 {
		// ERC-4337 Account Abstraction contract
		// While currently using ECDSA, AA contracts can be upgraded to PQC algorithms
		return domain.AccountTypeAA, domain.AlgorithmECDSAsecp256k1, domain.NISTLevel1
	}

	// Regular smart contract (not ERC-4337)
	// For now, we classify non-ERC-4337 contracts as AA too
	// In the future, we might want a separate "Contract" type
	return domain.AccountTypeContract, domain.AlgorithmECDSAsecp256k1, domain.NISTLevel1
}

// calculateRiskScore calculates the risk score based on network results
// Returns a score between 0.0 and 1.0, where higher means higher risk
func (s *DiscoveryService) calculateRiskScore(results []domain.NetworkResult, accountType domain.AccountType, nistLevel domain.NISTLevel) float64 {
	if len(results) == 0 {
		return 0.0
	}

	// Base risk from NIST level (Level 1 = quantum-broken = high risk)
	// NIST Level 1 (ECDSA-secp256k1) contributes 0.5 base risk
	baseRisk := 0.0
	if nistLevel == domain.NISTLevel1 {
		baseRisk = 0.5 // High base risk for quantum-broken algorithms
	}

	// Count networks where key is exposed
	exposedNetworks := 0
	totalTransactions := uint64(0)

	for _, result := range results {
		if result.IsKeyExposed {
			exposedNetworks++
			totalTransactions += result.TransactionCount
		}
	}

	// If no exposure, risk is minimal (unless it's a contract, which is different)
	if exposedNetworks == 0 {
		return baseRisk * 0.3 // Lower risk if not exposed yet
	}

	// Exposure risk: each exposed network adds 0.1-0.2 risk
	exposureRisk := float64(exposedNetworks) * 0.15
	if exposureRisk > 0.4 {
		exposureRisk = 0.4 // Cap at 0.4 for network exposure
	}

	// Transaction count risk: more transactions = more exposure
	transactionRisk := 0.0
	if totalTransactions > 0 {
		// Logarithmic scale: 1-10 tx = 0.05, 10-100 = 0.15, 100+ = 0.25
		if totalTransactions < 10 {
			transactionRisk = 0.05
		} else if totalTransactions < 100 {
			transactionRisk = 0.15
		} else {
			transactionRisk = 0.25
		}
	}

	riskScore := baseRisk + exposureRisk + transactionRisk

	// Clamp between 0.0 and 1.0
	if riskScore > 1.0 {
		riskScore = 1.0
	}
	if riskScore < 0.0 {
		riskScore = 0.0
	}

	return riskScore
}

// determineRiskCategory determines the risk category based on score and NIST level
// Note: This function is not currently used but kept for future use
func (s *DiscoveryService) determineRiskCategory(riskScore float64, nistLevel domain.NISTLevel) domain.RiskCategory {
	if nistLevel >= domain.NISTLevel5 {
		return domain.RiskPQCReady
	}

	if riskScore >= 0.7 {
		return domain.RiskHigh
	}

	if riskScore >= 0.4 {
		return domain.RiskMedium
	}

	return domain.RiskHigh // Conservative: low score but exposed is still high risk
}

// isEOAAddress determines if the code result indicates an EOA (Externally Owned Account)
// eth_getCode returns "0x" for EOA and "0x<bytecode>" for contracts
func isEOAAddress(code string) bool {
	// Empty string or just "0x" means no code (EOA)
	// Any string longer than "0x" (2 chars) contains bytecode (contract)
	return code == "" || code == "0x" || len(code) <= 2
}

// normalizeAddress ensures the address has the 0x prefix
func normalizeAddress(address string) string {
	address = strings.TrimSpace(address)
	if !strings.HasPrefix(address, "0x") {
		address = "0x" + address
	}
	return strings.ToLower(address)
}

// isValidAddress performs basic address validation
func isValidAddress(address string) bool {
	if len(address) != 42 || !strings.HasPrefix(address, "0x") {
		return false
	}

	// Check if remaining characters are valid hex
	for _, c := range address[2:] {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}

	return true
}

// recoverPublicKeyFromTransactionData attempts to recover the public key from a transaction
func (s *DiscoveryService) RecoverPublicKeyFromTransactionData(ctx context.Context, client *evm.Client, txData json.RawMessage, txHash string) (string, string, error) {
	// Parse transaction data
	var txJSON map[string]interface{}
	if err := json.Unmarshal(txData, &txJSON); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal transaction: %w", err)
	}

	// Extract transaction fields needed for recovery
	nonceHex, _ := txJSON["nonce"].(string)
	gasHex, _ := txJSON["gas"].(string)
	toAddr, _ := txJSON["to"].(string)
	valueHex, _ := txJSON["value"].(string)
	inputData, _ := txJSON["input"].(string)
	rHex, _ := txJSON["r"].(string)
	sHex, _ := txJSON["s"].(string)
	chainIDHex, _ := txJSON["chainId"].(string)

	// Check if it's an EIP-1559 transaction (London fork)
	maxFeePerGasHex, hasMaxFeePerGas := txJSON["maxFeePerGas"].(string)
	maxPriorityFeePerGasHex, hasMaxPriorityFeePerGas := txJSON["maxPriorityFeePerGas"].(string)
	isEIP1559 := hasMaxFeePerGas && hasMaxPriorityFeePerGas && maxFeePerGasHex != "" && maxPriorityFeePerGasHex != ""

	// Get chain ID
	var chainID *big.Int
	if chainIDHex != "" && chainIDHex != "0x" {
		chainID = new(big.Int)
		chainID.SetString(chainIDHex, 0)
	}

	// Build transaction for recovery
	nonce := hexToBigInt(nonceHex)
	gas := hexToBigInt(gasHex)
	value := hexToBigInt(valueHex)
	r := hexToBigInt(rHex)
	sigS := hexToBigInt(sHex)

	// Calculate v based on transaction type
	var v *big.Int
	if isEIP1559 {
		// For EIP-1559, use yParity instead of v
		// yParity is 0 or 1, and v = chainID * 2 + 35 + yParity
		yParityHex, hasYParity := txJSON["yParity"].(string)
		if !hasYParity {
			// Fallback: try to get yParity from v if it's 0 or 1
			vHex, _ := txJSON["v"].(string)
			vVal := hexToBigInt(vHex)
			if vVal.Uint64() == 0 || vVal.Uint64() == 1 {
				yParityHex = vHex
			} else {
				return "", "", fmt.Errorf("yParity not found for EIP-1559 transaction")
			}
		}
		yParity := hexToBigInt(yParityHex)
		// v = chainID * 2 + 35 + yParity
		v = new(big.Int).Mul(chainID, big.NewInt(2))
		v.Add(v, big.NewInt(35))
		v.Add(v, yParity)
	} else {
		// For legacy transactions, use v directly
		vHex, _ := txJSON["v"].(string)
		v = hexToBigInt(vHex)
	}

	// Decode input data
	var data []byte
	if inputData != "" && inputData != "0x" {
		var err error
		data, err = hex.DecodeString(strings.TrimPrefix(inputData, "0x"))
		if err != nil {
			return "", "", fmt.Errorf("failed to decode input data: %w", err)
		}
	}

	// Create transaction
	var to *common.Address
	if toAddr != "" && toAddr != "0x" && toAddr != "null" {
		addr := common.HexToAddress(toAddr)
		to = &addr
	}

	var tx *types.Transaction

	// Create transaction based on type
	if isEIP1559 {
		// EIP-1559 transaction (London fork)
		maxFeePerGas := hexToBigInt(maxFeePerGasHex)
		maxPriorityFeePerGas := hexToBigInt(maxPriorityFeePerGasHex)
		tx = types.NewTx(&types.DynamicFeeTx{
			ChainID:   chainID,
			Nonce:     nonce.Uint64(),
			To:        to,
			Value:     value,
			Gas:       gas.Uint64(),
			GasFeeCap: maxFeePerGas,
			GasTipCap: maxPriorityFeePerGas,
			Data:      data,
			V:         v,
			R:         r,
			S:         sigS,
		})
	} else {
		// Legacy transaction (EIP-155 or before)
		gasPriceHex, _ := txJSON["gasPrice"].(string)
		gasPrice := hexToBigInt(gasPriceHex)
		tx = types.NewTx(&types.LegacyTx{
			Nonce:    nonce.Uint64(),
			To:       to,
			Value:    value,
			Gas:      gas.Uint64(),
			GasPrice: gasPrice,
			Data:     data,
			V:        v,
			R:        r,
			S:        sigS,
		})
	}

	// Recover public key
	// For EIP-1559, pass yParity directly if available
	var yParity *big.Int
	if isEIP1559 {
		yParityHex, hasYParity := txJSON["yParity"].(string)
		if hasYParity && yParityHex != "" {
			yParity = hexToBigInt(yParityHex)
		}
	}

	pubKeyHex, recoveredTxHash, err := evm.RecoverPubKeyFromTx(tx, types.NewLondonSigner(chainID), chainID, yParity)
	if err != nil {
		return "", "", fmt.Errorf("failed to recover public key: %w", err)
	}

	return pubKeyHex, recoveredTxHash, nil
}

// hexToBigInt converts a hex string to *big.Int
func hexToBigInt(hexStr string) *big.Int {
	if hexStr == "" || hexStr == "0x" {
		return big.NewInt(0)
	}
	result := new(big.Int)
	result.SetString(hexStr, 0)
	return result
}
