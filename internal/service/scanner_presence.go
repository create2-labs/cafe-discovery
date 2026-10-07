package service

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"cafe-discovery/pkg/nats"
	redisconn "cafe-discovery/pkg/redis"
	natsio "github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
)

const scannerHeartbeatTTL = 20 * time.Second // Consider DOWN if no heartbeat for >15s; 20s TTL

const (
	// OnchainIndexerEtherscan is announced when the wallet scanner selected Etherscan.
	OnchainIndexerEtherscan = "etherscan"
	// OnchainIndexerMoralis is announced when the wallet scanner selected Moralis.
	OnchainIndexerMoralis = "moralis"
	// OnchainIndexerNone is announced when the wallet scanner has no indexer client.
	OnchainIndexerNone = "none"
	// OnchainIndexerUnknown means Discovery cannot name one indexer for the living wallet scanners.
	OnchainIndexerUnknown = "unknown"

	walletPresenceSweepInterval = 15 * time.Second
	walletPresenceSilentAfter   = 75 * time.Second
)

// ScannerPresenceTracker subscribes to scanner presence and heartbeat messages.
// Backend considers a scanner UP if a heartbeat was received within the last 15s (Redis key TTL 20s).
// Wallet joined messages also keep an in-memory indexer and last_seen, separate from the Redis key scanner:wallet:last_seen.
type ScannerPresenceTracker struct {
	conn   nats.Connection
	redis  redisconn.Connection
	mu     sync.RWMutex
	byType map[string]map[string]struct{}
	// walletMeta is scanner_id -> indexer and the last wallet joined time. TLS presence is not stored here.
	walletMeta    map[string]walletPresenceMeta
	sub           *natsio.Subscription
	subTLS        *natsio.Subscription
	subWallet     *natsio.Subscription
	sweepEvery    time.Duration
	silentAfter   time.Duration
	sweepStop     chan struct{}
	sweepDone     chan struct{}
	stopSweepOnce sync.Once
	now           func() time.Time
}

type walletPresenceMeta struct {
	indexer  string
	lastSeen time.Time
}

// NewScannerPresenceTracker creates a tracker, subscribes to presence and heartbeat subjects.
// If redis is non-nil, heartbeats are stored in Redis and HasScanner uses Redis (last_seen within TTL).
func NewScannerPresenceTracker(conn nats.Connection, redis redisconn.Connection) (*ScannerPresenceTracker, error) {
	t := &ScannerPresenceTracker{
		conn:        conn,
		redis:       redis,
		byType:      make(map[string]map[string]struct{}),
		walletMeta:  make(map[string]walletPresenceMeta),
		sweepEvery:  walletPresenceSweepInterval,
		silentAfter: walletPresenceSilentAfter,
		sweepStop:   make(chan struct{}),
		sweepDone:   make(chan struct{}),
	}
	sub, err := conn.Subscribe(nats.SubjectScannerPresence, t.handlePresence)
	if err != nil {
		return nil, err
	}
	t.sub = sub
	if redis != nil {
		subTLS, err := conn.Subscribe(nats.SubjectScannerHeartbeatTLS, t.handleHeartbeat("tls"))
		if err != nil {
			_ = sub.Unsubscribe()
			return nil, err
		}
		t.subTLS = subTLS
		subWallet, err := conn.Subscribe(nats.SubjectScannerHeartbeatWallet, t.handleHeartbeat("wallet"))
		if err != nil {
			_ = subTLS.Unsubscribe()
			_ = sub.Unsubscribe()
			return nil, err
		}
		t.subWallet = subWallet
		log.Info().Str("heartbeat_tls", nats.SubjectScannerHeartbeatTLS).Str("heartbeat_wallet", nats.SubjectScannerHeartbeatWallet).Msg("Scanner heartbeat tracker subscribed")
	}
	go t.runWalletSweep()
	log.Info().Str("subject", nats.SubjectScannerPresence).Msg("Scanner presence tracker subscribed")
	return t, nil
}

func (t *ScannerPresenceTracker) handleHeartbeat(kind string) func(*natsio.Msg) {
	return func(msg *natsio.Msg) {
		var h nats.ScannerHeartbeatMessage
		if err := json.Unmarshal(msg.Data, &h); err != nil || h.Kind == "" {
			return
		}
		if t.redis == nil {
			return
		}
		key := "scanner:" + kind + ":last_seen"
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = t.redis.Set(ctx, key, h.Timestamp, scannerHeartbeatTTL).Err()
	}
}

func (t *ScannerPresenceTracker) handlePresence(msg *natsio.Msg) {
	t.handleMessage(msg)
}

func (t *ScannerPresenceTracker) handleMessage(msg *natsio.Msg) {
	var presence nats.ScannerPresenceMessage
	if err := json.Unmarshal(msg.Data, &presence); err != nil {
		log.Warn().Err(err).Msg("Invalid scanner presence message")
		return
	}
	if presence.Type == "" || presence.ScannerID == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.byType[presence.Type] == nil {
		t.byType[presence.Type] = make(map[string]struct{})
	}
	switch presence.Event {
	case nats.ScannerPresenceJoined:
		t.byType[presence.Type][presence.ScannerID] = struct{}{}
		if presence.Type == "wallet" {
			t.walletMeta[presence.ScannerID] = walletPresenceMeta{
				indexer:  normalizeOnchainIndexer(presence.OnchainIndexer),
				lastSeen: t.clock(),
			}
		}
	case nats.ScannerPresenceLeft:
		delete(t.byType[presence.Type], presence.ScannerID)
		if presence.Type == "wallet" {
			delete(t.walletMeta, presence.ScannerID)
		}
	}
}

func normalizeOnchainIndexer(raw string) string {
	switch strings.TrimSpace(raw) {
	case OnchainIndexerEtherscan, OnchainIndexerMoralis, OnchainIndexerNone:
		return strings.TrimSpace(raw)
	default:
		return OnchainIndexerUnknown
	}
}

func (t *ScannerPresenceTracker) clock() time.Time {
	if t.now != nil {
		return t.now()
	}
	return time.Now().UTC()
}

// expireSilentWallets removes wallet instances whose last joined is older than the silence window.
// TLS presence is left unchanged.
func (t *ScannerPresenceTracker) expireSilentWallets(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	limit := t.silentAfter
	if limit <= 0 {
		limit = walletPresenceSilentAfter
	}
	for id, meta := range t.walletMeta {
		if now.Sub(meta.lastSeen) <= limit {
			continue
		}
		delete(t.walletMeta, id)
		if wallets := t.byType["wallet"]; wallets != nil {
			delete(wallets, id)
		}
	}
}

func (t *ScannerPresenceTracker) runWalletSweep() {
	defer close(t.sweepDone)
	every := t.sweepEvery
	if every <= 0 {
		every = walletPresenceSweepInterval
	}
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-t.sweepStop:
			return
		case now := <-ticker.C:
			t.expireSilentWallets(now)
		}
	}
}

// HasScanner returns true if a scanner of the given type is considered UP.
// When Redis is configured: true if scanner:<type>:last_seen exists (heartbeat within TTL),
// or if at least one scanner has announced (joined) and not yet left (in-memory presence).
// Otherwise: true if at least one scanner has announced (joined) and not yet left.
// This keeps HasScanner in sync with ListScanners so GET /discovery/v1/scanners and scan requests use the same notion of availability.
func (t *ScannerPresenceTracker) HasScanner(scannerType string) bool {
	if t.redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		key := "scanner:" + scannerType + ":last_seen"
		_, err := t.redis.Get(ctx, key).Result()
		if err == nil {
			return true
		}
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.byType[scannerType]) > 0
}

// OnchainIndexer returns the indexer shared by every living wallet instance.
// A Redis heartbeat alone, a missing announcement, or disagreeing instances yield unknown.
// An absent onchain_indexer field is unknown, never none.
func (t *ScannerPresenceTracker) OnchainIndexer() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	ids := t.byType["wallet"]
	if len(ids) == 0 {
		return OnchainIndexerUnknown
	}
	var common string
	seen := false
	for id := range ids {
		meta, ok := t.walletMeta[id]
		if !ok || meta.indexer == "" {
			return OnchainIndexerUnknown
		}
		if !seen {
			common = meta.indexer
			seen = true
			continue
		}
		if meta.indexer != common {
			return OnchainIndexerUnknown
		}
	}
	if !seen {
		return OnchainIndexerUnknown
	}
	return common
}

// ScannerInfo holds the type and count (and optionally IDs) of available scanners.
type ScannerInfo struct {
	Type  string   `json:"type"` // "tls" or "wallet"
	Count int      `json:"count"`
	IDs   []string `json:"ids,omitempty"` // scanner IDs (optional, for debugging/ops)
}

// ListScanners returns a snapshot of currently available scanner types with their counts.
func (t *ScannerPresenceTracker) ListScanners() []ScannerInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]ScannerInfo, 0, len(t.byType))
	for typ, ids := range t.byType {
		if len(ids) == 0 {
			continue
		}
		scannerIDs := make([]string, 0, len(ids))
		for id := range ids {
			scannerIDs = append(scannerIDs, id)
		}
		out = append(out, ScannerInfo{Type: typ, Count: len(scannerIDs), IDs: scannerIDs})
	}
	return out
}

// Close stops the wallet silence sweep and unsubscribes from presence and heartbeat subjects.
func (t *ScannerPresenceTracker) Close() error {
	t.stopSweep()
	if t.subTLS != nil {
		_ = t.subTLS.Unsubscribe()
	}
	if t.subWallet != nil {
		_ = t.subWallet.Unsubscribe()
	}
	if t.sub != nil {
		return t.sub.Unsubscribe()
	}
	return nil
}

func (t *ScannerPresenceTracker) stopSweep() {
	if t.sweepStop == nil {
		return
	}
	t.stopSweepOnce.Do(func() {
		close(t.sweepStop)
	})
	if t.sweepDone != nil {
		<-t.sweepDone
	}
}
