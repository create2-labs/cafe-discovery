package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"cafe-discovery/internal/metrics"
	"cafe-discovery/internal/persistence/scanhttp"
	"cafe-discovery/pkg/scan"

	lifecyclev1 "github.com/create2-labs/cafe-contracts/discovery/scanlifecycle/v1"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog"
)

const (
	lifeScan = "6f1c0c1e-9b2a-4d3e-8f7a-1c2b3d4e5f60"
	lifeUser = "11111111-1111-4111-8111-111111111111"
	lifeAddr = "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
	lifeWhen = "2026-10-20T12:00:00Z"
	lifeID   = `"transition_id":"SCAN_LIFECYCLE:99"`
)

type lifeReply struct {
	outcome string
	status  string
	ready   bool
	err     error
}

type lifeCase struct {
	name    string
	subject string
	payload []byte
	metaErr error
	replies []lifeReply
	calls   int
	waits   int
	ack     int
	term    int
	nak     bool
	reason  string
	log     string
}

type lifeMsg struct {
	subject string
	data    []byte
	meta    *jetstream.MsgMetadata
	metaErr error
	ack     int
	term    int
	nakN    int
	nak     time.Duration
}

func (m *lifeMsg) Subject() string { return m.subject }
func (m *lifeMsg) Data() []byte    { return m.data }
func (m *lifeMsg) Metadata() (*jetstream.MsgMetadata, error) {
	if m.metaErr != nil {
		return nil, m.metaErr
	}
	return m.meta, nil
}
func (m *lifeMsg) Ack() error  { m.ack++; return nil }
func (m *lifeMsg) Term() error { m.term++; return nil }
func (m *lifeMsg) NakWithDelay(delay time.Duration) error {
	m.nakN++
	m.nak = delay
	return nil
}

type bodyLog struct{ raw [][]byte }

func TestSettleWalletLifecycle(t *testing.T) {
	started := lifeStarted(scan.KindWallet)
	completed := lifeCompleted()
	failed := lifeFailed()
	httpErr := errors.New("unavailable")
	cases := []lifeCase{
		{name: "not wallet", subject: lifecyclev1.SubjectScanStarted, payload: lifeStarted(scan.KindTLS), ack: 1},
		{name: "invalid", subject: lifecyclev1.SubjectScanStarted, payload: []byte("{"), term: 1, reason: walletLifecycleInvalid, log: "wallet lifecycle invalid"},
		{name: "metadata", subject: lifecyclev1.SubjectScanStarted, payload: started, metaErr: errors.New("no metadata"), nak: true, reason: walletLifecycleMetadata, log: "wallet lifecycle metadata unavailable"},
		{name: "applied running", subject: lifecyclev1.SubjectScanStarted, payload: started, replies: []lifeReply{{outcome: "applied", status: "RUNNING"}}, calls: 1, ack: 1},
		{name: "already applied running", subject: lifecyclev1.SubjectScanStarted, payload: started, replies: []lifeReply{{outcome: "already_applied", status: "RUNNING"}}, calls: 1, ack: 1},
		{name: "applied success", subject: lifecyclev1.SubjectScanCompleted, payload: completed, replies: []lifeReply{{outcome: "applied", status: "SUCCESS", ready: true}}, calls: 1, ack: 1},
		{name: "already applied success", subject: lifecyclev1.SubjectScanCompleted, payload: completed, replies: []lifeReply{{outcome: "already_applied", status: "SUCCESS", ready: true}}, calls: 1, ack: 1},
		{name: "projection false", subject: lifecyclev1.SubjectScanCompleted, payload: completed, replies: []lifeReply{{outcome: "applied", status: "SUCCESS"}}, calls: 1, nak: true, reason: walletLifecycleProjection, log: walletLifecycleAlert},
		{name: "already applied projection false", subject: lifecyclev1.SubjectScanFailed, payload: failed, replies: []lifeReply{{outcome: "already_applied", status: "FAILED"}}, calls: 1, nak: true, reason: walletLifecycleProjection, log: walletLifecycleAlert},
		{name: "already running", subject: lifecyclev1.SubjectScanStarted, payload: started, replies: []lifeReply{{outcome: "already_running", status: "RUNNING"}}, calls: 1, ack: 1},
		{name: "already terminal", subject: lifecyclev1.SubjectScanCompleted, payload: completed, replies: []lifeReply{{outcome: "already_terminal", status: "FAILED"}}, calls: 1, ack: 1},
		{name: "rejected with status", subject: lifecyclev1.SubjectScanCompleted, payload: completed, replies: []lifeReply{{outcome: "rejected", status: "FAILED"}}, calls: 1, term: 1, reason: walletLifecycleRejected, log: "wallet lifecycle rejected"},
		{name: "rejected without status", subject: lifecyclev1.SubjectScanFailed, payload: failed, replies: []lifeReply{{outcome: "rejected"}}, calls: 1, term: 1, reason: walletLifecycleRejected, log: "wallet lifecycle rejected"},
		{name: "two retryable with status", subject: lifecyclev1.SubjectScanCompleted, payload: completed, replies: []lifeReply{{outcome: "retryable_error", status: "RUNNING"}, {outcome: "retryable_error", status: "RUNNING"}}, calls: 2, waits: 1, nak: true, reason: walletLifecycleRetryable, log: walletLifecycleAlert},
		{name: "two retryable without status", subject: lifecyclev1.SubjectScanFailed, payload: failed, replies: []lifeReply{{outcome: "retryable_error"}, {outcome: "retryable_error"}}, calls: 2, waits: 1, nak: true, reason: walletLifecycleRetryable, log: walletLifecycleAlert},
		{name: "two client errors", subject: lifecyclev1.SubjectScanStarted, payload: started, replies: []lifeReply{{err: httpErr}, {err: httpErr}}, calls: 2, waits: 1, nak: true, reason: walletLifecycleTransport, log: walletLifecycleAlert},
		{name: "retryable then applied", subject: lifecyclev1.SubjectScanStarted, payload: started, replies: []lifeReply{{outcome: "retryable_error"}, {outcome: "applied", status: "RUNNING"}}, calls: 2, waits: 1, ack: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			apply, bodies := scriptApply(tc.replies)
			var waits int
			var reasons []string
			var buf bytes.Buffer
			msg := &lifeMsg{subject: tc.subject, data: tc.payload, meta: lifeMeta(), metaErr: tc.metaErr}
			walletDeliverySettler{
				apply: apply,
				wait: func(_ context.Context, d time.Duration) error {
					waits++
					if d != walletLifecycleGap {
						t.Errorf("gap = %s", d)
					}
					return nil
				},
				record: func(reason string) { reasons = append(reasons, reason) },
				log:    zerolog.New(&buf),
			}.settle(t.Context(), msg)
			if len(bodies.raw) != tc.calls || waits != tc.waits || msg.ack != tc.ack || msg.term != tc.term {
				t.Fatalf("calls=%d waits=%d ack=%d term=%d", len(bodies.raw), waits, msg.ack, msg.term)
			}
			if tc.nak != (msg.nakN == 1 && msg.nak == walletLifecycleNakDelay) || (!tc.nak && msg.nakN != 0) {
				t.Fatalf("nak n=%d delay=%s", msg.nakN, msg.nak)
			}
			if tc.reason == "" && len(reasons) != 0 || (tc.reason != "" && (len(reasons) != 1 || reasons[0] != tc.reason)) {
				t.Fatalf("reasons=%v", reasons)
			}
			logged := buf.String()
			if tc.log != "" && !strings.Contains(logged, tc.log) {
				t.Fatalf("log %q", logged)
			}
			if tc.log != walletLifecycleAlert && strings.Contains(logged, walletLifecycleAlert) {
				t.Fatalf("unexpected alert %q", logged)
			}
			for i, raw := range bodies.raw {
				if !bytes.Contains(raw, []byte(lifeID)) {
					t.Fatalf("body %d = %s", i, raw)
				}
				if i > 0 && !bytes.Equal(bodies.raw[0], raw) {
					t.Fatalf("body %d differs", i)
				}
			}
		})
	}
}

func TestSettleWalletLifecycleHTTP503Twice(t *testing.T) {
	var n int
	var bodies [][]byte
	path := scanhttp.V1Base + strings.ReplaceAll(scanhttp.WalletScanTransition, "{scan_id}", lifeScan)
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+path, func(w http.ResponseWriter, r *http.Request) {
		n++
		raw, _ := io.ReadAll(r.Body)
		bodies = append(bodies, raw)
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := scanhttp.NewClient(scanhttp.Config{BaseURL: srv.URL, Token: "tok", HTTPClient: srv.Client()})
	msg := &lifeMsg{subject: lifecyclev1.SubjectScanStarted, data: lifeStarted(scan.KindWallet), meta: lifeMeta()}
	metrics.Get()
	before := lifecycleValue(t, walletLifecycleTransport)
	start := time.Now()
	SettleWalletLifecycle(t.Context(), client, msg)
	if time.Since(start) < walletLifecycleGap {
		t.Fatal("second attempt was not delayed by one second")
	}
	if n != 2 || len(bodies) != 2 || !bytes.Equal(bodies[0], bodies[1]) || !bytes.Contains(bodies[0], []byte(lifeID)) {
		t.Fatalf("physical requests = %d", n)
	}
	if msg.ack != 0 || msg.term != 0 || msg.nakN != 1 || msg.nak != walletLifecycleNakDelay {
		t.Fatalf("ack=%d term=%d nak=%d %s", msg.ack, msg.term, msg.nakN, msg.nak)
	}
	if lifecycleValue(t, walletLifecycleTransport) != before+1 {
		t.Fatal("transport metric")
	}
}

func lifecycleValue(t *testing.T, reason string) float64 {
	t.Helper()
	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	prefix := `cafe_discovery_wallet_lifecycle_total{reason="` + reason + `"}`
	for line := range strings.SplitSeq(rec.Body.String(), "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		fields := strings.Fields(line)
		value, err := strconv.ParseFloat(fields[len(fields)-1], 64)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	return 0
}

func scriptApply(replies []lifeReply) (walletApply, *bodyLog) {
	logged := &bodyLog{}
	return func(_ context.Context, _, _ uuid.UUID, body []byte) (string, string, bool, error) {
		logged.raw = append(logged.raw, append([]byte(nil), body...))
		i := len(logged.raw) - 1
		if i >= len(replies) {
			return "", "", false, errors.New("unexpected apply")
		}
		r := replies[i]
		return r.outcome, r.status, r.ready, r.err
	}, logged
}

func lifeMeta() *jetstream.MsgMetadata {
	return &jetstream.MsgMetadata{Stream: "SCAN_LIFECYCLE", Sequence: jetstream.SequencePair{Consumer: 7, Stream: 99}}
}

func lifeStarted(kind string) []byte {
	return []byte(`{"scan_id":"` + lifeScan + `","kind":"` + kind + `","user_id":"` + lifeUser + `","started_at":"` + lifeWhen + `","address":"` + lifeAddr + `","endpoint":"example.com"}`)
}

func lifeCompleted() []byte {
	return []byte(`{"scan_id":"` + lifeScan + `","kind":"wallet","user_id":"` + lifeUser + `","completed_at":"` + lifeWhen + `","address":"` + lifeAddr + `","result":{"type":"unknown","address":"` + lifeAddr + `"}}`)
}

func lifeFailed() []byte {
	return []byte(`{"scan_id":"` + lifeScan + `","kind":"wallet","user_id":"` + lifeUser + `","error":"indexer timeout","completed_at":"` + lifeWhen + `","address":"` + lifeAddr + `"}`)
}
