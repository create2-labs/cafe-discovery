// Package scanhttp implements the Discovery HTTP client for cafe-persistence internal/scan/v1 (PERS-D6a-read).
package scanhttp

// Route constants mirror cafe-persistence/internal/scanroutes (openapi/internal/scan/v1.yaml).
const (
	V1Base           = "/internal/scan/v1"
	WalletScans      = "/wallets/scans"
	WalletScanByID   = "/wallets/scans/{scan_id}"
	TLSScans         = "/tls/scans"
	TLSScansDefaults = "/tls/scans/defaults"
	TLSScanByID      = "/tls/scans/{scan_id}"
)

const (
	headerAuthorization = "Authorization"
	headerUserID        = "X-User-Id"
	headerTenantID      = "X-Tenant-Id"
)
