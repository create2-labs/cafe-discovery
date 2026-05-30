package handler

import (
	"testing"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"
	"cafe-discovery/internal/service"

	"github.com/google/uuid"
	"gorm.io/gorm"
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

type handlerPostScanLedgerStub struct {
	successful int64
	inFlight   int64
}

func (s *handlerPostScanLedgerStub) RecordSuccessUsage(uuid.UUID, uuid.UUID, domain.ScanUsageKind) error {
	return nil
}
func (s *handlerPostScanLedgerStub) CountSuccessUsage(uuid.UUID, domain.ScanUsageKind) (int64, error) {
	return s.successful, nil
}
func (s *handlerPostScanLedgerStub) CountInFlightScans(uuid.UUID, domain.ScanUsageKind) (int64, error) {
	return s.inFlight, nil
}
func (s *handlerPostScanLedgerStub) CountVisibleSuccessScans(uuid.UUID, domain.ScanUsageKind) (int64, error) {
	return s.successful, nil
}
func (s *handlerPostScanLedgerStub) TryAcquireSuccessSlot(uuid.UUID, domain.ScanUsageKind, int) (bool, error) {
	return true, nil
}
func (s *handlerPostScanLedgerStub) RecordSuccessUsageInTx(*gorm.DB, uuid.UUID, uuid.UUID, domain.ScanUsageKind) error {
	return nil
}
func (s *handlerPostScanLedgerStub) TryAcquireSuccessSlotInTx(*gorm.DB, uuid.UUID, domain.ScanUsageKind, int) (bool, error) {
	return true, nil
}
func (s *handlerPostScanLedgerStub) RecordSuccessUsageIfUnderLimitInTx(*gorm.DB, uuid.UUID, uuid.UUID, domain.ScanUsageKind, int) (bool, error) {
	return true, nil
}

var _ repository.ScanUsageLedgerRepository = (*handlerPostScanLedgerStub)(nil)

func TestDiscoveryHandler_checkScanLimits_UsesLedgerG1(t *testing.T) {
	t.Parallel()

	planID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	plan := &domain.Plan{ID: planID, WalletScanLimit: 5}
	user := &domain.User{ID: userID, PlanID: planID}
	planSvc := service.NewPlanService(&handlerPlanPlanRepo{plan: plan}, &handlerPlanUserRepo{user: user})

	h := &DiscoveryHandler{
		planService:     planSvc,
		scanUsageLedger: &handlerPostScanLedgerStub{successful: 4, inFlight: 1},
	}

	limitReached, msg, err := h.checkScanLimits(userID, "wallet")
	if err != nil {
		t.Fatalf("checkScanLimits: %v", err)
	}
	if !limitReached {
		t.Fatal("expected limit reached at 4 successful + 1 in-flight / limit 5")
	}
	if msg == "" {
		t.Fatal("expected non-empty limit message")
	}
}
