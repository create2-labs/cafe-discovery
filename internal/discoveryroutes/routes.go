// Package discoveryroutes defines canonical HTTP path constants for Discovery API v1 (WORKPLAN_API_PR PR11c).
package discoveryroutes

const (
	// V1Base is the in-process Fiber group prefix (edge strips /api and proxies /api/discovery/v1 → /discovery/v1).
	V1Base = "/discovery/v1"

	Wallets          = V1Base + "/wallets"
	WalletScans      = Wallets + "/scans"
	TLSScans         = V1Base + "/tls/scans"
	TLSScansDefaults = TLSScans + "/defaults"
	PostScan         = V1Base + "/scan"
	RPCs             = V1Base + "/rpcs"
	Scanners         = V1Base + "/scanners"

	// EdgeV1Base is the public Location prefix returned in POST /scan responses (browser-facing).
	EdgeV1Base      = "/api/discovery/v1"
	EdgeWalletScans = EdgeV1Base + "/wallets/scans/"
	EdgeTLSScans    = EdgeV1Base + "/tls/scans/"
	EdgeRPCs        = EdgeV1Base + "/rpcs"
	EdgeScanners    = EdgeV1Base + "/scanners"
)

// WalletScanByID returns the upstream path for a wallet scan detail/delete.
func WalletScanByID(scanID string) string {
	return WalletScans + "/" + scanID
}

// WalletScanCBOMByID returns the upstream path for wallet scan CBOM (W6).
func WalletScanCBOMByID(scanID string) string {
	return WalletScans + "/" + scanID + "/cbom"
}

// TLSScanByID returns the upstream path for a TLS scan detail/delete.
func TLSScanByID(scanID string) string {
	return TLSScans + "/" + scanID
}

// TLSScanCBOMByID returns the upstream path for TLS scan CBOM (W6).
func TLSScanCBOMByID(scanID string) string {
	return TLSScans + "/" + scanID + "/cbom"
}
