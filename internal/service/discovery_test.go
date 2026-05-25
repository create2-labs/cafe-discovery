package service

import (
	"context"
	"errors"
	"testing"

	"cafe-discovery/internal/domain"

	"github.com/google/uuid"
)

type recordingWalletScanRepository struct {
	created []*domain.ScanResultEntity
}

func (r *recordingWalletScanRepository) Create(entity *domain.ScanResultEntity) error {
	r.created = append(r.created, entity)
	return nil
}

func (r *recordingWalletScanRepository) FindByUserID(uuid.UUID, int, int) ([]*domain.ScanResultEntity, error) {
	return nil, errors.New("not implemented")
}

func (r *recordingWalletScanRepository) FindByID(uuid.UUID) (*domain.ScanResultEntity, error) {
	return nil, errors.New("not implemented")
}

func (r *recordingWalletScanRepository) FindOwnedWalletScanByID(uuid.UUID, uuid.UUID) (*domain.ScanResultEntity, error) {
	return nil, errors.New("not implemented")
}

func (r *recordingWalletScanRepository) ListOwnerWalletScansDiscoveryV1(uuid.UUID, string, int, int) ([]*domain.ScanResultEntity, int64, error) {
	return nil, 0, errors.New("not implemented")
}

func (r *recordingWalletScanRepository) ListOwnerWalletScansByAddress(uuid.UUID, string) ([]*domain.ScanResultEntity, error) {
	return nil, errors.New("not implemented")
}

func (r *recordingWalletScanRepository) CountByUserID(uuid.UUID) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *recordingWalletScanRepository) DeleteOwnedWalletScan(uuid.UUID, uuid.UUID) (bool, error) {
	return false, errors.New("not implemented")
}

func TestScanWalletPersistsEachExecutionForSameAddress(t *testing.T) {
	repo := &recordingWalletScanRepository{}
	svc := NewDiscoveryService(nil, nil, repo, nil)
	userID := uuid.New()
	address := "0x70af6fea3df8a81fa71e5e5abc2989f6880cfa21"

	if _, err := svc.ScanWallet(context.Background(), userID, address, false); err != nil {
		t.Fatalf("first scan failed: %v", err)
	}
	if _, err := svc.ScanWallet(context.Background(), userID, address, false); err != nil {
		t.Fatalf("second scan failed: %v", err)
	}

	if got := len(repo.created); got != 2 {
		t.Fatalf("expected one persisted row per execution, got %d", got)
	}
	for i, entity := range repo.created {
		if entity.UserID != userID {
			t.Fatalf("created[%d] user_id = %s, want %s", i, entity.UserID, userID)
		}
		if entity.Address != address {
			t.Fatalf("created[%d] address = %s, want %s", i, entity.Address, address)
		}
	}
}
