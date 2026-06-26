package scanhttp_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cafe-discovery/internal/persistence/scanhttp"
	"cafe-discovery/internal/persistence/scanpending"
)

func TestClientReserveWalletPendingConflict(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+scanhttp.V1Base+scanhttp.PendingWallet, func(w http.ResponseWriter, r *http.Request) {
		requireBearer(t, r)
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"SCAN_IN_PROGRESS","message":"busy"}`))
	})

	client := newPendingTestClient(t, mux)
	reserved, err := client.ReserveWallet(t.Context(), testUserID, "", &scanpending.Record{
		ScanID:  testScanID,
		Address: "0xabc",
	})
	if err != nil {
		t.Fatalf("ReserveWallet: %v", err)
	}
	if reserved {
		t.Fatal("want reserved=false on 409")
	}
}

func TestClientReserveWalletPendingOK(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+scanhttp.V1Base+scanhttp.PendingWallet, func(w http.ResponseWriter, r *http.Request) {
		requireBearer(t, r)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"reserved": true,
			"record": map[string]any{
				"scan_id": testScanID.String(), "user_id": testUserID.String(),
				"family": "wallet", "address": "0xabc", "created_at": now,
			},
		})
	})

	client := newPendingTestClient(t, mux)
	reserved, err := client.ReserveWallet(t.Context(), testUserID, "tenant-a", &scanpending.Record{
		ScanID:    testScanID,
		UserID:    testUserID,
		Address:   "0xabc",
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("ReserveWallet: %v", err)
	}
	if !reserved {
		t.Fatal("want reserved=true")
	}
}

func TestClientGetPendingScanNotFound(t *testing.T) {
	mux := http.NewServeMux()
	path := scanhttp.V1Base + strings.ReplaceAll(scanhttp.PendingByScanID, "{scan_id}", testScanID.String())
	mux.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) {
		requireBearer(t, r)
		w.WriteHeader(http.StatusNotFound)
	})

	client := newPendingTestClient(t, mux)
	rec, err := client.Get(t.Context(), testUserID, "", testScanID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec != nil {
		t.Fatalf("want nil on 404, got %+v", rec)
	}
}

func TestClientPutTLSPendingUnavailable(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+scanhttp.V1Base+scanhttp.PendingTLS, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	client := newPendingTestClient(t, mux)
	err := client.PutTLS(t.Context(), testUserID, "", &scanpending.Record{
		ScanID:   testScanID,
		Endpoint: "https://example.com",
	})
	if err != scanpending.ErrUnavailable {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}

func TestClientDeletePendingScan(t *testing.T) {
	deleted := false
	path := scanhttp.V1Base + strings.ReplaceAll(scanhttp.PendingByScanID, "{scan_id}", testScanID.String())
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE "+path, func(w http.ResponseWriter, r *http.Request) {
		requireBearer(t, r)
		deleted = true
		w.WriteHeader(http.StatusNoContent)
	})

	client := newPendingTestClient(t, mux)
	if err := client.Delete(t.Context(), testUserID, "", testScanID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !deleted {
		t.Fatal("pending delete not called")
	}
}

func newPendingTestClient(t *testing.T, handler http.Handler) scanpending.Store {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return scanhttp.NewClient(scanhttp.Config{
		BaseURL:    srv.URL,
		Token:      testToken,
		HTTPClient: srv.Client(),
	})
}
