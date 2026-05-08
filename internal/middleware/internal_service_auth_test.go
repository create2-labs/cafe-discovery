package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"cafe-discovery/internal/authz"

	"github.com/gofiber/fiber/v2"
)

func newTestApp(token string) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(InternalServiceAuth(InternalServiceAuthConfig{ExpectedToken: token}))
	app.Post("/internal/ping", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"ok": true})
	})
	return app
}

func TestInternalServiceAuth_RejectsMissingHeader(t *testing.T) {
	t.Parallel()
	app := newTestApp("expected-token")
	req := httptest.NewRequest(http.MethodPost, "/internal/ping", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestInternalServiceAuth_RejectsWrongToken(t *testing.T) {
	t.Parallel()
	app := newTestApp("expected-token")
	req := httptest.NewRequest(http.MethodPost, "/internal/ping", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestInternalServiceAuth_RejectsWhenNoExpectedTokenConfigured(t *testing.T) {
	t.Parallel()
	app := newTestApp("")
	req := httptest.NewRequest(http.MethodPost, "/internal/ping", nil)
	req.Header.Set("Authorization", "Bearer anything")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 when no token configured", resp.StatusCode)
	}
}

func TestInternalServiceAuth_AcceptsValidToken(t *testing.T) {
	t.Parallel()
	app := newTestApp("expected-token")
	req := httptest.NewRequest(http.MethodPost, "/internal/ping", nil)
	req.Header.Set("Authorization", "Bearer expected-token")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestInternalServiceAuth_RequestIDIsEnsuredOnReject(t *testing.T) {
	t.Parallel()
	app := newTestApp("expected-token")
	req := httptest.NewRequest(http.MethodPost, "/internal/ping", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if got := resp.Header.Get(authz.HeaderRequestID); got == "" {
		t.Fatalf("X-Request-Id header is empty on reject; expected generated value")
	}
}
