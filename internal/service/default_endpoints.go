package service

import (
	"context"
	"log"
	"time"

	"cafe-discovery/internal/repository"
	"cafe-discovery/pkg/nats"

	"github.com/google/uuid"
)

// DefaultEndpoint represents a default endpoint to scan at startup
type DefaultEndpoint struct {
	URL     string
	Default bool
	Comment string
}

// DefaultEndpoints is the list of endpoints to scan at application startup
// if they are not already in Redis (written by persistence after scan.requested.tls).
var DefaultEndpoints = []DefaultEndpoint{
	{URL: "https://nginx", Default: true, Comment: "Cafe Discovery"},
	{URL: "https://test.openquantumsafe.org", Default: true, Comment: "OpenQuantum Safe Test"},
	{URL: "https://test.openquantumsafe.org:6001", Default: true, Comment: "OpenQuantum Safe Test - Hybrid SecP256r1MLKEM768"},
	{URL: "https://test.openquantumsafe.org:6002", Default: true, Comment: "OpenQuantum Safe Test - Hybrid SecP384r1MLKEM1024"},
	{URL: "https://test.openquantumsafe.org:6003", Default: true, Comment: "OpenQuantum Safe Test - Hybrid X25519MLKEM768"},
	{URL: "https://pq.cloudflareresearch.com", Default: true, Comment: "Cloudflare PQC Test"},
	{URL: "https://rpc.ankr.com/eth", Default: true, Comment: "Ethereum Mainnet"},
	{URL: "https://arb1.arbitrum.io/rpc", Default: true, Comment: "Arbitrum"},
	{URL: "https://mainnet.optimism.io", Default: true, Comment: "Optimism"},
	{URL: "https://mainnet.base.org", Default: true, Comment: "Coinbase L2"},
	{URL: "https://zkevm-rpc.com", Default: true, Comment: "Polygon zkEVM"},
	{URL: "https://rpc.linea.build", Default: true, Comment: "Linea (Consensys zkEVM)"},
	{URL: "https://rpc.scroll.io", Default: true, Comment: "Scroll"},
	{URL: "https://rpc.mantlenetwork.io", Default: true, Comment: "Mantle"},
	{URL: "https://rpc.taiko.xyz", Default: true, Comment: "Taiko"},
	{URL: "https://rpc.blast.io", Default: true, Comment: "Blast"},
	{URL: "https://mainnet.mode.network", Default: true, Comment: "Mode"},
	{URL: "https://polygon-rpc.com", Default: true, Comment: "Polygon"},
	{URL: "https://lineascan.build", Default: true, Comment: "Linea Explorer (Mainnet)"},
	{URL: "https://goerli.lineascan.build", Default: true, Comment: "Linea Explorer (Goerli)"},
}

const defaultUserIDForRedis = "00000000-0000-0000-0000-000000000000" // uuid.Nil for default endpoints in Redis keys

// InitializeDefaultEndpointsSync runs after persistence and scanners are ready:
// 1) For each default endpoint URL, skip if already in Redis; otherwise publish scan.requested.tls (IsDefault=true).
// 2) Block until all requested URLs have a result in Redis (or timeout).
// Call this only after WaitForPersistence and WaitForScanners have returned.
func InitializeDefaultEndpointsSync(ctx context.Context, natsConn nats.Connection, redisTLSRepo repository.RedisTLSScanRepository) {
	log.Printf("🔍 Initializing default endpoints (%d endpoints)...", len(DefaultEndpoints))

	var toRequest []string
	for _, ep := range DefaultEndpoints {
		_, err := redisTLSRepo.FindByUserIDAndURL(ctx, defaultUserIDForRedis, ep.URL)
		if err == nil {
			log.Printf("  ⏭️  %s: already in Redis (skipping)", ep.URL)
			continue
		}
		toRequest = append(toRequest, ep.URL)
	}

	if len(toRequest) == 0 {
		log.Printf("✅ All default endpoints already in Redis")
		return
	}

	// Publish scan requests (scanners + persistence must be up)
	for _, url := range toRequest {
		msg := nats.TLSScanMessage{
			ScanID:    uuid.New(),
			UserID:    uuid.Nil,
			Endpoint:  url,
			IsDefault: true,
		}
		if err := nats.PublishJSON(natsConn, nats.SubjectScanRequestedTLS, msg); err != nil {
			log.Printf("  ⚠️  %s: failed to publish scan.requested.tls: %v", url, err)
			continue
		}
		log.Printf("  📡 Requested scan for %s", url)
		time.Sleep(300 * time.Millisecond) // avoid flooding
	}

	// Wait until each requested URL appears in Redis (persistence write-through after scan.completed/failed)
	deadline := time.Now().Add(defaultEndpointsPollTimeout)
	ticker := time.NewTicker(defaultEndpointsPollInterval)
	defer ticker.Stop()
	remaining := make(map[string]struct{})
	for _, u := range toRequest {
		remaining[u] = struct{}{}
	}

	for len(remaining) > 0 && time.Now().Before(deadline) {
		<-ticker.C
		for url := range remaining {
			_, err := redisTLSRepo.FindByUserIDAndURL(ctx, defaultUserIDForRedis, url)
			if err == nil {
				delete(remaining, url)
				log.Printf("  ✅ %s: now in Redis", url)
			}
		}
	}
	if len(remaining) > 0 {
		log.Printf("⚠️  %d default endpoint(s) still not in Redis after timeout: %v", len(remaining), remaining)
	} else {
		log.Printf("✅ Default endpoints initialization completed (all in Redis)")
	}
}
