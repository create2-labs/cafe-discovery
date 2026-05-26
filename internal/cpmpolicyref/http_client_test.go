package cpmpolicyref

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

func TestHTTPClient_ReferencedTrue(t *testing.T) {
	t.Parallel()
	uid := uuid.New()
	sid := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/policies/references/scan" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Fatalf("missing bearer")
		}
		b, _ := io.ReadAll(r.Body)
		var body map[string]any
		if err := json.Unmarshal(b, &body); err != nil {
			t.Fatal(err)
		}
		if body["scan_id"] != strings.ToLower(sid.String()) {
			t.Fatalf("scan_id = %v", body["scan_id"])
		}
		if body["user_id"] != uid.String() {
			t.Fatalf("user_id = %v", body["user_id"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"referenced": true, "count": 2})
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok", &http.Client{Timeout: 2 * time.Second})
	ref, err := c.PersistedPoliciesReferenceScan(context.Background(), uid, "tenant-x", sid)
	if err != nil {
		t.Fatal(err)
	}
	if !ref {
		t.Fatalf("want referenced true")
	}
}

func TestHTTPClient_403BecomesError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	c := NewHTTPClient(srv.URL, "tok", &http.Client{Timeout: 2 * time.Second})
	_, err := c.PersistedPoliciesReferenceScan(context.Background(), uuid.New(), "", uuid.New())
	if err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestHTTPClient_InvalidJSON(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-json"))
	}))
	defer srv.Close()
	c := NewHTTPClient(srv.URL, "tok", &http.Client{Timeout: 2 * time.Second})
	_, err := c.PersistedPoliciesReferenceScan(context.Background(), uuid.New(), "", uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHTTPClient_WalletTargetExists(t *testing.T) {
	t.Parallel()
	uid := uuid.New()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/policies/references/wallet-target" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		var body map[string]any
		if err := json.Unmarshal(b, &body); err != nil {
			t.Fatal(err)
		}
		if body["target_address"] != "0x742d35cc6634c0532925a3b844bc454e4438f44e" {
			t.Fatalf("target_address = %v", body["target_address"])
		}
		if body["user_id"] != uid.String() {
			t.Fatalf("user_id = %v", body["user_id"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"exists": true, "policy_count": 1, "draft_count": 0})
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "tok", &http.Client{Timeout: 2 * time.Second})
	ctx, err := c.ActiveWalletCPMContextForTarget(context.Background(), uid, "tenant-x", "0x742d35cc6634c0532925a3b844bc454e4438f44e")
	if err != nil {
		t.Fatal(err)
	}
	if !ctx.Exists || ctx.PolicyCount != 1 || ctx.DraftCount != 0 {
		t.Fatalf("got %+v", ctx)
	}
}

func TestHTTPClient_MissingConfig(t *testing.T) {
	t.Parallel()
	c := NewHTTPClient("", "tok", &http.Client{Timeout: 2 * time.Second})
	_, err := c.PersistedPoliciesReferenceScan(context.Background(), uuid.New(), "", uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
}
