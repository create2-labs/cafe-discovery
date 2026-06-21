package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"cafe-discovery/internal/version"

	"github.com/gofiber/fiber/v2"
)

func TestGetVersionReturnsJSON(t *testing.T) {
	t.Setenv("APP_VERSION", "v1.2.3-test")
	t.Cleanup(func() { _ = os.Unsetenv("APP_VERSION") })

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/version", func(c *fiber.Ctx) error {
		return c.JSON(version.Payload())
	})

	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("GET /version: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var body version.Response
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode version response: %v", err)
	}
	if body.Version != "v1.2.3-test" {
		t.Fatalf("version = %q, want v1.2.3-test", body.Version)
	}
}
