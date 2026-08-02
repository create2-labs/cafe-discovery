package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cafe-discovery/internal/authz"
	"cafe-discovery/internal/middleware"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const (
	testServiceToken = "test-service-token-please-change-me"
)

// stubAuthzService is a minimal ScanAuthorizationDecisionService used to
// drive handler-level tests without touching repositories.
type stubAuthzService struct {
	decision   authz.Decision
	err        error
	calledWith struct {
		Principal authz.Principal
		ScanID    string
	}
	called bool
}

func (s *stubAuthzService) CanReadScan(_ context.Context, principal authz.Principal, scanID string) (authz.Decision, error) {
	s.called = true
	s.calledWith.Principal = principal
	s.calledWith.ScanID = scanID
	return s.decision, s.err
}

// newAuthzTestApp wires the AUTH-05 endpoint under the same internal
// service-auth middleware as production so tests exercise the full route.
func newAuthzTestApp(t *testing.T, stub *stubAuthzService, enabled bool, serviceToken string) *fiber.App {
	t.Helper()
	app := fiber.New(fiber.Config{})
	internal := app.Group("/internal/authz", middleware.InternalServiceAuth(middleware.InternalServiceAuthConfig{
		ExpectedToken: serviceToken,
	}))
	h := NewScanAuthorizationHandler(stub, enabled)
	internal.Post("/scans/:scanId/can-read", h.CanReadScan)
	return app
}

func newCanReadRequest(scanID, userID, tenantID, requestID, serviceToken string) *http.Request {
	url := "/internal/authz/scans/" + scanID + "/can-read"
	req := httptest.NewRequest(http.MethodPost, url, strings.NewReader(""))
	if userID != "" {
		req.Header.Set(authz.HeaderUserID, userID)
	}
	if tenantID != "" {
		req.Header.Set(authz.HeaderTenantID, tenantID)
	}
	if requestID != "" {
		req.Header.Set(authz.HeaderRequestID, requestID)
	}
	if serviceToken != "" {
		req.Header.Set("Authorization", "Bearer "+serviceToken)
	}
	return req
}

func decodeEnvelope(t *testing.T, body io.Reader) map[string]any {
	t.Helper()
	out := map[string]any{}
	if err := json.NewDecoder(body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

// 1. Allowed owner: handler returns 200 with allowed=true.
func TestCanReadScan_AllowedOwnerReturns200(t *testing.T) {
	stub := &stubAuthzService{decision: authz.Decision{Allowed: true, ReasonCode: authz.ReasonCodeAllowed}}
	app := newAuthzTestApp(t, stub, true, testServiceToken)

	req := newCanReadRequest(uuid.New().String(), uuid.New().String(), "", "req-allow-1", testServiceToken)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body := decodeEnvelope(t, resp.Body)
	if allowed, _ := body["allowed"].(bool); !allowed {
		t.Fatalf("response.allowed = %v, want true", body["allowed"])
	}
	if code, _ := body["reason_code"].(string); code != authz.ReasonCodeAllowed {
		t.Fatalf("response.reason_code = %q, want %q", code, authz.ReasonCodeAllowed)
	}
	if rid, _ := body["request_id"].(string); rid != "req-allow-1" {
		t.Fatalf("response.request_id = %q, want %q", rid, "req-allow-1")
	}
	if got := resp.Header.Get(authz.HeaderRequestID); got != "req-allow-1" {
		t.Fatalf("response header X-Request-Id = %q, want %q", got, "req-allow-1")
	}
}

// 2. Cross-user denied: handler returns 403 with allowed=false and SCAN_AUTHZ_FORBIDDEN.
func TestCanReadScan_CrossUserDeniedReturns403(t *testing.T) {
	stub := &stubAuthzService{decision: authz.Decision{Allowed: false, ReasonCode: authz.ReasonCodeForbidden}}
	app := newAuthzTestApp(t, stub, true, testServiceToken)

	req := newCanReadRequest(uuid.New().String(), uuid.New().String(), "", "", testServiceToken)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	body := decodeEnvelope(t, resp.Body)
	if allowed, _ := body["allowed"].(bool); allowed {
		t.Fatalf("response.allowed = true, want false")
	}
	if code, _ := body["reason_code"].(string); code != authz.ReasonCodeForbidden {
		t.Fatalf("response.reason_code = %q, want %q", code, authz.ReasonCodeForbidden)
	}
	assertNoSensitiveLeakage(t, body)
}

// 3. Missing principal: handler returns 401 SCAN_AUTHZ_PRINCIPAL_REQUIRED and never invokes the decision service.
func TestCanReadScan_MissingPrincipalReturns401(t *testing.T) {
	stub := &stubAuthzService{}
	app := newAuthzTestApp(t, stub, true, testServiceToken)

	req := newCanReadRequest(uuid.New().String(), "", "", "", testServiceToken)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	body := decodeEnvelope(t, resp.Body)
	if code, _ := body["reason_code"].(string); code != authz.ReasonCodePrincipalRequired {
		t.Fatalf("reason_code = %q, want %q", code, authz.ReasonCodePrincipalRequired)
	}
	if stub.called {
		t.Fatalf("decision service must not be invoked when principal is missing")
	}
}

// 4. Tenant scoping: principal tenant_id is propagated to the decision service.
//
// The Discovery scan model does not yet carry tenant_id, so the test verifies
// the contract surface (header propagation) rather than enforcement. The
// service layer test documents the future deny-on-mismatch behavior.
func TestCanReadScan_TenantHeaderIsPropagated(t *testing.T) {
	stub := &stubAuthzService{decision: authz.Decision{Allowed: true, ReasonCode: authz.ReasonCodeAllowed}}
	app := newAuthzTestApp(t, stub, true, testServiceToken)

	req := newCanReadRequest(uuid.New().String(), uuid.New().String(), "tenant-a", "", testServiceToken)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if stub.calledWith.Principal.TenantID != "tenant-a" {
		t.Fatalf("decision service tenant_id = %q, want %q", stub.calledWith.Principal.TenantID, "tenant-a")
	}
}

// 5. Unknown scan: handler returns 403 SCAN_AUTHZ_NOT_VISIBLE without leaking existence.
func TestCanReadScan_UnknownScanReturns403NotVisible(t *testing.T) {
	stub := &stubAuthzService{decision: authz.Decision{Allowed: false, ReasonCode: authz.ReasonCodeNotVisible}}
	app := newAuthzTestApp(t, stub, true, testServiceToken)

	req := newCanReadRequest(uuid.New().String(), uuid.New().String(), "", "", testServiceToken)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	body := decodeEnvelope(t, resp.Body)
	if code, _ := body["reason_code"].(string); code != authz.ReasonCodeNotVisible {
		t.Fatalf("reason_code = %q, want %q", code, authz.ReasonCodeNotVisible)
	}
	assertNoSensitiveLeakage(t, body)
}

// 6. Malformed scanID: handler returns 400 SCAN_AUTHZ_SCAN_ID_MALFORMED before invoking the service.
func TestCanReadScan_MalformedScanIDReturns400(t *testing.T) {
	stub := &stubAuthzService{}
	app := newAuthzTestApp(t, stub, true, testServiceToken)

	req := newCanReadRequest("not-a-uuid", uuid.New().String(), "", "", testServiceToken)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	body := decodeEnvelope(t, resp.Body)
	if code, _ := body["reason_code"].(string); code != authz.ReasonCodeScanIDMalformed {
		t.Fatalf("reason_code = %q, want %q", code, authz.ReasonCodeScanIDMalformed)
	}
	if stub.called {
		t.Fatalf("decision service must not be invoked when scan id is malformed")
	}
}

// 7. Service auth: missing/invalid bearer token is rejected, valid token allows evaluation.
func TestCanReadScan_ServiceAuthRequired(t *testing.T) {
	stub := &stubAuthzService{decision: authz.Decision{Allowed: true, ReasonCode: authz.ReasonCodeAllowed}}
	app := newAuthzTestApp(t, stub, true, testServiceToken)

	t.Run("missing token", func(t *testing.T) {
		req := newCanReadRequest(uuid.New().String(), uuid.New().String(), "", "", "")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", resp.StatusCode)
		}
		body := decodeEnvelope(t, resp.Body)
		if code, _ := body["reason_code"].(string); code != authz.ReasonCodeServiceAuthRequired {
			t.Fatalf("reason_code = %q, want %q", code, authz.ReasonCodeServiceAuthRequired)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		req := newCanReadRequest(uuid.New().String(), uuid.New().String(), "", "", "wrong-token")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", resp.StatusCode)
		}
	})

	t.Run("valid token", func(t *testing.T) {
		req := newCanReadRequest(uuid.New().String(), uuid.New().String(), "", "", testServiceToken)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
	})
}

// 8. Request id: incoming X-Request-Id is returned in header and JSON; missing -> generated.
func TestCanReadScan_RequestIDPropagationAndGeneration(t *testing.T) {
	stub := &stubAuthzService{decision: authz.Decision{Allowed: true, ReasonCode: authz.ReasonCodeAllowed}}
	app := newAuthzTestApp(t, stub, true, testServiceToken)

	t.Run("propagates incoming id", func(t *testing.T) {
		req := newCanReadRequest(uuid.New().String(), uuid.New().String(), "", "trace-abc-123", testServiceToken)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		if got := resp.Header.Get(authz.HeaderRequestID); got != "trace-abc-123" {
			t.Fatalf("response X-Request-Id = %q, want %q", got, "trace-abc-123")
		}
		body := decodeEnvelope(t, resp.Body)
		if rid, _ := body["request_id"].(string); rid != "trace-abc-123" {
			t.Fatalf("response.request_id = %q, want %q", rid, "trace-abc-123")
		}
	})

	t.Run("generates id when missing", func(t *testing.T) {
		req := newCanReadRequest(uuid.New().String(), uuid.New().String(), "", "", testServiceToken)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		body := decodeEnvelope(t, resp.Body)
		rid, _ := body["request_id"].(string)
		if rid == "" {
			t.Fatalf("response.request_id is empty; expected generated value")
		}
		if header := resp.Header.Get(authz.HeaderRequestID); header != rid {
			t.Fatalf("response header X-Request-Id (%q) does not match body (%q)", header, rid)
		}
	})

	t.Run("sanitizes hostile incoming id", func(t *testing.T) {
		req := newCanReadRequest(uuid.New().String(), uuid.New().String(), "", "abc\r\nInjected: yes", testServiceToken)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		if injected := resp.Header.Get("Injected"); injected != "" {
			t.Fatalf("hostile request id leaked into response headers: %q", injected)
		}
		body := decodeEnvelope(t, resp.Body)
		rid, _ := body["request_id"].(string)
		if strings.ContainsAny(rid, "\r\n") || strings.Contains(rid, " ") {
			t.Fatalf("returned request_id %q is not sanitized", rid)
		}
	})
}

// 9. No sensitive leakage: deny responses must not contain scan owner, address, endpoint, etc.
func TestCanReadScan_NoSensitiveLeakageOnDeny(t *testing.T) {
	stub := &stubAuthzService{decision: authz.Decision{Allowed: false, ReasonCode: authz.ReasonCodeForbidden}}
	app := newAuthzTestApp(t, stub, true, testServiceToken)

	req := newCanReadRequest(uuid.New().String(), uuid.New().String(), "tenant-a", "trace-no-leak", testServiceToken)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	body := decodeEnvelope(t, resp.Body)

	allowedKeys := map[string]struct{}{
		"allowed":     {},
		"reason_code": {},
		"request_id":  {},
	}
	for k := range body {
		if _, ok := allowedKeys[k]; !ok {
			t.Fatalf("deny response contains unexpected key %q (potential leakage)", k)
		}
	}
	assertNoSensitiveLeakage(t, body)
}

// 10. CPM AUTH-02 contract compatibility: allow=200, deny=403, unavailable=503.
func TestCanReadScan_CPMAUTH02ContractCompatibility(t *testing.T) {
	t.Run("allow 200", func(t *testing.T) {
		stub := &stubAuthzService{decision: authz.Decision{Allowed: true, ReasonCode: authz.ReasonCodeAllowed}}
		app := newAuthzTestApp(t, stub, true, testServiceToken)
		req := newCanReadRequest(uuid.New().String(), uuid.New().String(), "", "", testServiceToken)
		resp, _ := app.Test(req)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("allow status = %d, want 200", resp.StatusCode)
		}
		body := decodeEnvelope(t, resp.Body)
		if allowed, _ := body["allowed"].(bool); !allowed {
			t.Fatalf("allow response.allowed = false, want true")
		}
	})

	t.Run("deny 403", func(t *testing.T) {
		stub := &stubAuthzService{decision: authz.Decision{Allowed: false, ReasonCode: authz.ReasonCodeForbidden}}
		app := newAuthzTestApp(t, stub, true, testServiceToken)
		req := newCanReadRequest(uuid.New().String(), uuid.New().String(), "", "", testServiceToken)
		resp, _ := app.Test(req)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("deny status = %d, want 403", resp.StatusCode)
		}
	})

	t.Run("not visible 403", func(t *testing.T) {
		stub := &stubAuthzService{decision: authz.Decision{Allowed: false, ReasonCode: authz.ReasonCodeNotVisible}}
		app := newAuthzTestApp(t, stub, true, testServiceToken)
		req := newCanReadRequest(uuid.New().String(), uuid.New().String(), "", "", testServiceToken)
		resp, _ := app.Test(req)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("not-visible status = %d, want 403", resp.StatusCode)
		}
	})

	t.Run("decision unavailable 503", func(t *testing.T) {
		stub := &stubAuthzService{err: errors.New("backend boom")}
		app := newAuthzTestApp(t, stub, true, testServiceToken)
		req := newCanReadRequest(uuid.New().String(), uuid.New().String(), "", "", testServiceToken)
		resp, _ := app.Test(req)
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("unavailable status = %d, want 503", resp.StatusCode)
		}
		body := decodeEnvelope(t, resp.Body)
		if code, _ := body["reason_code"].(string); code != authz.ReasonCodeUnavailable {
			t.Fatalf("unavailable reason_code = %q, want %q", code, authz.ReasonCodeUnavailable)
		}
	})

	t.Run("disabled 503", func(t *testing.T) {
		stub := &stubAuthzService{decision: authz.Decision{Allowed: true, ReasonCode: authz.ReasonCodeAllowed}}
		app := newAuthzTestApp(t, stub, false, testServiceToken)
		req := newCanReadRequest(uuid.New().String(), uuid.New().String(), "", "", testServiceToken)
		resp, _ := app.Test(req)
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("disabled status = %d, want 503", resp.StatusCode)
		}
		body := decodeEnvelope(t, resp.Body)
		if code, _ := body["reason_code"].(string); code != authz.ReasonCodeDisabled {
			t.Fatalf("disabled reason_code = %q, want %q", code, authz.ReasonCodeDisabled)
		}
	})
}

// assertNoSensitiveLeakage checks that the response body does not contain
// any value resembling a scan owner, scan metadata, wallet address, endpoint
// target, email, or service credential. The check is intentionally simple:
// the deny response envelope is a fixed JSON shape so any extra field would
// be a regression.
func assertNoSensitiveLeakage(t *testing.T, body map[string]any) {
	t.Helper()

	forbiddenKeys := []string{
		"owner_user_id", "owner", "user_id", "tenant_id",
		"address", "wallet", "endpoint", "url", "host",
		"email", "token", "service_token", "authorization",
		"scan", "scan_id", "scanId", // never echo the scan id either
	}
	for _, k := range forbiddenKeys {
		if _, ok := body[k]; ok {
			t.Fatalf("response body contains forbidden key %q (leakage)", k)
		}
	}
	for k, v := range body {
		s, ok := v.(string)
		if !ok {
			continue
		}
		lowered := strings.ToLower(s)
		if strings.Contains(lowered, "@") {
			t.Fatalf("response body field %q looks like an email: %q", k, s)
		}
		if strings.HasPrefix(lowered, "0x") && len(lowered) > 6 {
			t.Fatalf("response body field %q looks like a wallet address: %q", k, s)
		}
		if strings.Contains(lowered, testServiceToken) {
			t.Fatalf("response body field %q leaks the service token", k)
		}
	}
}
