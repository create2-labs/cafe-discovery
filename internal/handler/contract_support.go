package handler

import (
	"cafe-discovery/internal/config"
	"cafe-discovery/internal/persistence/scanread"
	"cafe-discovery/internal/service"
)

// NewDiscoveryHandlerForContractTest wires scan list/detail deps for internal/contract Option A tests.
func NewDiscoveryHandlerForContractTest(scanRead scanread.Store, cfgChain *config.ChainConfig) *DiscoveryHandler {
	return &DiscoveryHandler{
		discoveryService: service.NewDiscoveryService(nil, nil, nil, nil),
		scanRead:         scanRead,
		cfgChain:         cfgChain,
	}
}

// NewDiscoveryHandlerForContractTestFromRepo adapts a ScanResultRepository stub for contract tests.
func NewDiscoveryHandlerForContractTestFromRepo(repo scanResultReader, cfgChain *config.ChainConfig) *DiscoveryHandler {
	return NewDiscoveryHandlerForContractTest(NewRepoScanReadStub(repo, cfgChain), cfgChain)
}
