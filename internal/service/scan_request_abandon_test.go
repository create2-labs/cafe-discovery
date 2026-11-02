package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/metrics"
	"cafe-discovery/internal/persistence/scanpending"
	"cafe-discovery/internal/repository"
	"cafe-discovery/pkg/nats"

	lifecyclev1 "github.com/create2-labs/cafe-contracts/discovery/scanlifecycle/v1"
	"github.com/google/uuid"
	natsio "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func TestAbandoner_QueuedScanStaysReserved(t *testing.T) {
	a := NewScanRequestAbandoner(time.Minute)
	id := uuid.New()
	open := []repository.OpenScanReservation{{
		ScanID: id, UserID: uuid.New(), ScanKind: domain.ScanUsageKindWallet,
	}}
	queued := map[uuid.UUID]struct{}{id: {}}
	now := time.Now()
	if got := a.Due(now, open, queued); len(got) != 0 {
		t.Fatalf("due = %d, want 0", len(got))
	}
	if got := a.Due(now.Add(2*time.Minute), open, queued); len(got) != 0 {
		t.Fatalf("still queued due = %d, want 0", len(got))
	}
}

func TestAbandoner_MissingMessageWaitsThenFails(t *testing.T) {
	a := NewScanRequestAbandoner(time.Minute)
	id := uuid.New()
	open := []repository.OpenScanReservation{{
		ScanID: id, UserID: uuid.New(), ScanKind: domain.ScanUsageKindEndpoint,
	}}
	now := time.Now()
	if got := a.Due(now, open, nil); len(got) != 0 {
		t.Fatalf("first observation due = %d, want 0", len(got))
	}
	if got := a.Due(now.Add(30*time.Second), open, nil); len(got) != 0 {
		t.Fatalf("inside grace due = %d, want 0", len(got))
	}
	got := a.Due(now.Add(time.Minute), open, nil)
	if len(got) != 1 || got[0].ScanID != id {
		t.Fatalf("due = %+v", got)
	}
	a.MarkSent(id)
	if again := a.Due(now.Add(2*time.Minute), open, nil); len(again) != 0 {
		t.Fatalf("second report due = %d, want 0", len(again))
	}
}

func TestAbandoner_TerminalReservationDropsTheWatch(t *testing.T) {
	a := NewScanRequestAbandoner(time.Second)
	id := uuid.New()
	open := []repository.OpenScanReservation{{ScanID: id, ScanKind: domain.ScanUsageKindWallet}}
	now := time.Now()
	_ = a.Due(now, open, nil)
	if got := a.Due(now.Add(time.Second), nil, nil); len(got) != 0 {
		t.Fatalf("completed scan due = %d, want 0", len(got))
	}
	if got := a.Due(now.Add(2*time.Second), open, nil); len(got) != 0 {
		t.Fatalf("reopened watch due = %d, want 0 before a new grace", len(got))
	}
}

func TestAbandoner_CompletedScanThatLeftTheQueueIsNotFailed(t *testing.T) {
	a := NewScanRequestAbandoner(time.Millisecond)
	id := uuid.New()
	now := time.Now()
	if got := a.Due(now, nil, map[uuid.UUID]struct{}{}); len(got) != 0 {
		t.Fatalf("no open reservation due = %d", len(got))
	}
	if _, ok := a.missingSince[id]; ok {
		t.Fatal("watched a scan that already has a result")
	}
}

const abandonAddr = "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
const abandonAddrNorm = "0x742d35cc6634c0532925a3b844bc454e4438f44e"

type abandonOpenLedger struct {
	rows []repository.OpenScanReservation
}

func (l abandonOpenLedger) ListOpenScanReservations() ([]repository.OpenScanReservation, error) {
	return l.rows, nil
}

type abandonPending struct {
	rec    *scanpending.Record
	err    error
	gets   int
	userID uuid.UUID
	scanID uuid.UUID
	tenant string
}

func (p *abandonPending) Get(_ context.Context, userID uuid.UUID, tenantID string, scanID uuid.UUID) (*scanpending.Record, error) {
	p.gets++
	p.userID, p.tenant, p.scanID = userID, tenantID, scanID
	if p.err != nil {
		return nil, p.err
	}
	return p.rec, nil
}

func (p *abandonPending) PutTLS(context.Context, uuid.UUID, string, *scanpending.Record) error {
	return nil
}
func (p *abandonPending) ReserveWallet(context.Context, uuid.UUID, string, *scanpending.Record) (bool, error) {
	return false, nil
}
func (p *abandonPending) GetWalletByOwnerAddress(context.Context, uuid.UUID, string, string) (*scanpending.Record, error) {
	return nil, nil
}
func (p *abandonPending) Delete(context.Context, uuid.UUID, string, uuid.UUID) error { return nil }
func (p *abandonPending) DeleteWalletReservation(context.Context, uuid.UUID, string, string, uuid.UUID) error {
	return nil
}

type abandonConn struct {
	core     [][]byte
	jet      [][]byte
	jetFailN int
	jetCalls int
}

func (c *abandonConn) Publish(subject string, data []byte) error {
	if subject != lifecyclev1.SubjectScanFailed {
		return errors.New("unexpected subject")
	}
	c.core = append(c.core, append([]byte(nil), data...))
	return nil
}

func (c *abandonConn) PublishJetStream(subject string, data []byte) error {
	c.jetCalls++
	if c.jetCalls <= c.jetFailN {
		return errors.New("jetstream ack missing")
	}
	if subject != lifecyclev1.SubjectScanFailed {
		return errors.New("unexpected subject")
	}
	c.jet = append(c.jet, append([]byte(nil), data...))
	return nil
}

func (c *abandonConn) ListStreamPayloads(context.Context) ([][]byte, error) { return nil, nil }
func (c *abandonConn) ConsumeWalletLifecycle(context.Context, func(jetstream.Msg)) error {
	return nil
}
func (c *abandonConn) Subscribe(string, func(*natsio.Msg)) (*natsio.Subscription, error) {
	return nil, nil
}
func (c *abandonConn) QueueSubscribe(string, string, func(*natsio.Msg)) (*natsio.Subscription, error) {
	return nil, nil
}
func (c *abandonConn) Close()            {}
func (c *abandonConn) IsConnected() bool { return true }

func dueAbandoner(id uuid.UUID) *ScanRequestAbandoner {
	a := NewScanRequestAbandoner(time.Minute)
	a.missingSince[id] = time.Now().Add(-time.Hour)
	return a
}

func abandonRow(id, user uuid.UUID, kind domain.ScanUsageKind) repository.OpenScanReservation {
	return repository.OpenScanReservation{ScanID: id, UserID: user, ScanKind: kind}
}

func publishDue(t *testing.T, a *ScanRequestAbandoner, conn nats.Connection, row repository.OpenScanReservation, pending scanpending.Store) {
	t.Helper()
	err := a.publishMissing(context.Background(), conn, abandonOpenLedger{rows: []repository.OpenScanReservation{row}}, pending)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAbandonWallet_PendingAddressAcknowledgedOnce(t *testing.T) {
	id, user := uuid.New(), uuid.New()
	row := abandonRow(id, user, domain.ScanUsageKindWallet)
	pending := &abandonPending{rec: &scanpending.Record{ScanID: id, UserID: user, Address: abandonAddr}}
	conn := &abandonConn{}
	a := dueAbandoner(id)
	publishDue(t, a, conn, row, pending)
	publishDue(t, a, conn, row, pending)
	if pending.gets != 1 || pending.userID != user || pending.scanID != id || pending.tenant != "" {
		t.Fatalf("gets=%d user=%s scan=%s tenant=%q", pending.gets, pending.userID, pending.scanID, pending.tenant)
	}
	if len(conn.core) != 0 || len(conn.jet) != 1 {
		t.Fatalf("core=%d jet=%d", len(conn.core), len(conn.jet))
	}
	msg, err := lifecyclev1.DecodeFailed(conn.jet[0])
	if err != nil || msg.Address != abandonAddrNorm || msg.Kind != "wallet" || msg.Error != scanRequestLeftQueue {
		t.Fatalf("event %+v err %v", msg, err)
	}
}

func TestAbandonWallet_MissingAckDoesNotMark(t *testing.T) {
	id, user := uuid.New(), uuid.New()
	row := abandonRow(id, user, domain.ScanUsageKindWallet)
	pending := &abandonPending{rec: &scanpending.Record{Address: abandonAddr}}
	conn := &abandonConn{jetFailN: 1}
	a := dueAbandoner(id)
	err := a.publishMissing(context.Background(), conn, abandonOpenLedger{rows: []repository.OpenScanReservation{row}}, pending)
	if err == nil || len(conn.jet) != 0 {
		t.Fatalf("err=%v jet=%d", err, len(conn.jet))
	}
	publishDue(t, a, conn, row, pending)
	if len(conn.jet) != 1 {
		t.Fatalf("retry jet=%d", len(conn.jet))
	}
	msg, err := lifecyclev1.DecodeFailed(conn.jet[0])
	if err != nil || msg.Address != abandonAddrNorm {
		t.Fatalf("event %+v err %v", msg, err)
	}
}

func TestAbandonWallet_MissingAddressRetriesNextPass(t *testing.T) {
	metrics.Get()
	id, user := uuid.New(), uuid.New()
	row := abandonRow(id, user, domain.ScanUsageKindWallet)
	pending := &abandonPending{}
	conn := &abandonConn{}
	a := dueAbandoner(id)
	before := lifecycleValue(t, walletLifecycleInvalid)
	publishDue(t, a, conn, row, pending)
	if pending.gets != 1 || len(conn.core) != 0 || len(conn.jet) != 0 || lifecycleValue(t, walletLifecycleInvalid) != before+1 {
		t.Fatalf("gets=%d core=%d jet=%d", pending.gets, len(conn.core), len(conn.jet))
	}
	pending.rec = &scanpending.Record{Address: abandonAddr}
	publishDue(t, a, conn, row, pending)
	if len(conn.jet) != 1 {
		t.Fatalf("retry jet=%d", len(conn.jet))
	}
	msg, err := lifecyclev1.DecodeFailed(conn.jet[0])
	if err != nil || msg.Address != abandonAddrNorm {
		t.Fatalf("event %+v err %v", msg, err)
	}
}

func TestAbandonWallet_UnusableAddressDoesNotPublish(t *testing.T) {
	metrics.Get()
	id, user := uuid.New(), uuid.New()
	row := abandonRow(id, user, domain.ScanUsageKindWallet)
	cases := []*abandonPending{
		{rec: &scanpending.Record{Address: ""}},
		{rec: &scanpending.Record{Address: "0x1234"}},
		{err: errors.New("pending unavailable")},
	}
	for _, pending := range cases {
		conn := &abandonConn{}
		a := dueAbandoner(id)
		before := lifecycleValue(t, walletLifecycleInvalid)
		publishDue(t, a, conn, row, pending)
		publishDue(t, a, conn, row, pending)
		if len(conn.core) != 0 || len(conn.jet) != 0 || pending.gets != 2 || lifecycleValue(t, walletLifecycleInvalid) != before+2 {
			t.Fatalf("gets=%d core=%d jet=%d metric", pending.gets, len(conn.core), len(conn.jet))
		}
	}
}

func TestAbandonTLS_CorePublishWithoutPendingRead(t *testing.T) {
	id, user := uuid.New(), uuid.New()
	row := abandonRow(id, user, domain.ScanUsageKindEndpoint)
	pending := &abandonPending{rec: &scanpending.Record{Address: abandonAddr}}
	conn := &abandonConn{}
	a := dueAbandoner(id)
	publishDue(t, a, conn, row, pending)
	publishDue(t, a, conn, row, pending)
	if pending.gets != 0 || len(conn.core) != 1 || len(conn.jet) != 0 {
		t.Fatalf("gets=%d core=%d jet=%d", pending.gets, len(conn.core), len(conn.jet))
	}
	msg, err := lifecyclev1.DecodeFailed(conn.core[0])
	if err != nil || msg.Kind != "tls" || msg.Address != "" || msg.Error != scanRequestLeftQueue {
		t.Fatalf("event %+v err %v", msg, err)
	}
}
