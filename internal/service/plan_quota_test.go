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

var (
	_ repository.UserRepository       = (*planQuotaUserRepo)(nil)
	_ repository.PlanRepository       = (*planQuotaPlanRepo)(nil)
)

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
