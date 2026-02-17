package service

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"cafe-discovery/pkg/nats"
	redisconn "cafe-discovery/pkg/redis"
	natsio "github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
)

const scannerHeartbeatTTL = 20 * time.Second // Consider DOWN if no heartbeat for >15s; 20s TTL

// ScannerPresenceTracker subscribes to scanner presence and heartbeat messages.
// Backend considers a scanner UP if a heartbeat was received within the last 15s (Redis key TTL 20s).
type ScannerPresenceTracker struct {
	conn    nats.Connection
	redis   redisconn.Connection
	mu      sync.RWMutex
	byType  map[string]map[string]struct{}
	sub     *natsio.Subscription
	subTLS  *natsio.Subscription
	subWallet *natsio.Subscription
}

// NewScannerPresenceTracker creates a tracker, subscribes to presence and heartbeat subjects.
// If redis is non-nil, heartbeats are stored in Redis and HasScanner uses Redis (last_seen within TTL).
func NewScannerPresenceTracker(conn nats.Connection, redis redisconn.Connection) (*ScannerPresenceTracker, error) {
	t := &ScannerPresenceTracker{
		conn:   conn,
		redis:  redis,
		byType: make(map[string]map[string]struct{}),
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
	case nats.ScannerPresenceLeft:
		delete(t.byType[presence.Type], presence.ScannerID)
	}
}

// HasScanner returns true if a scanner of the given type is considered UP.
// When Redis is configured: true if scanner:<type>:last_seen exists (heartbeat within TTL).
// Otherwise: true if at least one scanner has announced (joined) and not yet left.
func (t *ScannerPresenceTracker) HasScanner(scannerType string) bool {
	if t.redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		key := "scanner:" + scannerType + ":last_seen"
		_, err := t.redis.Get(ctx, key).Result()
		if err == nil {
			return true
		}
		return false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.byType[scannerType]) > 0
}

// ScannerInfo holds the type and count (and optionally IDs) of available scanners.
type ScannerInfo struct {
	Type  string   `json:"type"`  // "tls" or "wallet"
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

// Close unsubscribes from presence and heartbeat subjects.
func (t *ScannerPresenceTracker) Close() error {
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
