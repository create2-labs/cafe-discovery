package service

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	redisconn "cafe-discovery/pkg/redis"

	presencev1 "github.com/create2-labs/cafe-contracts/discovery/scannerpresence/v1"
	natsio "github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

func TestOnchainIndexerAgreementAndAbsentField(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	tr := newPresenceTracker(now)

	tr.handleMessage(presenceMsg(t, "joined", "w1", "wallet", "etherscan"))
	tr.handleMessage(presenceMsg(t, "joined", "w2", "wallet", "etherscan"))
	if got := tr.OnchainIndexer(); got != presencev1.IndexerEtherscan {
		t.Fatalf("indexer = %q, want etherscan", got)
	}

	tr.handleMessage(presenceMsg(t, "joined", "w2", "wallet", ""))
	if got := tr.OnchainIndexer(); got != presencev1.IndexerUnknown {
		t.Fatalf("absent field indexer = %q, want unknown", got)
	}

	tr.handleMessage(presenceMsg(t, "left", "w2", "wallet", ""))
	if got := tr.OnchainIndexer(); got != presencev1.IndexerEtherscan {
		t.Fatalf("after left indexer = %q, want etherscan", got)
	}

	tr.handleMessage(presenceMsg(t, "joined", "w3", "wallet", "moralis"))
	if got := tr.OnchainIndexer(); got != presencev1.IndexerUnknown {
		t.Fatalf("divergent indexer = %q, want unknown", got)
	}
}

func TestOnchainIndexerNoneIsDistinctFromAbsent(t *testing.T) {
	t.Parallel()
	tr := newPresenceTracker(time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC))
	tr.handleMessage(vectorMsg(t, "ok_joined_none.json"))
	if got := tr.OnchainIndexer(); got != presencev1.IndexerNone {
		t.Fatalf("indexer = %q, want none", got)
	}
	tr.handleMessage(vectorMsg(t, "ok_joined.json"))
	if got := tr.OnchainIndexer(); got != presencev1.IndexerUnknown {
		t.Fatalf("absent indexer = %q, want unknown", got)
	}
	tr.handleMessage(vectorMsg(t, "ok_invalid_indexer.json"))
	if got := tr.OnchainIndexer(); got != presencev1.IndexerUnknown {
		t.Fatalf("invalid indexer = %q, want unknown", got)
	}
	tr.handleMessage(presenceMsg(t, "joined", "scanner-1", "wallet", "   "))
	if got := tr.OnchainIndexer(); got != presencev1.IndexerUnknown {
		t.Fatalf("blank indexer = %q, want unknown", got)
	}
}

func TestHeartbeatAloneDoesNotNameAnIndexer(t *testing.T) {
	t.Parallel()
	tr := newPresenceTracker(time.Now().UTC())
	tr.redis = stubRedis{values: map[string]string{
		"scanner:wallet:last_seen": "2026-10-06T12:00:00Z",
	}}
	if !tr.HasScanner("wallet") {
		t.Fatal("redis heartbeat should make the wallet scanner available")
	}
	if got := tr.OnchainIndexer(); got != presencev1.IndexerUnknown {
		t.Fatalf("indexer = %q, want unknown", got)
	}
	if tr.HasScanner("tls") {
		t.Fatal("tls heartbeat key is absent")
	}
}

func TestWalletJoinedRefreshesSilenceAndLeavesTLS(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	tr := newPresenceTracker(start)
	tr.handleMessage(presenceMsg(t, "joined", "w1", "wallet", "moralis"))
	tr.handleMessage(presenceMsg(t, "joined", "t1", "tls", ""))

	tr.expireSilentWallets(start.Add(walletPresenceSilentAfter))
	if !tr.HasScanner("wallet") || !tr.HasScanner("tls") {
		t.Fatal("exactly 75s of silence must keep the wallet and the tls scanner")
	}
	if got := tr.OnchainIndexer(); got != presencev1.IndexerMoralis {
		t.Fatalf("indexer = %q", got)
	}

	refreshed := start.Add(70 * time.Second)
	tr.now = func() time.Time { return refreshed }
	tr.handleMessage(presenceMsg(t, "joined", "w1", "wallet", "moralis"))
	tr.expireSilentWallets(start.Add(walletPresenceSilentAfter + time.Second))
	if !tr.HasScanner("wallet") {
		t.Fatal("a later joined must keep the wallet past the original silence deadline")
	}

	tr.expireSilentWallets(refreshed.Add(walletPresenceSilentAfter + time.Nanosecond))
	if tr.HasScanner("wallet") {
		t.Fatal("wallet silent for more than 75s must leave byType")
	}
	if got := tr.OnchainIndexer(); got != presencev1.IndexerUnknown {
		t.Fatalf("expired indexer = %q", got)
	}
	if !tr.HasScanner("tls") {
		t.Fatal("wallet sweep must not remove tls presence")
	}
}

func TestWalletLeftDropsMetadataImmediately(t *testing.T) {
	t.Parallel()
	tr := newPresenceTracker(time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC))
	tr.handleMessage(presenceMsg(t, "joined", "w1", "wallet", "etherscan"))
	tr.handleMessage(presenceMsg(t, "left", "w1", "wallet", "etherscan"))
	if tr.HasScanner("wallet") {
		t.Fatal("left wallet must leave byType")
	}
	if got := tr.OnchainIndexer(); got != presencev1.IndexerUnknown {
		t.Fatalf("indexer = %q", got)
	}
	tr.mu.RLock()
	_, still := tr.walletMeta["w1"]
	tr.mu.RUnlock()
	if still {
		t.Fatal("left must delete wallet metadata")
	}
}

func TestWalletSweepGoroutineRemovesSilentWalletAndCloseStopsIt(t *testing.T) {
	start := time.Now().UTC().Add(-walletPresenceSilentAfter - time.Second)
	tr := &ScannerPresenceTracker{
		byType: map[string]map[string]struct{}{
			"wallet": {"w1": {}},
			"tls":    {"t1": {}},
		},
		walletMeta: map[string]walletPresenceMeta{
			"w1": {indexer: presencev1.IndexerEtherscan, lastSeen: start},
		},
		sweepEvery:  15 * time.Millisecond,
		silentAfter: walletPresenceSilentAfter,
		sweepStop:   make(chan struct{}),
		sweepDone:   make(chan struct{}),
	}
	go tr.runWalletSweep()

	deadline := time.Now().Add(2 * time.Second)
	for {
		tr.mu.RLock()
		_, wallet := tr.byType["wallet"]["w1"]
		_, tls := tr.byType["tls"]["t1"]
		_, meta := tr.walletMeta["w1"]
		tr.mu.RUnlock()
		if !wallet && !meta && tls {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("sweep did not remove the silent wallet while keeping tls")
		}
		time.Sleep(5 * time.Millisecond)
	}
	tr.Close()
}

func TestMissingWalletMetadataIsUnknown(t *testing.T) {
	t.Parallel()
	tr := newPresenceTracker(time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC))
	tr.byType["wallet"] = map[string]struct{}{"w1": {}, "w2": {}}
	tr.walletMeta["w1"] = walletPresenceMeta{indexer: presencev1.IndexerEtherscan, lastSeen: tr.clock()}
	if got := tr.OnchainIndexer(); got != presencev1.IndexerUnknown {
		t.Fatalf("indexer = %q, want unknown", got)
	}
}

func TestUnknownScannerTypeIsIgnored(t *testing.T) {
	t.Parallel()
	tr := newPresenceTracker(time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC))
	tr.handleMessage(presenceMsg(t, "joined", "t1", "tls", ""))
	tr.handleMessage(vectorMsg(t, "ok_unknown_type.json"))
	if tr.HasScanner("indexer-gateway") || tr.HasScanner("wallet") {
		t.Fatal("indexer-gateway must not be tracked and must not count as a wallet")
	}
	if !tr.HasScanner("tls") {
		t.Fatal("tls presence must stay")
	}
	for _, info := range tr.ListScanners() {
		if info.Type != "tls" {
			t.Fatalf("listed type %q", info.Type)
		}
	}
	tr.handleMessage(vectorMsg(t, "ok_joined.json"))
	if !tr.HasScanner("wallet") || tr.HasScanner("indexer-gateway") {
		t.Fatal("a wallet joined after an unknown type must be tracked, and indexer-gateway must stay ignored")
	}
}

func TestMalformedPresenceIsLoggedAndConsumerContinues(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	tr := newPresenceTracker(time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC))
	tr.logger = zerolog.New(&buf)
	for _, name := range []string{"bad_event.json", "bad_unreadable.txt", "bad_empty_type.json"} {
		tr.handleMessage(vectorMsg(t, name))
	}
	logged := buf.Bytes()
	if !bytes.Contains(logged, []byte("Invalid scanner presence message")) {
		t.Fatalf("log = %s", logged)
	}
	if bytes.Contains(logged, []byte("scanner-1")) || bytes.Contains(logged, []byte(`"event"`)) {
		t.Fatalf("log contains the message body: %s", logged)
	}
	if tr.HasScanner("wallet") || tr.HasScanner("tls") {
		t.Fatal("malformed messages must not be tracked")
	}
	tr.handleMessage(vectorMsg(t, "ok_joined.json"))
	if !tr.HasScanner("wallet") {
		t.Fatal("consumer stopped after a malformed presence message")
	}
	tr.handleMessage(vectorMsg(t, "ok_left.json"))
	if tr.HasScanner("tls") || !tr.HasScanner("wallet") {
		t.Fatal("a tls left message must not create tls presence or drop the wallet")
	}
}

func newPresenceTracker(now time.Time) *ScannerPresenceTracker {
	frozen := now
	return &ScannerPresenceTracker{
		logger:      zerolog.Nop(),
		byType:      map[string]map[string]struct{}{},
		walletMeta:  map[string]walletPresenceMeta{},
		silentAfter: walletPresenceSilentAfter,
		now:         func() time.Time { return frozen },
	}
}

func presenceMsg(t *testing.T, event, id, kind, indexer string) *natsio.Msg {
	t.Helper()
	body, err := json.Marshal(presencev1.Message{
		Event:          event,
		ScannerID:      id,
		Type:           kind,
		OnchainIndexer: indexer,
	})
	if err != nil {
		t.Fatal(err)
	}
	return &natsio.Msg{Data: body}
}

func vectorMsg(t *testing.T, name string) *natsio.Msg {
	t.Helper()
	body, err := presencev1.Vectors.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return &natsio.Msg{Data: body}
}

type stubRedis struct {
	values map[string]string
}

func (s stubRedis) Get(_ context.Context, key string) *redis.StringCmd {
	if v, ok := s.values[key]; ok {
		return redis.NewStringResult(v, nil)
	}
	return redis.NewStringResult("", redis.Nil)
}

func (s stubRedis) Set(context.Context, string, interface{}, time.Duration) *redis.StatusCmd {
	return redis.NewStatusResult("OK", nil)
}

func (s stubRedis) Del(context.Context, ...string) *redis.IntCmd {
	return redis.NewIntResult(0, nil)
}

func (s stubRedis) Keys(context.Context, string) *redis.StringSliceCmd {
	return redis.NewStringSliceResult(nil, nil)
}

func (s stubRedis) Close() error { return nil }

func (s stubRedis) Ping(context.Context) *redis.StatusCmd {
	return redis.NewStatusResult("PONG", nil)
}

var _ redisconn.Connection = stubRedis{}
