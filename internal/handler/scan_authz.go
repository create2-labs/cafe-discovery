package handler

import (
	"context"
	"strings"

	"cafe-discovery/internal/authz"
	"cafe-discovery/internal/metrics"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog/log"
)

// ScanAuthorizationDecisionService is the minimal interface the AUTH-05
// handler needs from the underlying service. It is defined here (next to the
// handler) so tests can substitute a fake without touching the production
// service implementation.
type ScanAuthorizationDecisionService interface {
	CanReadScan(ctx context.Context, principal authz.Principal, scanID string) (authz.Decision, error)
}

// ScanAuthorizationHandler exposes the internal scan-authorization endpoint
// consumed by CPM (AUTH-02 -> AUTH-05). The handler is intentionally
// internal-only: it is wired behind an internal service-auth middleware and
// it must never be reachable through public ingress.
type ScanAuthorizationHandler struct {
	service ScanAuthorizationDecisionService
	enabled bool
	route   string
}

// NewScanAuthorizationHandler returns a handler that delegates the decision
// to the provided service. When enabled is false, the handler returns 503
// SCAN_AUTHZ_DISABLED so CPM fails closed.
func NewScanAuthorizationHandler(service ScanAuthorizationDecisionService, enabled bool) *ScanAuthorizationHandler {
	return &ScanAuthorizationHandler{
		service: service,
		enabled: enabled,
		route:   ScanAuthorizationRoutePattern,
	}
}

// ScanAuthorizationRoutePattern is the canonical route pattern used as a
// label value in metrics. It is also exported so the container can register
// the route with the same string the handler logs and reports.
const ScanAuthorizationRoutePattern = "/internal/authz/scans/:scanId/can-read"

// CanReadScan handles POST /internal/authz/scans/:scanId/can-read.
//
// Contract:
//   - Headers: X-User-Id (required), X-Tenant-Id (optional), X-Request-Id (optional).
//     Internal service auth is enforced by middleware before this handler runs;
//     X-User-Id is only trusted because of that gate.
//   - Body: empty.
//   - Response: { allowed, reason_code, request_id } JSON envelope.
func (h *ScanAuthorizationHandler) CanReadScan(c fiber.Ctx) error {
	requestID := authz.EnsureRequestID(c.Get(authz.HeaderRequestID))
	c.Set(authz.HeaderRequestID, requestID)

	if h == nil || h.service == nil || !h.enabled {
		return h.respond(c, fiber.StatusServiceUnavailable, false, authz.ReasonCodeDisabled, requestID, "unavailable", authz.Principal{})
	}

	rawUserID := strings.TrimSpace(c.Get(authz.HeaderUserID))
	if rawUserID == "" {
		return h.respond(c, fiber.StatusUnauthorized, false, authz.ReasonCodePrincipalRequired, requestID, "denied", authz.Principal{})
	}
	tenantID := strings.TrimSpace(c.Get(authz.HeaderTenantID))
	principal := authz.Principal{
		UserID:   rawUserID,
		TenantID: tenantID,
	}

	scanID := strings.TrimSpace(c.Params("scanId"))
	if !authz.IsValidScanID(scanID) {
		return h.respond(c, fiber.StatusBadRequest, false, authz.ReasonCodeScanIDMalformed, requestID, "malformed", principal)
	}

	decision, err := h.service.CanReadScan(c.RequestCtx(), principal, scanID)
	if err != nil {
		log.Error().
			Err(err).
			Str("request_id", requestID).
			Str("route", h.route).
			Str("user_id", principal.UserID).
			Str("tenant_id", principal.TenantID).
			Msg("scan authorization decision failed; returning 503 fail-closed")
		return h.respond(c, fiber.StatusServiceUnavailable, false, authz.ReasonCodeUnavailable, requestID, "unavailable", principal)
	}

	if decision.Allowed {
		return h.respond(c, fiber.StatusOK, true, decision.ReasonCode, requestID, "allowed", principal)
	}
	switch decision.ReasonCode {
	case authz.ReasonCodeScanIDMalformed:
		return h.respond(c, fiber.StatusBadRequest, false, decision.ReasonCode, requestID, "malformed", principal)
	case authz.ReasonCodePrincipalRequired:
		return h.respond(c, fiber.StatusUnauthorized, false, decision.ReasonCode, requestID, "denied", principal)
	case authz.ReasonCodeUnavailable, authz.ReasonCodeDisabled:
		return h.respond(c, fiber.StatusServiceUnavailable, false, decision.ReasonCode, requestID, "unavailable", principal)
	default:
		// Forbidden / not-visible / unknown deny reason -> 403.
		return h.respond(c, fiber.StatusForbidden, false, decision.ReasonCode, requestID, "denied", principal)
	}
}

func (h *ScanAuthorizationHandler) respond(c fiber.Ctx, status int, allowed bool, reasonCode, requestID, outcome string, principal authz.Principal) error {
	if h != nil {
		metrics.Get().RecordScanAuthzDecision(outcome, reasonCode, h.route)
	}
	logEvent := log.Info()
	if outcome == "unavailable" || outcome == "malformed" {
		logEvent = log.Warn()
	}
	route := ""
	if h != nil {
		route = h.route
	}
	logEvent.
		Str("request_id", requestID).
		Str("route", route).
		Str("outcome", outcome).
		Str("reason_code", reasonCode).
		Str("user_id", principal.UserID).
		Str("tenant_id", principal.TenantID).
		Bool("allowed", allowed).
		Msg("scan authorization decision")

	return c.Status(status).JSON(fiber.Map{
		"allowed":     allowed,
		"reason_code": reasonCode,
		"request_id":  requestID,
	})
}

// fiber:context-methods migrated
