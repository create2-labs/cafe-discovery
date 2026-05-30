package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"cafe-discovery/internal/config"
	"cafe-discovery/internal/discoveryroutes"
	"cafe-discovery/internal/handler"

	"github.com/gofiber/fiber/v2"
)

func TestDiscoveryV1UtilityRoutes_PublicRPCsAndScanners(t *testing.T) {
	t.Parallel()

	h := handler.NewDiscoveryHandler(
		nil, nil,
		&config.ChainConfig{
			Blockchains: []config.Blockchain{
				{Name: "ethereum-mainnet", RPC: "https://ethereum-rpc.example"},
			},
		},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	v1Public := app.Group(discoveryroutes.V1Base)
	v1Public.Get("/rpcs", h.ListRPCs)
	v1Public.Get("/scanners", h.ListAvailableScanners)

	req := httptest.NewRequest(http.MethodGet, discoveryroutes.RPCs, nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("GET rpcs: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", discoveryroutes.RPCs, resp.StatusCode)
	}
	var rpcBody struct {
		Count       int `json:"count"`
		Blockchains []struct {
			Name string `json:"name"`
			RPC  string `json:"rpc"`
		} `json:"blockchains"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcBody); err != nil {
		t.Fatalf("decode rpcs: %v", err)
	}
	if rpcBody.Count != 1 || len(rpcBody.Blockchains) != 1 || rpcBody.Blockchains[0].Name != "ethereum-mainnet" {
		t.Fatalf("unexpected rpcs body: %+v", rpcBody)
	}

	reqScanners := httptest.NewRequest(http.MethodGet, discoveryroutes.Scanners, nil)
	respScanners, err := app.Test(reqScanners, -1)
	if err != nil {
		t.Fatalf("GET scanners: %v", err)
	}
	defer respScanners.Body.Close()
	if respScanners.StatusCode != fiber.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", discoveryroutes.Scanners, respScanners.StatusCode)
	}
	var scannersBody struct {
		Scanners []any `json:"scanners"`
	}
	if err := json.NewDecoder(respScanners.Body).Decode(&scannersBody); err != nil {
		t.Fatalf("decode scanners: %v", err)
	}
	if scannersBody.Scanners == nil {
		t.Fatalf("scanners field missing")
	}
	_, _ = io.Copy(io.Discard, respScanners.Body)
}
