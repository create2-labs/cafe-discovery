package handler

import (
	"cafe-discovery/internal/config"
	"cafe-discovery/internal/repository"
)

// NewDiscoveryHandlerForContractTest wires scan list/detail deps for internal/contract Option A tests.
func NewDiscoveryHandlerForContractTest(scanResultRepo repository.ScanResultRepository, cfgChain *config.ChainConfig) *DiscoveryHandler {
	return &DiscoveryHandler{
		scanResultRepo: scanResultRepo,
		cfgChain:       cfgChain,
	}
}
