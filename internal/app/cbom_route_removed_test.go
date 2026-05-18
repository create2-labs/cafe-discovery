package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestDiscoveryCBOMRouteRemoved(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	discovery := app.Group("/discovery")
	discovery.Post("/assessments/request", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusAccepted)
	})

	req := httptest.NewRequest(http.MethodGet, "/discovery/cbom/0x742d35Cc6634C0532925a3b844Bc454e4438f44e", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("GET /discovery/cbom/* status = %d, want %d", resp.StatusCode, fiber.StatusNotFound)
	}
}
