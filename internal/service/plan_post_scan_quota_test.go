package service

import (
	"testing"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type postScanLedgerStub struct {
	successful int64
	inFlight   int64
}

func (s *postScanLedgerStub) RecordSuccessUsage(uuid.UUID, uuid.UUID, domain.ScanUsageKind) error {
	return nil
}
func (s *postScanLedgerStub) CountSuccessUsage(uuid.UUID, domain.ScanUsageKind) (int64, error) {
	return s.successful, nil
}
func (s *postScanLedgerStub) CountInFlightScans(uuid.UUID, domain.ScanUsageKind) (int64, error) {
	return s.inFlight, nil
}
func (s *postScanLedgerStub) CountVisibleSuccessScans(uuid.UUID, domain.ScanUsageKind) (int64, error) {
	return s.successful, nil
}
func (s *postScanLedgerStub) TryAcquireSuccessSlot(uuid.UUID, domain.ScanUsageKind, int) (bool, error) {
	return true, nil
}
func (s *postScanLedgerStub) RecordSuccessUsageInTx(*gorm.DB, uuid.UUID, uuid.UUID, domain.ScanUsageKind) error {
	return nil
}
func (s *postScanLedgerStub) TryAcquireSuccessSlotInTx(*gorm.DB, uuid.UUID, domain.ScanUsageKind, int) (bool, error) {
	return true, nil
}
func (s *postScanLedgerStub) RecordSuccessUsageIfUnderLimitInTx(*gorm.DB, uuid.UUID, uuid.UUID, domain.ScanUsageKind, int) (bool, error) {
	return true, nil
}

var _ repository.ScanUsageLedgerRepository = (*postScanLedgerStub)(nil)

func TestPlanParallelScanCap(t *testing.T) {
	t.Parallel()
	tests := []struct {
		limit     int
		unlimited bool
		want      int
	}{
		{limit: 0, unlimited: true, want: 3},
		{limit: 5, unlimited: false, want: 3},
		{limit: 2, unlimited: false, want: 2},
		{limit: 1, unlimited: false, want: 1},
	}
	for _, tc := range tests {
		if got := planParallelScanCap(tc.limit, tc.unlimited); got != tc.want {
			t.Errorf("planParallelScanCap(%d, %v) = %d, want %d", tc.limit, tc.unlimited, got, tc.want)
		}
	}
}

func TestCheckPostScanQuota_G1SuccessfulPlusInFlight(t *testing.T) {
	t.Parallel()

	planID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	plan := &domain.Plan{ID: planID, WalletScanLimit: 5}
	user := &domain.User{ID: userID, PlanID: planID}
	svc := NewPlanService(&planQuotaPlanRepo{plan: plan}, &planQuotaUserRepo{user: user})
	ledger := &postScanLedgerStub{successful: 4, inFlight: 1}

	canScan, _, deny, err := svc.CheckPostScanQuota(userID, "wallet", ledger)
	if err != nil {
		t.Fatalf("CheckPostScanQuota: %v", err)
	}
	if canScan {
		t.Fatal("expected POST blocked when successful+in_flight reaches limit")
	}
	if deny != PostScanQuotaDenyQuota {
		t.Fatalf("deny = %q, want quota", deny)
	}

	ledger.inFlight = 0
	canScan, _, deny, err = svc.CheckPostScanQuota(userID, "wallet", ledger)
	if err != nil {
		t.Fatalf("CheckPostScanQuota: %v", err)
	}
	if !canScan {
		t.Fatalf("expected POST allowed with 4/5 successful and no in-flight, deny=%q", deny)
	}
}

func TestCheckPostScanQuota_G2ParallelCap(t *testing.T) {
	t.Parallel()

	planID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	plan := &domain.Plan{ID: planID, WalletScanLimit: 100}
	user := &domain.User{ID: userID, PlanID: planID}
	svc := NewPlanService(&planQuotaPlanRepo{plan: plan}, &planQuotaUserRepo{user: user})
	ledger := &postScanLedgerStub{successful: 0, inFlight: 3}

	canScan, _, deny, err := svc.CheckPostScanQuota(userID, "wallet", ledger)
	if err != nil {
		t.Fatalf("CheckPostScanQuota: %v", err)
	}
	if canScan {
		t.Fatal("expected POST blocked at parallel cap (3 in-flight)")
	}
	if deny != PostScanQuotaDenyParallel {
		t.Fatalf("deny = %q, want parallel", deny)
	}
}

func TestCheckPostScanQuota_UnlimitedStillCapsParallelism(t *testing.T) {
	t.Parallel()

	planID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	plan := &domain.Plan{ID: planID, WalletScanLimit: 0}
	user := &domain.User{ID: userID, PlanID: planID}
	svc := NewPlanService(&planQuotaPlanRepo{plan: plan}, &planQuotaUserRepo{user: user})
	ledger := &postScanLedgerStub{successful: 10, inFlight: 3}

	canScan, _, deny, err := svc.CheckPostScanQuota(userID, "wallet", ledger)
	if err != nil {
		t.Fatalf("CheckPostScanQuota: %v", err)
	}
	if canScan {
		t.Fatal("expected parallel cap even on unlimited plan")
	}
	if deny != PostScanQuotaDenyParallel {
		t.Fatalf("deny = %q, want parallel", deny)
	}
}
