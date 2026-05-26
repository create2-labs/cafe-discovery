package handler

import (
	"testing"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"
	"cafe-discovery/internal/service"

	"github.com/google/uuid"
)

type handlerPlanUserRepo struct {
	user *domain.User
}

func (r *handlerPlanUserRepo) Create(*domain.User) error { return nil }
func (r *handlerPlanUserRepo) FindByID(string) (*domain.User, error) {
	return r.user, nil
}
func (r *handlerPlanUserRepo) FindByEmail(string) (*domain.User, error) { return nil, nil }
func (r *handlerPlanUserRepo) ExistsByEmail(string) (bool, error)      { return false, nil }

type handlerPlanPlanRepo struct {
	plan *domain.Plan
}

func (r *handlerPlanPlanRepo) Create(*domain.Plan) error { return nil }
func (r *handlerPlanPlanRepo) FindByID(uuid.UUID) (*domain.Plan, error) {
	return r.plan, nil
}
func (r *handlerPlanPlanRepo) FindByType(domain.PlanType) (*domain.Plan, error) { return nil, nil }
func (r *handlerPlanPlanRepo) FindAll() ([]*domain.Plan, error)                 { return nil, nil }
func (r *handlerPlanPlanRepo) FindActive() ([]*domain.Plan, error)              { return nil, nil }

type handlerWalletCountRepo struct {
	count int64
}

func (r *handlerWalletCountRepo) CountByUserID(uuid.UUID) (int64, error) {
	return r.count, nil
}

func (*handlerWalletCountRepo) Create(*domain.ScanResultEntity) error { return nil }
func (*handlerWalletCountRepo) FindByUserID(uuid.UUID, int, int) ([]*domain.ScanResultEntity, error) {
	return nil, nil
}
func (*handlerWalletCountRepo) FindByID(uuid.UUID) (*domain.ScanResultEntity, error) { return nil, nil }
func (*handlerWalletCountRepo) FindOwnedWalletScanByID(uuid.UUID, uuid.UUID) (*domain.ScanResultEntity, error) {
	return nil, nil
}
func (*handlerWalletCountRepo) DeleteOwnedWalletScan(uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}
func (*handlerWalletCountRepo) ListOwnerWalletScansDiscoveryV1(uuid.UUID, string, int, int) ([]*domain.ScanResultEntity, int64, error) {
	return nil, 0, nil
}
func (*handlerWalletCountRepo) ListOwnerWalletScansByAddress(uuid.UUID, string) ([]*domain.ScanResultEntity, error) {
	return nil, nil
}

var _ repository.ScanResultRepository = (*handlerWalletCountRepo)(nil)

func TestDiscoveryHandler_checkScanLimits_UsesPostgresExecutionCount(t *testing.T) {
	t.Parallel()

	planID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	plan := &domain.Plan{ID: planID, WalletScanLimit: 2}
	user := &domain.User{ID: userID, PlanID: planID}
	planSvc := service.NewPlanService(&handlerPlanPlanRepo{plan: plan}, &handlerPlanUserRepo{user: user})

	h := &DiscoveryHandler{
		planService:    planSvc,
		scanResultRepo: &handlerWalletCountRepo{count: 2},
	}

	limitReached, msg, err := h.checkScanLimits(userID, "wallet")
	if err != nil {
		t.Fatalf("checkScanLimits: %v", err)
	}
	if !limitReached {
		t.Fatal("expected limit reached at 2 persisted executions")
	}
	if msg == "" {
		t.Fatal("expected non-empty limit message")
	}
}
