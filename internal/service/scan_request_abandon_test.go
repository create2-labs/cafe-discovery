package service

import (
	"testing"
	"time"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"

	"github.com/google/uuid"
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
