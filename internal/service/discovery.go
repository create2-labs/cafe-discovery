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

// scanResultData holds the data needed to build a scan result
type scanResultData struct {
	accountType domain.AccountType
	algorithm   domain.Algorithm
	nistLevel   domain.NISTLevel
	keyExposed  bool
	isERC4337   bool
	publicKey   string
	riskScore   float64
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
	normalizedAddress, err := s.validateAndNormalizeAddress(address)
	if err != nil {
		return nil, err
	}

	// For anonymous users (uuid.Nil), skip existing scan check but still check rate limits
	isAnonymous := userID == uuid.Nil
	if !isAnonymous {
		existingScan, err := s.getExistingScan(userID, normalizedAddress)
		if err != nil || existingScan != nil {
			return existingScan, err
		}
	}

	// Check plan limits for both authenticated and anonymous users (anonymous uses rate limiting)
	if err := s.checkPlanLimits(userID); err != nil {
		return nil, err
	}

	networkResults, networks, recoveredPublicKey := s.scanAllNetworks(ctx, normalizedAddress)
	keyExposed, isERC4337 := s.extractKeyExposureInfo(networkResults)
	accountType, algorithm, nistLevel := s.determineAccountType(networkResults)
	riskScore := s.calculateRiskScore(networkResults, accountType, nistLevel)

	scanData := scanResultData{
		accountType: accountType,
		algorithm:   algorithm,
		nistLevel:   nistLevel,
		keyExposed:  keyExposed,
		isERC4337:   isERC4337,
		publicKey:   recoveredPublicKey,
		riskScore:   riskScore,
	}
	result := s.buildScanResult(normalizedAddress, scanData, networks)

	// Save scan result only if not anonymous (userID != uuid.Nil)
	// Anonymous users can scan but results are not saved
	if !isAnonymous {
		s.saveScanResult(userID, result)
	}

	return result, nil
}

// validateAndNormalizeAddress validates and normalizes the Ethereum address
func (s *DiscoveryService) validateAndNormalizeAddress(address string) (string, error) {
	normalized := normalizeAddress(address)
	if !isValidAddress(normalized) {
		return "", fmt.Errorf("invalid Ethereum address: %s", address)
	}
	return normalized, nil
}

// getExistingScan checks if a scan already exists for the user and address
func (s *DiscoveryService) getExistingScan(userID uuid.UUID, address string) (*domain.ScanResult, error) {
	existingEntity, err := s.scanResultRepo.FindByUserIDAndAddress(userID, address)
	if err == nil && existingEntity != nil {
		return existingEntity.ToScanResult(), nil
	}
	return nil, err
}

// checkPlanLimits verifies if the user can perform a scan based on their plan limits
func (s *DiscoveryService) checkPlanLimits(userID uuid.UUID) error {
	if s.planService == nil {
		return nil
	}

	canScan, usage, err := s.planService.CheckScanLimit(userID, "wallet", s.scanResultRepo, nil)
	if err != nil {
		return fmt.Errorf("failed to check plan limits: %w", err)
	}
	if !canScan {
		return fmt.Errorf("wallet scan limit reached (%d/%d). Please upgrade your plan to continue", usage.WalletScansUsed, usage.WalletScanLimit)
	}
	return nil
}

// scanAllNetworks scans the address across all configured networks
func (s *DiscoveryService) scanAllNetworks(ctx context.Context, address string) ([]domain.NetworkResult, []string, string) {
	var networks []string
	var networkResults []domain.NetworkResult
	var recoveredPublicKey string

	for networkName, client := range s.clients {
		result, err := s.scanNetwork(ctx, client, networkName, address)
		if err != nil {
			continue
		}

		networkResults = append(networkResults, *result)

		if recoveredPublicKey == "" && result.PublicKey != "" {
			recoveredPublicKey = result.PublicKey
		}

		if result.IsKeyExposed {
			networks = append(networks, networkName)
		}
	}

	return networkResults, networks, recoveredPublicKey
}

// extractKeyExposureInfo extracts key exposure and ERC4337 information from network results
func (s *DiscoveryService) extractKeyExposureInfo(networkResults []domain.NetworkResult) (bool, bool) {
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

	return keyExposed, isERC4337
}

// buildScanResult constructs a ScanResult from the collected information
func (s *DiscoveryService) buildScanResult(address string, data scanResultData, networks []string) *domain.ScanResult {
	return &domain.ScanResult{
		Address:     address,
		Type:        data.accountType,
		Algorithm:   data.algorithm,
		NISTLevel:   data.nistLevel,
		KeyExposed:  data.keyExposed,
		PublicKey:   data.publicKey,
		IsERC4337:   data.isERC4337,
		RiskScore:   data.riskScore,
		Networks:    networks,
		Connections: []string{},
	}
}

// saveScanResult saves the scan result to the database
func (s *DiscoveryService) saveScanResult(userID uuid.UUID, result *domain.ScanResult) {
	scanResultEntity := domain.FromScanResult(userID, result)
	if err := s.scanResultRepo.Create(scanResultEntity); err != nil {
		// Log error but don't fail the request - scan was successful
		// In production, you might want to use a logger here
		_ = err
	}
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
	publicKey := s.tryRecoverPublicKeyFromMoralis(ctx, client, address, networkName)

	code, err := client.GetCode(ctx, address, "latest")
	if err != nil {
		return nil, fmt.Errorf("failed to get code: %w", err)
	}

	isEOA := isEOAAddress(code)
	isERC4337 := s.checkERC4337Support(ctx, client, address, isEOA)

	txCount, err := client.GetTransactionCount(ctx, address, "latest")
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction count: %w", err)
	}

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

// tryRecoverPublicKeyFromMoralis attempts to recover the public key using Moralis transaction data
func (s *DiscoveryService) tryRecoverPublicKeyFromMoralis(ctx context.Context, client *evm.Client, address, networkName string) string {
	if s.moralisClient == nil || client.MoralisChainName == "" {
		return ""
	}

	moralisTxs, err := s.moralisClient.GetTransactionsByAddress(address, client.MoralisChainName)
	if err != nil || len(moralisTxs) == 0 {
		return ""
	}

	txHash := moralisTxs[0].Hash
	if txHash == "" {
		return ""
	}

	log.Println("txHash", txHash, "address", address, "networkName", networkName)
	txData, err := client.GetTransactionByHash(ctx, txHash)
	if err != nil || txData == nil {
		return ""
	}

	log.Println("txData", string(txData))
	recoveredKey, _, err := s.RecoverPublicKeyFromTransactionData(ctx, client, txData, txHash)
	if err != nil || recoveredKey == "" {
		return ""
	}

	return recoveredKey
}

// checkERC4337Support checks if a contract implements ERC-4337 (only for contracts, not EOAs)
func (s *DiscoveryService) checkERC4337Support(ctx context.Context, client *evm.Client, address string, isEOA bool) bool {
	if isEOA {
		return false
	}

	isERC4337, err := client.CheckERC4337Support(ctx, address)
	if err != nil {
		// Log error but don't fail - assume not ERC-4337 if check fails
		return false
	}

	return isERC4337
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
	_ = accountType // Reserved for future use in risk calculation
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
/*
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
*/

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

// txFields holds extracted transaction fields
type txFields struct {
	nonceHex                string
	gasHex                  string
	toAddr                  string
	valueHex                string
	inputData               string
	rHex                    string
	sHex                    string
	chainIDHex              string
	maxFeePerGasHex         string
	maxPriorityFeePerGasHex string
	gasPriceHex             string
}

// txBuildParams holds parameters for building a transaction
type txBuildParams struct {
	chainID *big.Int
	nonce   *big.Int
	to      *common.Address
	value   *big.Int
	gas     *big.Int
	data    []byte
	v       *big.Int
	r       *big.Int
	s       *big.Int
	fields  txFields
}

// RecoverPublicKeyFromTransactionData attempts to recover the public key from a transaction
func (s *DiscoveryService) RecoverPublicKeyFromTransactionData(ctx context.Context, client *evm.Client, txData json.RawMessage, txHash string) (string, string, error) {
	txJSON, err := s.parseTransactionJSON(txData)
	if err != nil {
		return "", "", err
	}

	fields := s.extractTransactionFields(txJSON)
	isEIP1559 := s.isEIP1559Transaction(txJSON, fields)
	chainID := s.parseChainID(fields.chainIDHex)

	v, err := s.calculateV(txJSON, isEIP1559, chainID)
	if err != nil {
		return "", "", err
	}

	data, err := s.decodeInputData(fields.inputData)
	if err != nil {
		return "", "", err
	}

	to := s.parseToAddress(fields.toAddr)
	tx, err := s.buildTransaction(fields, isEIP1559, chainID, to, data, v)
	if err != nil {
		return "", "", err
	}

	yParity := s.extractYParity(txJSON, isEIP1559)
	pubKeyHex, recoveredTxHash, err := evm.RecoverPubKeyFromTx(tx, types.NewLondonSigner(chainID), chainID, yParity)
	if err != nil {
		return "", "", fmt.Errorf("failed to recover public key: %w", err)
	}

	return pubKeyHex, recoveredTxHash, nil
}

// parseTransactionJSON parses the transaction JSON data
func (s *DiscoveryService) parseTransactionJSON(txData json.RawMessage) (map[string]interface{}, error) {
	var txJSON map[string]interface{}
	if err := json.Unmarshal(txData, &txJSON); err != nil {
		return nil, fmt.Errorf("failed to unmarshal transaction: %w", err)
	}
	return txJSON, nil
}

// extractTransactionFields extracts all transaction fields from JSON
func (s *DiscoveryService) extractTransactionFields(txJSON map[string]interface{}) txFields {
	getString := func(key string) string {
		val, _ := txJSON[key].(string)
		return val
	}

	return txFields{
		nonceHex:                getString("nonce"),
		gasHex:                  getString("gas"),
		toAddr:                  getString("to"),
		valueHex:                getString("value"),
		inputData:               getString("input"),
		rHex:                    getString("r"),
		sHex:                    getString("s"),
		chainIDHex:              getString("chainId"),
		maxFeePerGasHex:         getString("maxFeePerGas"),
		maxPriorityFeePerGasHex: getString("maxPriorityFeePerGas"),
		gasPriceHex:             getString("gasPrice"),
	}
}

// isEIP1559Transaction determines if the transaction is EIP-1559
func (s *DiscoveryService) isEIP1559Transaction(txJSON map[string]interface{}, fields txFields) bool {
	_, hasMaxFeePerGas := txJSON["maxFeePerGas"].(string)
	_, hasMaxPriorityFeePerGas := txJSON["maxPriorityFeePerGas"].(string)
	return hasMaxFeePerGas && hasMaxPriorityFeePerGas && fields.maxFeePerGasHex != "" && fields.maxPriorityFeePerGasHex != ""
}

// parseChainID parses the chain ID from hex string
func (s *DiscoveryService) parseChainID(chainIDHex string) *big.Int {
	if chainIDHex == "" || chainIDHex == "0x" {
		return nil
	}
	chainID := new(big.Int)
	chainID.SetString(chainIDHex, 0)
	return chainID
}

// calculateV calculates the v value based on transaction type
func (s *DiscoveryService) calculateV(txJSON map[string]interface{}, isEIP1559 bool, chainID *big.Int) (*big.Int, error) {
	if isEIP1559 {
		return s.calculateVForEIP1559(txJSON, chainID)
	}
	return s.calculateVForLegacy(txJSON), nil
}

// calculateVForEIP1559 calculates v for EIP-1559 transactions
func (s *DiscoveryService) calculateVForEIP1559(txJSON map[string]interface{}, chainID *big.Int) (*big.Int, error) {
	yParityHex, hasYParity := txJSON["yParity"].(string)
	if !hasYParity {
		// Fallback: try to get yParity from v if it's 0 or 1
		vHex, _ := txJSON["v"].(string)
		vVal := hexToBigInt(vHex)
		if vVal.Uint64() == 0 || vVal.Uint64() == 1 {
			yParityHex = vHex
		} else {
			return nil, fmt.Errorf("yParity not found for EIP-1559 transaction")
		}
	}

	yParity := hexToBigInt(yParityHex)
	// v = chainID * 2 + 35 + yParity
	v := new(big.Int).Mul(chainID, big.NewInt(2))
	v.Add(v, big.NewInt(35))
	v.Add(v, yParity)
	return v, nil
}

// calculateVForLegacy calculates v for legacy transactions
func (s *DiscoveryService) calculateVForLegacy(txJSON map[string]interface{}) *big.Int {
	vHex, _ := txJSON["v"].(string)
	return hexToBigInt(vHex)
}

// decodeInputData decodes the input data from hex string
func (s *DiscoveryService) decodeInputData(inputData string) ([]byte, error) {
	if inputData == "" || inputData == "0x" {
		return nil, nil
	}
	data, err := hex.DecodeString(strings.TrimPrefix(inputData, "0x"))
	if err != nil {
		return nil, fmt.Errorf("failed to decode input data: %w", err)
	}
	return data, nil
}

// parseToAddress parses the 'to' address from string
func (s *DiscoveryService) parseToAddress(toAddr string) *common.Address {
	if toAddr == "" || toAddr == "0x" || toAddr == "null" {
		return nil
	}
	addr := common.HexToAddress(toAddr)
	return &addr
}

// buildTransaction builds a transaction based on type
func (s *DiscoveryService) buildTransaction(fields txFields, isEIP1559 bool, chainID *big.Int, to *common.Address, data []byte, v *big.Int) (*types.Transaction, error) {
	params := txBuildParams{
		chainID: chainID,
		nonce:   hexToBigInt(fields.nonceHex),
		to:      to,
		value:   hexToBigInt(fields.valueHex),
		gas:     hexToBigInt(fields.gasHex),
		data:    data,
		v:       v,
		r:       hexToBigInt(fields.rHex),
		s:       hexToBigInt(fields.sHex),
		fields:  fields,
	}

	if isEIP1559 {
		return s.buildEIP1559Transaction(params), nil
	}
	return s.buildLegacyTransaction(params), nil
}

// buildEIP1559Transaction builds an EIP-1559 transaction
func (s *DiscoveryService) buildEIP1559Transaction(params txBuildParams) *types.Transaction {
	maxFeePerGas := hexToBigInt(params.fields.maxFeePerGasHex)
	maxPriorityFeePerGas := hexToBigInt(params.fields.maxPriorityFeePerGasHex)
	return types.NewTx(&types.DynamicFeeTx{
		ChainID:   params.chainID,
		Nonce:     params.nonce.Uint64(),
		To:        params.to,
		Value:     params.value,
		Gas:       params.gas.Uint64(),
		GasFeeCap: maxFeePerGas,
		GasTipCap: maxPriorityFeePerGas,
		Data:      params.data,
		V:         params.v,
		R:         params.r,
		S:         params.s,
	})
}

// buildLegacyTransaction builds a legacy transaction
func (s *DiscoveryService) buildLegacyTransaction(params txBuildParams) *types.Transaction {
	gasPrice := hexToBigInt(params.fields.gasPriceHex)
	return types.NewTx(&types.LegacyTx{
		Nonce:    params.nonce.Uint64(),
		To:       params.to,
		Value:    params.value,
		Gas:      params.gas.Uint64(),
		GasPrice: gasPrice,
		Data:     params.data,
		V:        params.v,
		R:        params.r,
		S:        params.s,
	})
}

// extractYParity extracts yParity for EIP-1559 transactions
func (s *DiscoveryService) extractYParity(txJSON map[string]interface{}, isEIP1559 bool) *big.Int {
	if !isEIP1559 {
		return nil
	}
	yParityHex, hasYParity := txJSON["yParity"].(string)
	if hasYParity && yParityHex != "" {
		return hexToBigInt(yParityHex)
	}
	return nil
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
