package service

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	// Placeholder values; the tests check that neither reaches an error text.
	providerTestSecret = "changeme-provider-secret"
	providerTestToken  = "token-marker-7f3a"
)

// useChallengeProvider starts a double of the provider, points
// challengeVerifyURL at it, and restores the variable and closes the server
// when the test ends.
func useChallengeProvider(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	previous := challengeVerifyURL
	challengeVerifyURL = server.URL
	t.Cleanup(func() {
		challengeVerifyURL = previous
		server.Close()
	})
	return server
}

// requireWrappedChallengeError checks the error is wrapped, not the sentinel
// itself, and carries neither the secret nor the token.
func requireWrappedChallengeError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !errors.Is(err, ErrTurnstileValidation) {
		t.Fatalf("error %q does not wrap ErrTurnstileValidation", err)
	}
	if err == ErrTurnstileValidation { //nolint:errorlint // the test asserts a wrapped error, not the sentinel itself
		t.Fatal("expected a wrapped error, got the sentinel itself")
	}
	requireNoCredentialInText(t, err)
}

func requireNoCredentialInText(t *testing.T, err error) {
	t.Helper()
	for _, secret := range []string{providerTestSecret, providerTestToken} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("error text %q contains a credential", err.Error())
		}
	}
}

// Behaviour of the provider branch: the status is read before the body, and
// every failure wraps the sentinel.
func TestChallengeProviderAnswers(t *testing.T) {
	const bodyMarker = "body-marker-91cd"
	tests := []struct {
		name     string
		status   int
		body     string
		wantNil  bool
		wantSame bool   // the sentinel itself
		wantText string // text the wrapped error contains
		noText   string // text the wrapped error must not contain
	}{
		{name: "200 success true", status: http.StatusOK, body: `{"success":true}`, wantNil: true},
		{name: "200 success false", status: http.StatusOK, body: `{"success":false}`, wantSame: true},
		{name: "500 with success true", status: http.StatusInternalServerError, body: `{"success":true}`, wantText: "500"},
		{name: "200 with a non-JSON body", status: http.StatusOK, body: "not json " + bodyMarker, noText: bodyMarker},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setChallengeSecret(t, providerTestSecret)
			useChallengeProvider(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = fmt.Fprint(w, tc.body)
			})

			err := verifyTurnstileToken(providerTestToken)

			switch {
			case tc.wantNil:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			case tc.wantSame:
				if err != ErrTurnstileValidation { //nolint:errorlint // the test asserts the sentinel itself
					t.Fatalf("error = %v, want the sentinel itself", err)
				}
			default:
				requireWrappedChallengeError(t, err)
				if tc.wantText != "" && !strings.Contains(err.Error(), tc.wantText) {
					t.Fatalf("error text %q does not contain %q", err.Error(), tc.wantText)
				}
				if tc.noText != "" && strings.Contains(err.Error(), tc.noText) {
					t.Fatalf("error text %q contains %q", err.Error(), tc.noText)
				}
			}
		})
	}
}

// An unreachable provider is a wrapped failure.
func TestChallengeProviderUnreachable(t *testing.T) {
	setChallengeSecret(t, providerTestSecret)
	server := useChallengeProvider(t, func(http.ResponseWriter, *http.Request) {})
	server.Close() // the URL now points at a closed server

	requireWrappedChallengeError(t, verifyTurnstileToken(providerTestToken))
}

// A provider that stalls is cut by the client timeout.
func TestChallengeProviderTimeout(t *testing.T) {
	setChallengeSecret(t, providerTestSecret)

	release := make(chan struct{})
	var once sync.Once
	releaseAll := func() { once.Do(func() { close(release) }) }

	useChallengeProvider(t, func(http.ResponseWriter, *http.Request) {
		select {
		case <-release:
		case <-time.After(3 * time.Second):
		}
	})
	// Registered after the helper, so it runs before the server closes.
	t.Cleanup(releaseAll)

	previous := challengeHTTPClient
	challengeHTTPClient = &http.Client{Timeout: 100 * time.Millisecond}
	t.Cleanup(func() { challengeHTTPClient = previous })

	start := time.Now()
	err := verifyTurnstileToken(providerTestToken)
	elapsed := time.Since(start)

	requireWrappedChallengeError(t, err)
	if elapsed >= time.Second {
		t.Fatalf("call took %v, want under 1s", elapsed)
	}
}

// The one client of every provider call carries the 5 second bound.
func TestChallengeHTTPClientTimeout(t *testing.T) {
	if challengeVerifyTimeout != 5*time.Second {
		t.Fatalf("challengeVerifyTimeout = %v, want 5s", challengeVerifyTimeout)
	}
	if challengeHTTPClient.Timeout != challengeVerifyTimeout {
		t.Fatalf("challengeHTTPClient.Timeout = %v, want %v", challengeHTTPClient.Timeout, challengeVerifyTimeout)
	}
}
