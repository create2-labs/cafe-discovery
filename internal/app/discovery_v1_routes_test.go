package app

import (
	"io"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gofiber/fiber/v2"
)

type testV1WalletHandlers struct {
	last atomic.Value // string: which wallet handler ran
}

func (t *testV1WalletHandlers) GetAllWallets(c *fiber.Ctx) error {
	t.last.Store("list")
	return c.SendStatus(fiber.StatusOK)
}

func (t *testV1WalletHandlers) GetWallet(c *fiber.Ctx) error {
	t.last.Store("get:" + c.Params("pubKeyHash"))
	return c.SendStatus(fiber.StatusOK)
}

func (t *testV1WalletHandlers) DeleteWallet(c *fiber.Ctx) error {
	t.last.Store("del:" + c.Params("pubKeyHash"))
	return c.SendStatus(fiber.StatusOK)
}

func TestDiscoveryV1Routes_WalletsScansNotCapturedAsPubKeyHash(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	v1 := app.Group("/discovery/v1")
	h := &testV1WalletHandlers{}
	registerDiscoveryV1Routes(v1, nil, nil, h)

	// Literal /wallets/scans must hit 501 stub, not GetWallet(pubKeyHash="scans").
	req := httptest.NewRequest("GET", "/discovery/v1/wallets/scans", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusNotImplemented {
		t.Fatalf("GET .../wallets/scans status = %d, want %d (Not Implemented stub)", resp.StatusCode, fiber.StatusNotImplemented)
	}
	if got := h.last.Load(); got != nil {
		t.Fatalf("wallet handler should not run for .../wallets/scans, last=%v", got)
	}
	_ = resp.Body.Close()
}

func TestDiscoveryV1Routes_WalletByPubKeyHashUsesParamRoute(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	v1 := app.Group("/discovery/v1")
	h := &testV1WalletHandlers{}
	registerDiscoveryV1Routes(v1, nil, nil, h)

	req := httptest.NewRequest("GET", "/discovery/v1/wallets/0xabc123", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got, _ := h.last.Load().(string); got != "get:0xabc123" {
		t.Fatalf("last handler = %q, want get:0xabc123", got)
	}
	_ = resp.Body.Close()
}

func TestDiscoveryV1Routes_TLSAndScanStubs(t *testing.T) {
	t.Parallel()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	v1 := app.Group("/discovery/v1")
	registerDiscoveryV1Routes(v1, nil, nil, &testV1WalletHandlers{})

	for _, path := range []string{
		"/discovery/v1/tls/scans",
		"/discovery/v1/tls/scans/550e8400-e29b-41d4-a716-446655440000",
	} {
		req := httptest.NewRequest("GET", path, nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		if resp.StatusCode != fiber.StatusNotImplemented {
			t.Fatalf("GET %s status = %d, want %d", path, resp.StatusCode, fiber.StatusNotImplemented)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}

	post := httptest.NewRequest("POST", "/discovery/v1/scan", nil)
	resp, err := app.Test(post, -1)
	if err != nil {
		t.Fatalf("POST /scan: %v", err)
	}
	if resp.StatusCode != fiber.StatusNotImplemented {
		t.Fatalf("POST .../scan status = %d, want %d", resp.StatusCode, fiber.StatusNotImplemented)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}
