package service

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"
	"cafe-discovery/pkg/nats"

	"github.com/google/uuid"
)

const (
	// scanRequestMissingGrace is how long a reserved scan may be absent from the
	// work queue before Discovery publishes scan.failed. The wait covers the gap
	// between the scanner ack and the persistence write.
	scanRequestMissingGrace = 2 * time.Minute
	scanRequestAbandonEvery = 15 * time.Second

	scanRequestLeftQueue = "scan request left the queue without a result"
)

// ScanRequestAbandoner decides when a reserved scan has left the work queue
// without a terminal result, so the credit can be returned via scan.failed.
type ScanRequestAbandoner struct {
	Grace        time.Duration
	missingSince map[uuid.UUID]time.Time
	sent         map[uuid.UUID]struct{}
}

func NewScanRequestAbandoner(grace time.Duration) *ScanRequestAbandoner {
	if grace <= 0 {
		grace = scanRequestMissingGrace
	}
	return &ScanRequestAbandoner{
		Grace:        grace,
		missingSince: map[uuid.UUID]time.Time{},
		sent:         map[uuid.UUID]struct{}{},
	}
}

// Due returns reservations that have been missing from the queue for Grace.
// A scan still in the queue, or one already reported, is not returned.
func (a *ScanRequestAbandoner) Due(now time.Time, open []repository.OpenScanReservation, queued map[uuid.UUID]struct{}) []repository.OpenScanReservation {
	seen := make(map[uuid.UUID]struct{}, len(open))
	var due []repository.OpenScanReservation
	for _, row := range open {
		seen[row.ScanID] = struct{}{}
		if _, ok := a.sent[row.ScanID]; ok {
			continue
		}
		if _, ok := queued[row.ScanID]; ok {
			delete(a.missingSince, row.ScanID)
			continue
		}
		first, ok := a.missingSince[row.ScanID]
		if !ok {
			a.missingSince[row.ScanID] = now
			continue
		}
		if now.Sub(first) >= a.Grace {
			due = append(due, row)
		}
	}
	for id := range a.missingSince {
		if _, ok := seen[id]; !ok {
			delete(a.missingSince, id)
			delete(a.sent, id)
		}
	}
	return due
}

// MarkSent records that scan.failed was published for this scan.
func (a *ScanRequestAbandoner) MarkSent(id uuid.UUID) {
	a.sent[id] = struct{}{}
}

// RunScanRequestAbandoner publishes scan.failed for reservations whose message
// left the work queue and never produced a terminal result.
func RunScanRequestAbandoner(ctx context.Context, conn nats.Connection, ledger repository.ScanUsageLedgerRepository) {
	a := NewScanRequestAbandoner(scanRequestMissingGrace)
	ticker := time.NewTicker(scanRequestAbandonEvery)
	defer ticker.Stop()
	for {
		if err := a.publishMissing(ctx, conn, ledger); err != nil {
			log.Printf("scan request abandon: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *ScanRequestAbandoner) publishMissing(ctx context.Context, conn nats.Connection, ledger repository.ScanUsageLedgerRepository) error {
	payloads, err := conn.ListStreamPayloads(ctx)
	if err != nil {
		return err
	}
	open, err := ledger.ListOpenScanReservations()
	if err != nil {
		return err
	}
	for _, row := range a.Due(time.Now(), open, scanIDsFromPayloads(payloads)) {
		if err := publishAbandonedScan(conn, row); err != nil {
			return err
		}
		a.MarkSent(row.ScanID)
	}
	return nil
}

func scanIDsFromPayloads(payloads [][]byte) map[uuid.UUID]struct{} {
	out := make(map[uuid.UUID]struct{}, len(payloads))
	for _, payload := range payloads {
		var body struct {
			ScanID uuid.UUID `json:"scan_id"`
		}
		if json.Unmarshal(payload, &body) != nil || body.ScanID == uuid.Nil {
			continue
		}
		out[body.ScanID] = struct{}{}
	}
	return out
}

func publishAbandonedScan(conn nats.Connection, row repository.OpenScanReservation) error {
	kind := "wallet"
	if row.ScanKind == domain.ScanUsageKindEndpoint {
		kind = "tls"
	}
	return nats.PublishJSON(conn, nats.SubjectScanFailed, nats.ScanFailedMessage{
		ScanID:      row.ScanID,
		Kind:        kind,
		UserID:      row.UserID,
		Error:       scanRequestLeftQueue,
		CompletedAt: time.Now().UTC().Format(time.RFC3339),
	})
}
