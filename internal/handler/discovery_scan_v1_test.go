package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"cafe-discovery/internal/discoveryroutes"
	"cafe-discovery/internal/service"
	"cafe-discovery/pkg/nats"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	natsio "github.com/nats-io/nats.go"
)

type mockNATSConn struct {
	lastSubject string
	lastData    []byte
	publishErr  error
}

func (m *mockNATSConn) Publish(subject string, data []byte) error {
	if m.publishErr != nil {
		return m.publishErr
	}
	m.lastSubject = subject
	m.lastData = append([]byte(nil), data...)
	return nil
}

func (m *mockNATSConn) Subscribe(string, func(msg *natsio.Msg)) (*natsio.Subscription, error) {
	return nil, nil
}

func (m *mockNATSConn) QueueSubscribe(string, string, func(msg *natsio.Msg)) (*natsio.Subscription, error) {
	return nil, nil
}

func (m *mockNATSConn) Close() {}

func (m *mockNATSConn) IsConnected() bool { return true }

type alwaysScanners struct{}

func (alwaysScanners) HasScanner(string) bool { return true }

func (alwaysScanners) ListScanners() []service.ScannerInfo { return nil }

// walletScannerAbsent reports no wallet scanner (TLS may still be "present" but unused in this test).
type walletScannerAbsent struct{}

func (walletScannerAbsent) HasScanner(s string) bool { return s != "wallet" }

func (walletScannerAbsent) ListScanners() []service.ScannerInfo { return nil }

func TestPostDiscoveryScanV1_WalletAccepted(t *testing.T) {
	t.Parallel()
	n := &mockNATSConn{}
	h := &DiscoveryHandler{
		discoveryService: service.NewDiscoveryService(nil, nil, nil, nil),
		natsConn:         n,
		scannerPresence:  alwaysScanners{},
	}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Post(discoveryroutes.PostScan, func(c *fiber.Ctx) error {
		c.Locals("user_id", uuid.MustParse("11111111-1111-1111-1111-111111111111"))
		return h.PostDiscoveryScanV1(c)
	})

	body := []byte(`{"address":"0x742d35Cc6634C0532925a3b844Bc454e4438f44e"}`)
	req := httptest.NewRequest(http.MethodPost, discoveryroutes.PostScan, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, b)
	}
	if n.lastSubject != nats.SubjectScanRequestedWallet {
		t.Fatalf("NATS subject = %q, want %q", n.lastSubject, nats.SubjectScanRequestedWallet)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["status"] != "requested" {
		t.Fatalf("status = %v, want requested", out["status"])
	}
	if out["scan_family"] != "wallet" {
		t.Fatalf("scan_family = %v", out["scan_family"])
	}
	scanID, _ := out["scan_id"].(string)
	if scanID == "" {
		t.Fatalf("missing scan_id")
	}
	wantLoc := discoveryroutes.EdgeWalletScans + scanID
	if out["location"] != wantLoc {
		t.Fatalf("location = %q, want %q", out["location"], wantLoc)
	}
	var published struct {
		ScanID  string `json:"scan_id"`
		Address string `json:"address"`
	}
	if err := json.Unmarshal(n.lastData, &published); err != nil {
		t.Fatalf("published json: %v", err)
	}
	if published.ScanID != scanID {
		t.Fatalf("published scan_id %q != response scan_id %q", published.ScanID, scanID)
	}
	wantAddr := "0x742d35cc6634c0532925a3b844bc454e4438f44e"
	if published.Address != wantAddr {
		t.Fatalf("published address = %q, want %q", published.Address, wantAddr)
	}
}

func TestPostDiscoveryScanV1_TLSAccepted(t *testing.T) {
	t.Parallel()
	n := &mockNATSConn{}
	h := &DiscoveryHandler{
		discoveryService: service.NewDiscoveryService(nil, nil, nil, nil),
		natsConn:         n,
		scannerPresence:  alwaysScanners{},
	}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Post(discoveryroutes.PostScan, func(c *fiber.Ctx) error {
		c.Locals("user_id", uuid.MustParse("22222222-2222-2222-2222-222222222222"))
		return h.PostDiscoveryScanV1(c)
	})
	body := []byte(`{"url":"https://example.com/path"}`)
	req := httptest.NewRequest(http.MethodPost, discoveryroutes.PostScan, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, b)
	}
	if n.lastSubject != nats.SubjectScanRequestedTLS {
		t.Fatalf("NATS subject = %q", n.lastSubject)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["scan_family"] != "tls" || out["status"] != "requested" {
		t.Fatalf("response = %#v", out)
	}
	scanID, _ := out["scan_id"].(string)
	wantLoc := discoveryroutes.EdgeTLSScans + scanID
	if out["location"] != wantLoc {
		t.Fatalf("location = %q, want %q", out["location"], wantLoc)
	}
}

func TestPostDiscoveryScanV1_BothAddressAndURL(t *testing.T) {
	t.Parallel()
	h := &DiscoveryHandler{discoveryService: service.NewDiscoveryService(nil, nil, nil, nil)}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Post("/scan", func(c *fiber.Ctx) error {
		c.Locals("user_id", uuid.New())
		return h.PostDiscoveryScanV1(c)
	})
	req := httptest.NewRequest(http.MethodPost, "/scan", bytes.NewReader([]byte(`{"address":"0x","url":"https://a"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestPostDiscoveryScanV1_NoScanner503(t *testing.T) {
	t.Parallel()
	h := &DiscoveryHandler{
		discoveryService: service.NewDiscoveryService(nil, nil, nil, nil),
		natsConn:         &mockNATSConn{},
		scannerPresence:  walletScannerAbsent{},
	}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Post("/scan", func(c *fiber.Ctx) error {
		c.Locals("user_id", uuid.New())
		return h.PostDiscoveryScanV1(c)
	})
	req := httptest.NewRequest(http.MethodPost, "/scan", bytes.NewReader([]byte(`{"address":"0x742d35Cc6634C0532925a3b844Bc454e4438f44e"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
}
