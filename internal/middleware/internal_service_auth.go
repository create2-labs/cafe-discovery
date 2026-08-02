package middleware

import (
	"crypto/subtle"
	"strings"

	"cafe-discovery/internal/authz"

	"github.com/gofiber/fiber/v3"
)

// InternalServiceAuthConfig configures the internal service-to-service auth
// guard used by the AUTH-05 scan authorization endpoint.
//
// The guard is intentionally minimal: it expects a static bearer token that
// CPM (and only CPM) shares with Discovery. This is documented as a
// temporary measure until mTLS or a signed service JWT is available.
type InternalServiceAuthConfig struct {
	// ExpectedToken is the shared service token. When empty, the guard
	// rejects every request with SCAN_AUTHZ_SERVICE_AUTH_REQUIRED so the
	// internal endpoint cannot be reached without explicit configuration.
	ExpectedToken string

	// OnReject is invoked instead of writing a default 401 response. It
	// allows the handler to record metrics and structured logs alongside
	// the rejection. The handler is responsible for writing the response
	// when OnReject is provided.
	OnReject fiber.Handler
}

// InternalServiceAuth returns a Fiber middleware that protects internal
// endpoints with a static bearer token. The middleware does NOT consult any
// header that could be spoofed by browsers (e.g. X-User-Id) and it must be
// the first thing executed on the route so principal headers are only
// trusted after the service-auth check passes.
//
// TODO(auth-05-hardening): replace the static service token with first-class
// service identity (mTLS or signed service JWT) once the platform supports
// it. The token comparison is constant-time to avoid timing oracles.
func InternalServiceAuth(cfg InternalServiceAuthConfig) fiber.Handler {
	return func(c fiber.Ctx) error {
		header := strings.TrimSpace(c.Get(fiber.HeaderAuthorization))
		if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
			return rejectInternalServiceAuth(c, cfg)
		}
		raw := strings.TrimSpace(header[len("Bearer "):])
		if raw == "" {
			return rejectInternalServiceAuth(c, cfg)
		}
		expected := strings.TrimSpace(cfg.ExpectedToken)
		if expected == "" {
			return rejectInternalServiceAuth(c, cfg)
		}
		if subtle.ConstantTimeCompare([]byte(raw), []byte(expected)) != 1 {
			return rejectInternalServiceAuth(c, cfg)
		}
		return c.Next()
	}
}

func rejectInternalServiceAuth(c fiber.Ctx, cfg InternalServiceAuthConfig) error {
	if cfg.OnReject != nil {
		return cfg.OnReject(c)
	}
	requestID := authz.EnsureRequestID(c.Get(authz.HeaderRequestID))
	c.Set(authz.HeaderRequestID, requestID)
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"allowed":     false,
		"reason_code": authz.ReasonCodeServiceAuthRequired,
		"request_id":  requestID,
	})
}
