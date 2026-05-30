package service

import (
	"testing"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"

	"github.com/google/uuid"
)

type planQuotaUserRepo struct {
	user *domain.User
}

func (r *planQuotaUserRepo) Create(*domain.User) error { return nil }
func (r *planQuotaUserRepo) FindByID(string) (*domain.User, error) {
	return r.user, nil
}
func (r *planQuotaUserRepo) FindByEmail(string) (*domain.User, error) { return nil, nil }
func (r *planQuotaUserRepo) ExistsByEmail(string) (bool, error)      { return false, nil }

type planQuotaPlanRepo struct {
	plan *domain.Plan
}

func (r *planQuotaPlanRepo) Create(*domain.Plan) error { return nil }
func (r *planQuotaPlanRepo) FindByID(uuid.UUID) (*domain.Plan, error) {
	return r.plan, nil
}
func (r *planQuotaPlanRepo) FindByType(domain.PlanType) (*domain.Plan, error) { return nil, nil }
func (r *planQuotaPlanRepo) FindAll() ([]*domain.Plan, error)                 { return nil, nil }
func (r *planQuotaPlanRepo) FindActive() ([]*domain.Plan, error)              { return nil, nil }

// walletScanRepoStub implements ScanResultRepository for quota tests (only CountByUserID is used).
type walletScanRepoStub struct {
	count int64
}

func (r *walletScanRepoStub) CountByUserID(uuid.UUID) (int64, error) {
	return r.count, nil
}

func (*walletScanRepoStub) Create(*domain.ScanResultEntity) error { return nil }
func (*walletScanRepoStub) FindByUserID(uuid.UUID, int, int) ([]*domain.ScanResultEntity, error) {
	return nil, nil
}
func (*walletScanRepoStub) FindByID(uuid.UUID) (*domain.ScanResultEntity, error) { return nil, nil }
func (*walletScanRepoStub) FindOwnedWalletScanByID(uuid.UUID, uuid.UUID) (*domain.ScanResultEntity, error) {
	return nil, nil
}
func (*walletScanRepoStub) DeleteOwnedWalletScan(uuid.UUID, uuid.UUID) (bool, error) { return false, nil }
func (*walletScanRepoStub) ListOwnerWalletScansDiscoveryV1(uuid.UUID, string, int, int) ([]*domain.ScanResultEntity, int64, error) {
	return nil, 0, nil
}
func (*walletScanRepoStub) ListOwnerWalletScansByAddress(uuid.UUID, string) ([]*domain.ScanResultEntity, error) {
	return nil, nil
}

var (
	_ repository.ScanResultRepository = (*walletScanRepoStub)(nil)
	_ repository.UserRepository       = (*planQuotaUserRepo)(nil)
	_ repository.PlanRepository       = (*planQuotaPlanRepo)(nil)
)

func TestCheckScanLimit_WalletCountsExecutionsNotUniqueAddresses(t *testing.T) {
	t.Parallel()

	planID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	plan := &domain.Plan{
		ID:              planID,
		WalletScanLimit: 2,
	}
	user := &domain.User{ID: userID, PlanID: planID}
	svc := NewPlanService(&planQuotaPlanRepo{plan: plan}, &planQuotaUserRepo{user: user})

	t.Run("one execution under limit", func(t *testing.T) {
		repo := &walletScanRepoStub{count: 1}
		canScan, usage, err := svc.CheckScanLimit(userID, "wallet", repo, nil)
		if err != nil {
			t.Fatalf("CheckScanLimit: %v", err)
		}
		if !canScan {
			t.Fatal("expected canScan true with 1/2 executions")
		}
		if usage.WalletScansUsed != 1 {
			t.Fatalf("WalletScansUsed = %d, want 1", usage.WalletScansUsed)
		}
	})

	t.Run("two executions same address at limit", func(t *testing.T) {
		repo := &walletScanRepoStub{count: 2}
		canScan, usage, err := svc.CheckScanLimit(userID, "wallet", repo, nil)
		if err != nil {
			t.Fatalf("CheckScanLimit: %v", err)
		}
		if canScan {
			t.Fatal("expected canScan false at 2/2 executions")
		}
		if usage.WalletScansUsed != 2 {
			t.Fatalf("WalletScansUsed = %d, want 2", usage.WalletScansUsed)
		}
	})
}

func TestGetPlanUsage_WalletExecutionCount(t *testing.T) {
	t.Parallel()

	planID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	plan := &domain.Plan{ID: planID, WalletScanLimit: 5}
	user := &domain.User{ID: userID, PlanID: planID}
	svc := NewPlanService(&planQuotaPlanRepo{plan: plan}, &planQuotaUserRepo{user: user})

	ledger := &usageAPILedgerStub{walletUsed: 2, walletVisible: 2}
	usage, err := svc.GetPlanUsage(userID, ledger)
	if err != nil {
		t.Fatalf("GetPlanUsage: %v", err)
	}
	if usage.WalletScansUsed != 2 {
		t.Fatalf("WalletScansUsed = %d, want 2 (ledger success count)", usage.WalletScansUsed)
	}
	if usage.WalletScansLeft != 3 {
		t.Fatalf("WalletScansLeft = %d, want 3", usage.WalletScansLeft)
	}
}
