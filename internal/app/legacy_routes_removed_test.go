package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cafe-discovery/internal/config"
	"cafe-discovery/internal/discoveryroutes"
	"cafe-discovery/internal/handler"
	"cafe-discovery/internal/service"

	"github.com/gofiber/fiber/v2"
)

func newLegacyRouteTestApp(t *testing.T) *fiber.App {
	t.Helper()

	authSvc, err := service.NewAuthService(nil, nil, "imm11-legacy-route-test-secret", time.Hour)
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}

	discoveryH := handler.NewDiscoveryHandlerForContractTest(nil, &config.ChainConfig{
		Blockchains: []config.Blockchain{
			{Name: "ethereum-mainnet", RPC: "https://ethereum-rpc.example"},
		},
	})
	tlsH := handler.NewTLSHandler(nil, nil, nil, nil, nil, nil, nil, nil)
	authH := handler.NewAuthHandler(authSvc, nil)
	cafeWalletH := handler.NewCafeWalletHandler(nil)
	planH := handler.NewPlanHandler(nil, nil, nil)
	scanAuthzH := handler.NewScanAuthorizationHandler(nil, false)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	setupRoutes(app, discoveryH, tlsH, authH, authSvc, cafeWalletH, planH, scanAuthzH, "imm11-internal-token")
	return app
}

func TestDiscoveryLegacyRoutesRemoved_IMM11(t *testing.T) {
	t.Parallel()

	app := newLegacyRouteTestApp(t)

	sampleAddr := "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
	sampleURL := "https://example.com"

	cases := []struct {
		name   string
		method string
		path   string
	}{
		{"POST legacy scan", http.MethodPost, discoveryroutes.LegacyPostScan},
		{"GET legacy wallet scans list", http.MethodGet, discoveryroutes.LegacyGetWalletScans},
		{"GET legacy wallet scans by address", http.MethodGet, discoveryroutes.LegacyGetWalletScans + "/" + sampleAddr},
		{"GET legacy TLS scans list", http.MethodGet, discoveryroutes.LegacyGetTLSScans},
		{"GET legacy TLS scans by URL", http.MethodGet, discoveryroutes.LegacyGetTLSScans + "?url=" + sampleURL},
		{"POST legacy TLS scan", http.MethodPost, discoveryroutes.LegacyPostTLSScan},
		{"GET wallet-policy-contexts", http.MethodGet, discoveryroutes.LegacyWalletPolicyContexts},
		{"GET wallet-policy-contexts by scan", http.MethodGet, discoveryroutes.LegacyWalletPolicyContexts + "/550e8400-e29b-41d4-a716-446655440000"},
		{"GET legacy CBOM by address", http.MethodGet, discoveryroutes.LegacyCBOMPrefix + "/" + sampleAddr},
		{"GET legacy utilities rpcs", http.MethodGet, discoveryroutes.LegacyGetRPCs},
		{"GET legacy utilities scanners", http.MethodGet, discoveryroutes.LegacyGetScanners},
		{"POST legacy assessments", http.MethodPost, discoveryroutes.LegacyGetAssessmentsRequest},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != fiber.StatusNotFound {
				t.Fatalf("%s %s status = %d, want %d", tc.method, tc.path, resp.StatusCode, fiber.StatusNotFound)
			}
		})
	}
}

func TestDiscoveryV1UtilityRoutesStillPublic_IMM11(t *testing.T) {
	t.Parallel()

	app := newLegacyRouteTestApp(t)

	for _, path := range []string{discoveryroutes.RPCs, discoveryroutes.Scanners} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("GET %s status = %d, want 200 (v1 public utilities)", path, resp.StatusCode)
		}
	}
}
