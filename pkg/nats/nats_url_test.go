package nats

import (
	"bytes"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"

	"cafe-discovery/internal/config"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// urlMarker is put in every part of a URL that a case controls.
const urlMarker = "body-must-not-leak"

func TestRedactedURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"user and password", "nats://user:" + urlMarker + "@host:4222", "nats://host:4222"},
		{"user name only", "nats://" + urlMarker + "@host:4222", "nats://host:4222"},
		{"path, query, and fragment", "nats://host:4222/" + urlMarker + "?k=" + urlMarker + "#" + urlMarker, "nats://host:4222"},
		{"a list", "nats://u:" + urlMarker + "@a:4222, nats://u:" + urlMarker + "@b:4222", "nats://a:4222,nats://b:4222"},
		{"an invalid port", "nats://u:" + urlMarker + "@host:" + urlMarker, "[unparseable]"},
		{"no scheme", "u:" + urlMarker + "@host:4222", "[unparseable]"},
		{"a list with one bad entry", "nats://u:" + urlMarker + "@a:4222,nats://u:" + urlMarker + "@host:" + urlMarker, "nats://a:4222,[unparseable]"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := redactedURL(tc.raw)
			if got != tc.want {
				t.Errorf("redactedURL() = %q, want %q", got, tc.want)
			}
			if strings.Contains(got, urlMarker) {
				t.Errorf("redactedURL() carries the marker: %q", got)
			}
		})
	}
}

// closedLoopbackPort returns a port on 127.0.0.1 where nothing listens.
func closedLoopbackPort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}

func TestNewLogsAndErrorsCarryNoUserInfo(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		wantURLField string
		wantSentinel bool
	}{
		{
			name:         "unparseable URL",
			url:          "nats://u:" + urlMarker + "@host:" + urlMarker,
			wantURLField: "[unparseable]",
			wantSentinel: true,
		},
		{
			name:         "parseable URL, nothing listens",
			url:          "nats://u:" + urlMarker + "@127.0.0.1:" + closedLoopbackPort(t),
			wantURLField: "", // set below: depends on the port
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			prev := log.Logger
			log.Logger = zerolog.New(&buf)
			t.Cleanup(func() { log.Logger = prev })

			prevURL := viper.Get(config.NATSURL)
			viper.Set(config.NATSURL, tc.url)
			t.Cleanup(func() { viper.Set(config.NATSURL, prevURL) })

			want := tc.wantURLField
			if want == "" {
				want = "nats://" + tc.url[strings.LastIndex(tc.url, "@")+1:]
			}

			conn, err := New()
			if err == nil {
				conn.Close()
				t.Fatal("New() returned no error")
			}
			if tc.wantSentinel && !errors.Is(err, errNATSURLUnparseable) {
				t.Errorf("error %q does not wrap errNATSURLUnparseable", err)
			}
			if strings.Contains(err.Error(), urlMarker) {
				t.Errorf("error carries the marker: %q", err)
			}
			if strings.Contains(buf.String(), urlMarker) {
				t.Errorf("log carries the marker: %q", buf.String())
			}
			if !strings.Contains(buf.String(), `"url":"`+want+`"`) {
				t.Errorf("log url field is not %q: %q", want, buf.String())
			}
		})
	}
}
