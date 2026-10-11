package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"cafe-discovery/internal/service"

	"github.com/gofiber/fiber/v3"
	"gopkg.in/yaml.v3"
)

type scriptedPresence struct {
	available bool
	scanners  []service.ScannerInfo
}

func (s scriptedPresence) HasScanner(kind string) bool {
	return kind == "wallet" && s.available
}

func (s scriptedPresence) ListScanners() []service.ScannerInfo { return s.scanners }

func TestGetOnchainScanner(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		presence  ScannerPresenceChecker
		available bool
	}{
		{name: "no tracker", presence: nil, available: false},
		{name: "wallet up", presence: scriptedPresence{available: true}, available: true},
		{name: "wallet down", presence: scriptedPresence{available: false}, available: false},
		{
			name: "indexer-gateway presence adds no http field",
			presence: scriptedPresence{
				available: true,
				scanners:  []service.ScannerInfo{{Type: "indexer-gateway", Count: 1, IDs: []string{"gw"}}},
			},
			available: true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := getOnchainScannerBody(t, tc.presence)
			if _, ok := body["onchain_indexer"]; ok {
				t.Fatalf("body = %s", body)
			}
			if _, ok := body["indexer_gateway_available"]; ok {
				t.Fatalf("body = %s", body)
			}
			if len(body) != 1 {
				t.Fatalf("body keys = %d, want only scanner_available: %s", len(body), body)
			}
			available, ok := body["scanner_available"].(bool)
			if !ok || available != tc.available {
				t.Fatalf("scanner_available = %#v, want %v", body["scanner_available"], tc.available)
			}
		})
	}
}

func getOnchainScannerBody(t *testing.T, presence ScannerPresenceChecker) map[string]any {
	t.Helper()
	h := &DiscoveryHandler{scannerPresence: presence}
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
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestOnchainScannerOpenAPIMatchesResponse(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../../openapi/discovery-v1.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, forbidden := range []string{"onchain_indexer", "indexer_gateway_available", "etherscan", "moralis"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("spec still contains %q", forbidden)
		}
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	schema := yamlMap(t, yamlMap(t, yamlMap(t, doc, "components"), "schemas"), "OnchainScannerResponse")
	required, ok := schema["required"].([]any)
	if !ok || len(required) != 1 || required[0] != "scanner_available" {
		t.Fatalf("required = %#v", schema["required"])
	}
	props := yamlMap(t, schema, "properties")
	if _, ok := props["scanner_available"]; !ok || len(props) != 1 {
		t.Fatalf("properties = %#v", props)
	}
	get := yamlMap(t, yamlMap(t, yamlMap(t, doc, "paths"), "/onchainscanner"), "get")
	if get["operationId"] != "getOnchainScanner" {
		t.Fatalf("operationId = %#v", get["operationId"])
	}
}

func yamlMap(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	child, ok := parent[key].(map[string]any)
	if !ok {
		t.Fatalf("missing %s", key)
	}
	return child
}
