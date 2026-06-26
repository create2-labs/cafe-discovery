package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"cafe-discovery/internal/config"
	"cafe-discovery/internal/service"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func TestListDiscoveryV1WalletScans_ChainIDWithoutAddress(t *testing.T) {
	t.Parallel()
	h := &DiscoveryHandler{
		discoveryService: service.NewDiscoveryService(nil, nil, nil, nil),
		cfgChain:         &config.ChainConfig{},
		scanRead:         nil,
	}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/wallets/scans", func(c *fiber.Ctx) error {
		c.Locals("user_id", uuid.MustParse("11111111-1111-1111-1111-111111111111"))
		return h.ListDiscoveryV1WalletScans(c)
	})
	req := httptest.NewRequest(http.MethodGet, "/wallets/scans?chain_id=1", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestListDiscoveryV1TLSScans_ForbiddenAddressQuery(t *testing.T) {
	t.Parallel()
	h := &TLSHandler{scanRead: nil}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/tls/scans", func(c *fiber.Ctx) error {
		c.Locals("user_id", uuid.MustParse("22222222-2222-2222-2222-222222222222"))
		return h.ListDiscoveryV1TLSScans(c)
	})
	req := httptest.NewRequest(http.MethodGet, "/tls/scans?address=0xabc", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
}
