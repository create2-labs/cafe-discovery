package scanhttp_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cafe-discovery/internal/persistence/scanhttp"
	"cafe-discovery/internal/persistence/scanread"

	"github.com/google/uuid"
)

const testToken = "test-persistence-token"

var (
	testUserID = uuid.MustParse("11111111-1111-4111-8111-111111111111")
	testScanID = uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
)

func newTestClient(t *testing.T, handler http.Handler) scanread.Store {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return scanhttp.NewClient(scanhttp.Config{
		BaseURL:    srv.URL,
		Token:      testToken,
		HTTPClient: srv.Client(),
	})
}

func requireBearer(t *testing.T, r *http.Request) {
	t.Helper()
	if got := r.Header.Get("Authorization"); got != "Bearer "+testToken {
		t.Fatalf("authorization = %q", got)
	}
	if r.Header.Get("X-User-Id") != testUserID.String() {
		t.Fatalf("X-User-Id = %q", r.Header.Get("X-User-Id"))
	}
}

func TestClientGetWalletScan(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	row := map[string]any{
		"id": testScanID.String(), "user_id": testUserID.String(), "address": "0xabc",
		"type": "EOA", "algorithm": "ECDSA-secp256k1", "nist_level": 1,
		"status": "SUCCESS", "networks": `["ethereum"]`, "created_at": now, "updated_at": now,
	}
	mux := http.NewServeMux()
	path := scanhttp.V1Base + strings.ReplaceAll(scanhttp.WalletScanByID, "{scan_id}", testScanID.String())
	mux.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) {
		requireBearer(t, r)
		_ = json.NewEncoder(w).Encode(row)
	})

	client := newTestClient(t, mux)
	ent, err := client.GetWalletScan(t.Context(), testUserID, "tenant-a", testScanID)
	if err != nil {
		t.Fatalf("GetWalletScan: %v", err)
	}
	if ent == nil || ent.ID != testScanID {
		t.Fatalf("entity = %+v", ent)
	}
	if ent.Address != "0xabc" {
		t.Fatalf("address = %q", ent.Address)
	}
}

func TestClientGetWalletScanNotFound(t *testing.T) {
	mux := http.NewServeMux()
	path := scanhttp.V1Base + strings.ReplaceAll(scanhttp.WalletScanByID, "{scan_id}", testScanID.String())
	mux.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) {
		requireBearer(t, r)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not_found"}`))
	})

	client := newTestClient(t, mux)
	ent, err := client.GetWalletScan(t.Context(), testUserID, "", testScanID)
	if err != nil {
		t.Fatalf("GetWalletScan: %v", err)
	}
	if ent != nil {
		t.Fatalf("want nil entity on 404, got %+v", ent)
	}
}

func TestClientListWalletScansUnavailable(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+scanhttp.V1Base+scanhttp.WalletScans, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	client := newTestClient(t, mux)
	_, _, _, _, err := client.ListWalletScans(t.Context(), testUserID, "", nil)
	if err != scanread.ErrUnavailable {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}
