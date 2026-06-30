package metrics_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cafe-discovery/internal/metrics"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
)

func TestMetricsHandlerReturns200(t *testing.T) {
	m := metrics.Init()
	m.RecordWalletScan(time.Second, true)

	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /metrics status=%d body=%q", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() == 0 {
		t.Fatal("GET /metrics returned empty body")
	}
}

func TestMetricsHandlerThroughFiberAdaptor(t *testing.T) {
	metrics.Init()

	app := fiber.New()
	app.Use(metrics.HTTPMiddleware())
	app.Get("/metrics", adaptor.HTTPHandler(metrics.Handler()))

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/metrics", nil), -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /metrics status=%d body=%q", resp.StatusCode, string(body))
	}
}

func TestMetricsHandlerAfterUnmatchedRequests(t *testing.T) {
	metrics.Init()

	app := fiber.New()
	app.Use(metrics.HTTPMiddleware())
	app.Get("/metrics", adaptor.HTTPHandler(metrics.Handler()))

	for _, path := range []string{
		"/robots.txt",
		"/discovery/v1/wallets/scans/not-a-uuid",
		"/%ff%fe",
		"/path/with\"quote",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if _, err := app.Test(req, -1); err != nil {
			t.Fatalf("request %q: %v", path, err)
		}
	}

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/metrics", nil), -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /metrics status=%d body=%q", resp.StatusCode, string(body))
	}
}
