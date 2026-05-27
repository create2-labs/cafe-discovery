package discoveryroutes

// Legacy paths removed at IMM-11 / WORKPLAN_API §8.7 (superseded by v1 §0.1).
// Use in tests and deploy smoke scripts only — not served by mux or edge.
const (
	LegacyPostScan              = "/discovery/scan"
	LegacyGetWalletScans        = "/discovery/scans"
	LegacyGetTLSScans           = "/discovery/tls/scans"
	LegacyPostTLSScan           = "/discovery/tls/scan"
	LegacyWalletPolicyContexts  = "/discovery/wallet-policy-contexts"
	LegacyCBOMPrefix            = "/discovery/cbom"
	LegacyGetAssessmentsRequest = "/discovery/assessments/request"
	LegacyGetRPCs               = "/discovery/rpcs"
	LegacyGetScanners           = "/discovery/scanners"

	// Edge-facing legacy paths (generic /api/ proxy would otherwise forward these).
	LegacyEdgePostScan             = "/api/discovery/scan"
	LegacyEdgeGetWalletScans       = "/api/discovery/scans"
	LegacyEdgeGetTLSScans          = "/api/discovery/tls/scans"
	LegacyEdgeWalletPolicyContexts = "/api/discovery/wallet-policy-contexts"
	LegacyEdgeCBOMPrefix           = "/api/discovery/cbom"
)
