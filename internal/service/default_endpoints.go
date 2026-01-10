package service

import (
	"context"
	"log"
	"time"

	"cafe-discovery/internal/repository"
)

// DefaultEndpoint represents a default endpoint to scan at startup
type DefaultEndpoint struct {
	URL     string
	Default bool
	Comment string
}

// DefaultEndpoints is the list of endpoints to scan at application startup
// if they are not already in the database
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

// InitializeDefaultEndpoints scans all default endpoints if they are not already in the database
// This function runs asynchronously and does not block the application startup
// Default endpoints are not associated with any user (userID=nil)
func InitializeDefaultEndpoints(ctx context.Context, tlsService *TLSService, tlsScanResultRepo repository.TLSScanResultRepository) {
	log.Printf("🔍 Initializing default endpoints (%d endpoints)...", len(DefaultEndpoints))

	// Run in a goroutine to not block startup
	go func() {
		for _, ep := range DefaultEndpoints {
			// Check if default endpoint already exists in database (default=true)
			existing, err := tlsScanResultRepo.FindDefaultByURL(ep.URL)
			if err != nil {
				log.Printf("  ⚠️  %s: error checking existing scan - %v", ep.URL, err)
				continue
			}
			if existing != nil {
				log.Printf("  ⏭️  %s: already scanned as default endpoint (skipping)", ep.URL)
				continue
			}

			// Scan the endpoint and save with default=true (userID=nil for default endpoints)
			log.Printf("  📡 Scanning %s (%s)...", ep.URL, ep.Comment)
			result, err := tlsService.ScanTLS(ctx, nil, ep.URL, true) // nil userID for default endpoints
			if err != nil {
				log.Printf("  ❌ %s: scan failed - %v", ep.URL, err)
				continue
			}

			// Log success
			pqcInfo := ""
			if result.KexPQCReady {
				pqcInfo = " [PQC Ready]"
			}
			if result.PQCMode != "" && result.PQCMode != "classical" {
				pqcInfo += " [" + result.PQCMode + "]"
			}
			log.Printf("  ✅ %s: TLS=%s%s (saved as default)", ep.URL, result.ProtocolVersion, pqcInfo)

			// Small delay to avoid overwhelming the system
			time.Sleep(500 * time.Millisecond)
		}

		log.Printf("✅ Default endpoints initialization completed")
	}()
}
