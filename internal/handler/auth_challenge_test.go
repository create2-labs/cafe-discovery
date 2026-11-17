package handler

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http/httptest"
	"strings"
	"testing"

	"cafe-discovery/internal/service"

	"github.com/gofiber/fiber/v3"
)

// useLogBuffer redirects the standard logger to a buffer and restores it when
// the test ends.
func useLogBuffer(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previousWriter := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(&buf)
	t.Cleanup(func() {
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
	})
	return &buf
}

// The client always gets the fixed challenge error; only a provider failure
// is logged.
func TestRespondChallengeFailure(t *testing.T) {
	const marker = "cause-marker-5b21"
	wantBody := fmt.Sprintf(`{"error":%q}`, service.ErrTurnstileValidation.Error())

	tests := []struct {
		name      string
		err       error
		wantLog   bool
		logMarker string
	}{
		{name: "sentinel itself", err: service.ErrTurnstileValidation},
		{
			name:      "wrapped provider failure",
			err:       fmt.Errorf("%w: stage: %w", service.ErrTurnstileValidation, errors.New(marker)),
			wantLog:   true,
			logMarker: marker,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf := useLogBuffer(t)
			app := fiber.New(fiber.Config{})
			app.Post("/challenge", func(c fiber.Ctx) error {
				return respondChallengeFailure(c, tc.err)
			})

			resp, err := app.Test(httptest.NewRequest("POST", "/challenge", nil))
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}

			if resp.StatusCode != fiber.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
			if string(body) != wantBody {
				t.Fatalf("body = %s, want %s", body, wantBody)
			}
			if strings.Contains(string(body), marker) {
				t.Fatalf("body %s carries the cause", body)
			}
			if tc.wantLog {
				if !strings.Contains(buf.String(), tc.logMarker) {
					t.Fatalf("log %q does not contain the cause", buf.String())
				}
			} else if buf.Len() != 0 {
				t.Fatalf("log = %q, want empty", buf.String())
			}
		})
	}
}
