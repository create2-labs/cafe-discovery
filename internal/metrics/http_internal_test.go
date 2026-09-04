package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unsafe"
)

func TestCanonicalHTTPMethodClonesUnsafeBuffer(t *testing.T) {
	// Simulate fasthttp: Method() shares a mutable buffer; ToUpper may return that view for ASCII.
	buf := []byte("GETXXXX")
	unsafeMethod := unsafe.String(&buf[0], 3) // "GET"

	got := canonicalHTTPMethod(unsafeMethod)
	copy(buf, []byte("GETTXXX")) // mutate underlying buffer after canonicalization

	if got != "GET" {
		t.Fatalf("canonical method = %q, want GET", got)
	}
	// Buffer now reads as GETT…; owned label must stay GET.
	if string(buf[:4]) == "GETT" && got == string(buf[:4]) {
		t.Fatalf("label still aliases request buffer: got=%q buf=%q", got, buf)
	}

	// GETT typo must normalize to the safe constant GET (not a buffer view).
	buf2 := []byte("GETTXXX")
	unsafeGett := unsafe.String(&buf2[0], 4)
	got2 := canonicalHTTPMethod(unsafeGett)
	copy(buf2, []byte("POSTXXX"))
	if got2 != "GET" {
		t.Fatalf("GETT canonical = %q, want GET", got2)
	}
}

func TestHTTPMetricsSurviveMethodBufferReuse(t *testing.T) {
	initHTTPMetrics()

	buf := []byte("GETXXXX")
	methodView := unsafe.String(&buf[0], 3)

	labelMethod := sanitizeLabelValue(canonicalHTTPMethod(cloneFiberString(methodView)))
	httpRequestsTotal.WithLabelValues(labelMethod, "200", "/internal/auth/session/validate").Inc()
	httpRequestDurationSeconds.WithLabelValues(labelMethod, "200", "/internal/auth/session/validate").Observe(0.001)

	// Corrupt the original buffer as fasthttp does between requests.
	copy(buf, []byte("GETTXXX"))
	httpRequestsTotal.WithLabelValues(
		sanitizeLabelValue(canonicalHTTPMethod(cloneFiberString(unsafe.String(&buf[0], 4)))),
		"200",
		"/internal/authz/scans/:scanId/can-read",
	).Inc()

	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /metrics status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, `method="GETT"`) {
		t.Fatalf("metrics still expose raw GETT label:\n%s", body)
	}
}
