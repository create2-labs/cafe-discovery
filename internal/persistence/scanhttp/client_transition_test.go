package scanhttp_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cafe-discovery/internal/persistence/scanhttp"
)

func TestApplyWalletTransitionTwoCallsAreTwoRequests(t *testing.T) {
	var n int
	path := scanhttp.V1Base + strings.ReplaceAll(scanhttp.WalletScanTransition, "{scan_id}", testScanID.String())
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+path, func(w http.ResponseWriter, r *http.Request) {
		n++
		requireBearer(t, r)
		if r.Header.Get("X-Tenant-Id") != "" {
			t.Fatal("tenant header set")
		}
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"outcome":"applied","status":"RUNNING","projection_ready":false,"ready_published":false}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := scanhttp.NewClient(scanhttp.Config{BaseURL: srv.URL, Token: testToken, HTTPClient: srv.Client()})

	if _, _, _, err := client.ApplyWalletTransition(t.Context(), testUserID, testScanID, []byte(`{}`)); err == nil {
		t.Fatal("non-200 must not be an outcome")
	}
	outcome, status, ready, err := client.ApplyWalletTransition(t.Context(), testUserID, testScanID, []byte(`{}`))
	if err != nil || outcome != "applied" || status != "RUNNING" || ready {
		t.Fatalf("outcome %q status %q ready %v err %v", outcome, status, ready, err)
	}
	if n != 2 {
		t.Fatalf("physical requests = %d, want 2 not 4", n)
	}
}
