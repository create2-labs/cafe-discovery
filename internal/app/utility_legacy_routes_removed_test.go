package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestDiscoveryLegacyUtilityRoutesRemoved(t *testing.T) {
	t.Parallel()

	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/discovery/rpcs"},
		{http.MethodGet, "/discovery/scanners"},
		{http.MethodPost, "/discovery/assessments/request"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != fiber.StatusNotFound {
				t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusNotFound)
			}
		})
	}
}
