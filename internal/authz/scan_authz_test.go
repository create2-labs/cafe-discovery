package authz

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestSanitizeRequestID_StripsHostileBytes(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"abc":                                   "abc",
		"abc-123":                               "abc-123",
		"trace_42":                              "trace_42",
		"abc\r\nInjected":                       "abcInjected",
		"abc def":                               "abcdef",
		"abc;rm -rf /":                          "abcrm-rf",
		"":                                      "",
		strings.Repeat("a", maxRequestIDLen+10): strings.Repeat("a", maxRequestIDLen),
	}
	for in, want := range cases {
		if got := SanitizeRequestID(in); got != want {
			t.Errorf("SanitizeRequestID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEnsureRequestID_GeneratesWhenEmpty(t *testing.T) {
	t.Parallel()
	got := EnsureRequestID("")
	if got == "" {
		t.Fatalf("EnsureRequestID(\"\") returned empty value")
	}
	got2 := EnsureRequestID("")
	if got == got2 {
		t.Fatalf("EnsureRequestID generated same id twice: %q", got)
	}
}

func TestEnsureRequestID_PreservesValidIncoming(t *testing.T) {
	t.Parallel()
	if got := EnsureRequestID("trace-abc-123"); got != "trace-abc-123" {
		t.Fatalf("EnsureRequestID preserved value = %q, want %q", got, "trace-abc-123")
	}
}

func TestIsValidScanID(t *testing.T) {
	t.Parallel()
	if !IsValidScanID(uuid.New().String()) {
		t.Fatalf("expected valid uuid to be accepted")
	}
	for _, bad := range []string{"", "   ", "not-a-uuid", "0x1234"} {
		if IsValidScanID(bad) {
			t.Fatalf("expected %q to be rejected", bad)
		}
	}
}

func TestPrincipalValidate(t *testing.T) {
	t.Parallel()
	if err := (Principal{}).Validate(); err == nil {
		t.Fatalf("empty principal must be invalid")
	}
	if err := (Principal{UserID: "abc"}).Validate(); err != nil {
		t.Fatalf("non-empty user id must be valid, got %v", err)
	}
}
