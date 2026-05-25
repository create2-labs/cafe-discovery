package handler

import (
	"cafe-discovery/internal/config"
	"cafe-discovery/internal/repository"
	"cafe-discovery/internal/service"
)

// NewDiscoveryHandlerForContractTest wires scan list/detail deps for internal/contract Option A tests.
func NewDiscoveryHandlerForContractTest(scanResultRepo repository.ScanResultRepository, cfgChain *config.ChainConfig) *DiscoveryHandler {
	return &DiscoveryHandler{
		discoveryService: service.NewDiscoveryService(nil, nil, nil, nil),
		scanResultRepo:   scanResultRepo,
		cfgChain:         cfgChain,
	}
}
