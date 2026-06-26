package cphttp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestClient_ScanReferencedTrue(t *testing.T) {
	t.Parallel()
	uid := uuid.New()
	sid := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != V1Base+ReferenceScan {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Fatalf("missing bearer")
		}
		if r.Header.Get("X-User-Id") != uid.String() {
			t.Fatalf("user_id = %q", r.Header.Get("X-User-Id"))
		}
		if r.URL.Query().Get("scan_id") != strings.ToLower(sid.String()) {
			t.Fatalf("scan_id = %q", r.URL.Query().Get("scan_id"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"referenced": true, "count": 2})
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, Token: "tok", HTTPClient: &http.Client{Timeout: 2 * time.Second}})
	ref, err := c.PersistedPoliciesReferenceScan(context.Background(), uid, "tenant-x", sid)
	if err != nil {
		t.Fatal(err)
	}
	if !ref {
		t.Fatal("want referenced true")
	}
}

func TestClient_Scan403BecomesError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	c := NewClient(Config{BaseURL: srv.URL, Token: "tok", HTTPClient: &http.Client{Timeout: 2 * time.Second}})
	_, err := c.PersistedPoliciesReferenceScan(context.Background(), uuid.New(), "", uuid.New())
	if err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestClient_ScanInvalidJSON(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "not-json")
	}))
	defer srv.Close()
	c := NewClient(Config{BaseURL: srv.URL, Token: "tok", HTTPClient: &http.Client{Timeout: 2 * time.Second}})
	_, err := c.PersistedPoliciesReferenceScan(context.Background(), uuid.New(), "", uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_WalletTargetExists(t *testing.T) {
	t.Parallel()
	uid := uuid.New()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != V1Base+ReferenceWallet {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("wallet_address") != "0x742d35cc6634c0532925a3b844bc454e4438f44e" {
			t.Fatalf("wallet_address = %q", r.URL.Query().Get("wallet_address"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"exists": true, "policy_count": 1, "draft_count": 0})
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, Token: "tok", HTTPClient: &http.Client{Timeout: 2 * time.Second}})
	ctx, err := c.ActiveWalletCPMContextForTarget(context.Background(), uid, "tenant-x", "0x742d35cc6634c0532925a3b844bc454e4438f44e")
	if err != nil {
		t.Fatal(err)
	}
	if !ctx.Exists || ctx.PolicyCount != 1 || ctx.DraftCount != 0 {
		t.Fatalf("got %+v", ctx)
	}
}

func TestClient_MissingConfig(t *testing.T) {
	t.Parallel()
	c := NewClient(Config{BaseURL: "", Token: "tok", HTTPClient: &http.Client{Timeout: 2 * time.Second}})
	_, err := c.PersistedPoliciesReferenceScan(context.Background(), uuid.New(), "", uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}
