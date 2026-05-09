// Package authz defines the internal scan-authorization contract that Discovery
// exposes to the Crypto Policy Management service (CPM) as part of AUTH-05.
//
// Discovery remains the authoritative source for scan visibility. CPM
// authenticates the caller (AUTH-01) and delegates scan authorization to
// Discovery via this contract (CPM AUTH-02). CPM must not read Discovery
// persistence directly.
package authz

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

// Reason codes returned by the scan-authorization decision service and
// surfaced verbatim to CPM in the response body. They are deliberately stable
// strings so CPM and operators can rely on them for traceability and metrics.
const (
	// ReasonCodeAllowed is returned when the principal is allowed to read the scan.
	ReasonCodeAllowed = "SCAN_AUTHZ_ALLOWED"

	// ReasonCodeForbidden is returned when the scan exists but the principal is
	// not its owner or otherwise authorized to read it.
	ReasonCodeForbidden = "SCAN_AUTHZ_FORBIDDEN"

	// ReasonCodeNotVisible is returned when the scan does not exist or is not
	// visible to the principal. Discovery returns 403 (not 404) for this case
	// to align with CPM AUTH-02 fail-closed semantics; anti-enumeration 404
	// hardening is deferred to a later PR.
	ReasonCodeNotVisible = "SCAN_AUTHZ_NOT_VISIBLE"

	// ReasonCodeScanIDMalformed is returned when the path scan id is empty or
	// not a syntactically valid identifier (e.g. not a UUID).
	ReasonCodeScanIDMalformed = "SCAN_AUTHZ_SCAN_ID_MALFORMED"

	// ReasonCodePrincipalRequired is returned when the propagated principal
	// (X-User-Id) is missing or unusable. Discovery does not synthesize a
	// principal; CPM is expected to propagate the authenticated user.
	ReasonCodePrincipalRequired = "SCAN_AUTHZ_PRINCIPAL_REQUIRED"

	// ReasonCodeServiceAuthRequired is returned when the internal service
	// authentication is missing or invalid.
	ReasonCodeServiceAuthRequired = "SCAN_AUTHZ_SERVICE_AUTH_REQUIRED"

	// ReasonCodeUnavailable is returned when the authorization decision
	// cannot be resolved (e.g. backend repository error). CPM is expected to
	// fail closed on any 5xx response.
	ReasonCodeUnavailable = "SCAN_AUTHZ_UNAVAILABLE"

	// ReasonCodeDisabled is returned when the internal authorization endpoint
	// has been explicitly disabled via configuration.
	ReasonCodeDisabled = "SCAN_AUTHZ_DISABLED"
)

// HeaderUserID is the header name CPM uses to propagate the authenticated
// user id to Discovery. Discovery treats this header as authoritative only
// after the internal service-auth check passes.
const (
	HeaderUserID    = "X-User-Id"
	HeaderTenantID  = "X-Tenant-Id"
	HeaderRequestID = "X-Request-Id"
)

// Principal represents the propagated identity coming from the authenticated
// CPM caller. The fields mirror the CPM Principal contract defined in
// cafe-crypto-policy-mgt/internal/authz.
type Principal struct {
	UserID   string
	TenantID string
}

// Validate checks that the principal is usable for an authorization decision.
// Discovery does not synthesize principal fields; CPM is the producer of the
// propagated headers and must populate UserID at minimum.
func (p Principal) Validate() error {
	if strings.TrimSpace(p.UserID) == "" {
		return ErrPrincipalUserIDRequired
	}
	return nil
}

// Decision is the outcome of a scan-authorization evaluation. The struct is
// deliberately small and free of scan metadata to enforce the privacy rules:
// deny responses must not leak owner, tenant, address, endpoint, or any
// scan attribute back to the caller.
type Decision struct {
	Allowed    bool
	ReasonCode string
}

// Request is the input to the authorization service; ScanID is propagated
// from the URL path and Principal is built from validated headers.
type Request struct {
	ScanID    string
	Principal Principal
	RequestID string
}

// SanitizeRequestID returns a sanitized version of the incoming X-Request-Id.
// Allowed characters are ASCII letters, digits, hyphens, and underscores;
// other bytes are stripped. The result is capped at 128 characters to keep
// log/metric labels predictable.
func SanitizeRequestID(raw string) string {
	if raw == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		switch {
		case unicode.IsLetter(r) && r < unicode.MaxASCII:
			b.WriteRune(r)
		case unicode.IsDigit(r) && r < unicode.MaxASCII:
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		}
		if b.Len() >= maxRequestIDLen {
			break
		}
	}
	return b.String()
}

// EnsureRequestID returns a sanitized incoming request id, or generates a
// new opaque id when the incoming value is empty after sanitization. The
// returned value is always non-empty.
func EnsureRequestID(raw string) string {
	if cleaned := SanitizeRequestID(raw); cleaned != "" {
		return cleaned
	}
	return generateRequestID()
}

const maxRequestIDLen = 128

// IsValidScanID reports whether scanID is a syntactically valid Discovery
// scan identifier. Discovery scan ids are UUIDs (gorm char(36)).
func IsValidScanID(scanID string) bool {
	scanID = strings.TrimSpace(scanID)
	if scanID == "" {
		return false
	}
	if _, err := uuid.Parse(scanID); err != nil {
		return false
	}
	return true
}

func generateRequestID() string {
	const fallbackPrefix = "req_"
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// Extremely unlikely; return a stable but degenerate identifier so
		// callers always see a non-empty request id in responses and logs.
		return fallbackPrefix + "0000000000000000"
	}
	return fallbackPrefix + hex.EncodeToString(buf[:])
}
