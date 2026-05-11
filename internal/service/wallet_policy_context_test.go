package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"cafe-discovery/internal/domain"

	"github.com/google/uuid"
)

// listWalletScanRepoStub returns wallet rows per owner user id (owner-scoping under test).
type listWalletScanRepoStub struct {
	byUser map[uuid.UUID][]*domain.ScanResultEntity
}

func (s *listWalletScanRepoStub) Create(*domain.ScanResultEntity) error {
	return errors.New("not implemented")
}

func (s *listWalletScanRepoStub) FindByUserID(userID uuid.UUID, limit, offset int) ([]*domain.ScanResultEntity, error) {
	if s.byUser == nil {
		return nil, nil
	}
	rows := s.byUser[userID]
	if offset >= len(rows) {
		return nil, nil
	}
	end := offset + limit
	if limit <= 0 || end > len(rows) {
		end = len(rows)
	}
	out := append([]*domain.ScanResultEntity(nil), rows[offset:end]...)
	return out, nil
}

func (s *listWalletScanRepoStub) FindByID(uuid.UUID) (*domain.ScanResultEntity, error) {
	return nil, nil
}

func (s *listWalletScanRepoStub) FindByUserIDAndAddress(uuid.UUID, string) (*domain.ScanResultEntity, error) {
	return nil, nil
}

func (s *listWalletScanRepoStub) CountByUserID(userID uuid.UUID) (int64, error) {
	if s.byUser == nil {
		return 0, nil
	}
	return int64(len(s.byUser[userID])), nil
}

func TestListWalletPolicyContexts_OwnerIsolation(t *testing.T) {
	t.Parallel()

	userA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	userB := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	scanAID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	repo := &listWalletScanRepoStub{
		byUser: map[uuid.UUID][]*domain.ScanResultEntity{
			userA: {
				{
					ID:        scanAID,
					UserID:    userA,
					Address:   "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Type:      domain.AccountTypeEOA,
					NISTLevel: domain.NISTLevel1,
					Networks:  mustJSONNetworks(t, []string{"ethereum-mainnet"}),
					Status:    "SUCCESS",
					UpdatedAt: time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC),
				},
			},
			userB: {},
		},
	}
	cache := NewUserScanCacheService(repo, &stubTLSScanResultRepository{}, nil, nil)

	ctxsA, totalA, err := cache.ListWalletPolicyContexts(context.Background(), userA, 20, 0)
	if err != nil {
		t.Fatalf("ListWalletPolicyContexts user A: %v", err)
	}
	if totalA != 1 || len(ctxsA) != 1 {
		t.Fatalf("user A: want 1 context, got total=%d len=%d", totalA, len(ctxsA))
	}
	if ctxsA[0].ScanID != scanAID.String() {
		t.Fatalf("scan_id = %q", ctxsA[0].ScanID)
	}
	if ctxsA[0].Status != "completed" {
		t.Fatalf("SUCCESS must normalize to completed, got %q", ctxsA[0].Status)
	}

	ctxsB, totalB, err := cache.ListWalletPolicyContexts(context.Background(), userB, 20, 0)
	if err != nil {
		t.Fatalf("ListWalletPolicyContexts user B: %v", err)
	}
	if totalB != 0 || len(ctxsB) != 0 {
		t.Fatalf("user B must see zero contexts, got total=%d len=%d", totalB, len(ctxsB))
	}
}

func TestListWalletPolicyContexts_ChainIDsUnknownNetworkEmpty(t *testing.T) {
	t.Parallel()

	user := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	scanID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	repo := &listWalletScanRepoStub{
		byUser: map[uuid.UUID][]*domain.ScanResultEntity{
			user: {{
				ID:        scanID,
				UserID:    user,
				Address:   "0xcccccccccccccccccccccccccccccccccccccccc",
				Type:      domain.AccountTypeEOA,
				NISTLevel: domain.NISTLevel1,
				Networks:  mustJSONNetworks(t, []string{"unknown-future-network"}),
				Status:    "SUCCESS",
				UpdatedAt: time.Now().UTC(),
			}},
		},
	}
	cache := NewUserScanCacheService(repo, &stubTLSScanResultRepository{}, nil, nil)

	ctxs, _, err := cache.ListWalletPolicyContexts(context.Background(), user, 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctxs) != 1 {
		t.Fatalf("want 1 context, got %d", len(ctxs))
	}
	if len(ctxs[0].ChainIDs) != 0 {
		t.Fatalf("unknown networks must yield empty chain_ids, got %#v", ctxs[0].ChainIDs)
	}
}

func mustJSONNetworks(t *testing.T, nets []string) string {
	t.Helper()
	b, err := json.Marshal(nets)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
