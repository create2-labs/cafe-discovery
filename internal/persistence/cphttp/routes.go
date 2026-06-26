// Package cphttp implements the Discovery HTTP client for cafe-persistence internal/cp/v1 (PERS-D6b).
package cphttp

// Route constants mirror cafe-persistence/internal/cproutes (openapi/internal/cp/v1.yaml).
const (
	V1Base          = "/internal/cp/v1"
	ReferenceWallet = "/references/wallet"
	ReferenceScan   = "/references/scan"
)

const (
	headerAuthorization = "Authorization"
	headerUserID        = "X-User-Id"
	headerTenantID      = "X-Tenant-Id"
)
