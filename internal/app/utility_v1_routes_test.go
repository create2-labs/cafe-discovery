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

	"github.com/gofiber/fiber/v3"
)

func TestDiscoveryV1UtilityRoutes_PublicRPCsAndScanners(t *testing.T) {
	t.Parallel()

	h := handler.NewDiscoveryHandler(
		&config.ChainConfig{
			Blockchains: []config.Blockchain{
				{Name: "ethereum-mainnet", RPC: "https://ethereum-rpc.example"},
			},
		},
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)

	app := fiber.New(fiber.Config{})
	v1Public := app.Group(discoveryroutes.V1Base)
	v1Public.Get("/rpcs", h.ListRPCs)
	v1Public.Get("/scanners", h.ListAvailableScanners)
	v1Public.Get("/onchainscanner", h.GetOnchainScanner)

	req := httptest.NewRequest(http.MethodGet, discoveryroutes.RPCs, nil)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
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
	respScanners, err := app.Test(reqScanners, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
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

	reqIndexer := httptest.NewRequest(http.MethodGet, discoveryroutes.OnchainScanner, nil)
	respIndexer, err := app.Test(reqIndexer, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("GET onchainscanner: %v", err)
	}
	defer respIndexer.Body.Close()
	if respIndexer.StatusCode != fiber.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", discoveryroutes.OnchainScanner, respIndexer.StatusCode)
	}
	var indexerBody map[string]any
	if err := json.NewDecoder(respIndexer.Body).Decode(&indexerBody); err != nil {
		t.Fatalf("decode onchainscanner: %v", err)
	}
	if indexerBody["scanner_available"] != false {
		t.Fatalf("onchainscanner = %+v, want scanner_available false", indexerBody)
	}
	if _, ok := indexerBody["onchain_indexer"]; ok {
		t.Fatalf("onchainscanner = %+v, want no onchain_indexer", indexerBody)
	}
	if _, ok := indexerBody["indexer_gateway_available"]; ok {
		t.Fatalf("onchainscanner = %+v, want no indexer_gateway_available", indexerBody)
	}
}
