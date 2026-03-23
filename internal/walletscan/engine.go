package walletscan

import (
	"context"
	"fmt"

	"cafe-discovery/internal/domain"
	"cafe-discovery/pkg/evm"
	"cafe-discovery/pkg/moralis"
)

// WalletScanEngine runs wallet discovery across configured chains without persistence or plan checks.
// It is the dependency boundary for NATS scanners and for DiscoveryService.Execute path.
type WalletScanEngine struct {
	clients       map[string]*evm.Client
	moralisClient *moralis.MoralisClient
}

// NewWalletScanEngine builds an engine with RPC and Moralis clients only.
func NewWalletScanEngine(clients map[string]*evm.Client, moralisClient *moralis.MoralisClient) *WalletScanEngine {
	return &WalletScanEngine{
		clients:       clients,
		moralisClient: moralisClient,
	}
}

// ValidateAndNormalizeAddress validates and normalizes an Ethereum address.
func (e *WalletScanEngine) ValidateAndNormalizeAddress(address string) (string, error) {
	normalized := normalizeAddress(address)
	if !isValidAddress(normalized) {
		return "", fmt.Errorf("invalid Ethereum address: %s", address)
	}
	return normalized, nil
}

// Execute scans the wallet and returns a domain result. Metrics are recorded by DiscoveryService.ScanWallet when used from the API.
func (e *WalletScanEngine) Execute(ctx context.Context, address string) (*domain.ScanResult, error) {
	normalizedAddress, err := e.ValidateAndNormalizeAddress(address)
	if err != nil {
		return nil, err
	}

	networkResults, networks, recoveredPublicKey, transactionHash, exposedNetwork := e.scanAllNetworks(ctx, normalizedAddress)
	keyExposed, isERC4337 := e.extractKeyExposureInfo(networkResults)
	accountType, algorithm, nistLevel := e.determineAccountType(networkResults)
	riskScore := e.calculateRiskScore(networkResults, accountType, nistLevel)

	isEOA := true
	for _, r := range networkResults {
		if !r.IsEOA {
			isEOA = false
			break
		}
	}

	scanData := scanResultData{
		accountType:     accountType,
		algorithm:       algorithm,
		nistLevel:       nistLevel,
		keyExposed:      keyExposed,
		isEOA:           isEOA,
		isERC4337:       isERC4337,
		publicKey:       recoveredPublicKey,
		transactionHash: transactionHash,
		exposedNetwork:  exposedNetwork,
		riskScore:       riskScore,
	}
	return e.buildScanResult(normalizedAddress, scanData, networks), nil
}
