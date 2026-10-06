package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"cafe-discovery/internal/service"

	"github.com/gofiber/fiber/v3"
)

type scriptedPresence struct {
	available bool
	indexer   string
}

func (s scriptedPresence) HasScanner(kind string) bool {
	return kind == "wallet" && s.available
}

func (s scriptedPresence) ListScanners() []service.ScannerInfo { return nil }

func (s scriptedPresence) OnchainIndexer() string { return s.indexer }

func TestGetOnchainScanner(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		presence  ScannerPresenceChecker
		available bool
		indexer   string
	}{
		{name: "no tracker", presence: nil, available: false, indexer: "unknown"},
		{name: "agreed etherscan", presence: scriptedPresence{available: true, indexer: "etherscan"}, available: true, indexer: "etherscan"},
		{name: "heartbeat alone", presence: scriptedPresence{available: true, indexer: "unknown"}, available: true, indexer: "unknown"},
		{name: "blank becomes unknown", presence: scriptedPresence{available: false, indexer: "  "}, available: false, indexer: "unknown"},
		{name: "none is kept", presence: scriptedPresence{available: true, indexer: "none"}, available: true, indexer: "none"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := &DiscoveryHandler{scannerPresence: tc.presence}
			app := fiber.New()
			app.Get("/onchainscanner", h.GetOnchainScanner)
			resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/onchainscanner", nil), fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != fiber.StatusOK {
				t.Fatalf("status = %d", resp.StatusCode)
			}
			var body struct {
				ScannerAvailable bool   `json:"scanner_available"`
				OnchainIndexer   string `json:"onchain_indexer"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.ScannerAvailable != tc.available || body.OnchainIndexer != tc.indexer {
				t.Fatalf("body = %+v, want available=%v indexer=%s", body, tc.available, tc.indexer)
			}
		})
	}
}
