package service

import (
	"testing"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type usageAPILedgerStub struct {
	walletUsed      int64
	walletVisible   int64
	walletInFlight  int64
	endpointUsed    int64
	endpointVisible int64
	endpointInFlight int64
}

func (s *usageAPILedgerStub) RecordSuccessUsage(uuid.UUID, uuid.UUID, domain.ScanUsageKind) error {
	return nil
}
func (s *usageAPILedgerStub) CountSuccessUsage(_ uuid.UUID, kind domain.ScanUsageKind) (int64, error) {
	switch kind {
	case domain.ScanUsageKindWallet:
		return s.walletUsed, nil
	case domain.ScanUsageKindEndpoint:
		return s.endpointUsed, nil
	default:
		return 0, nil
	}
}
func (s *usageAPILedgerStub) CountInFlightScans(_ uuid.UUID, kind domain.ScanUsageKind) (int64, error) {
	switch kind {
	case domain.ScanUsageKindWallet:
		return s.walletInFlight, nil
	case domain.ScanUsageKindEndpoint:
		return s.endpointInFlight, nil
	default:
		return 0, nil
	}
}
func (s *usageAPILedgerStub) CountVisibleSuccessScans(_ uuid.UUID, kind domain.ScanUsageKind) (int64, error) {
	switch kind {
	case domain.ScanUsageKindWallet:
		return s.walletVisible, nil
	case domain.ScanUsageKindEndpoint:
		return s.endpointVisible, nil
	default:
		return 0, nil
	}
}
func (s *usageAPILedgerStub) TryAcquireSuccessSlot(uuid.UUID, domain.ScanUsageKind, int) (bool, error) {
	return true, nil
}
func (s *usageAPILedgerStub) RecordSuccessUsageInTx(*gorm.DB, uuid.UUID, uuid.UUID, domain.ScanUsageKind) error {
	return nil
}
func (s *usageAPILedgerStub) TryAcquireSuccessSlotInTx(*gorm.DB, uuid.UUID, domain.ScanUsageKind, int) (bool, error) {
	return true, nil
}
func (s *usageAPILedgerStub) RecordSuccessUsageIfUnderLimitInTx(*gorm.DB, uuid.UUID, uuid.UUID, domain.ScanUsageKind, int) (bool, error) {
	return true, nil
}

var _ repository.ScanUsageLedgerRepository = (*usageAPILedgerStub)(nil)

func TestGetPlanUsage_LedgerBreakdown(t *testing.T) {
	t.Parallel()

	planID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	plan := &domain.Plan{ID: planID, WalletScanLimit: 5, EndpointScanLimit: 3}
	user := &domain.User{ID: userID, PlanID: planID}
	svc := NewPlanService(&planQuotaPlanRepo{plan: plan}, &planQuotaUserRepo{user: user})

	ledger := &usageAPILedgerStub{
		walletUsed:       3,
		walletVisible:    2,
		walletInFlight:   1,
		endpointUsed:     2,
		endpointVisible:  1,
		endpointInFlight: 0,
	}

	usage, err := svc.GetPlanUsage(userID, ledger)
	if err != nil {
		t.Fatalf("GetPlanUsage: %v", err)
	}

	if usage.WalletScansUsed != 3 {
		t.Fatalf("WalletScansUsed = %d, want 3", usage.WalletScansUsed)
	}
	if usage.WalletScansVisible != 2 {
		t.Fatalf("WalletScansVisible = %d, want 2", usage.WalletScansVisible)
	}
	if usage.WalletScansDeletedByUser != 1 {
		t.Fatalf("WalletScansDeletedByUser = %d, want 1 (used - visible)", usage.WalletScansDeletedByUser)
	}
	if usage.WalletScansInFlight != 1 {
		t.Fatalf("WalletScansInFlight = %d, want 1", usage.WalletScansInFlight)
	}
	if usage.WalletScansLeft != 2 {
		t.Fatalf("WalletScansLeft = %d, want 2", usage.WalletScansLeft)
	}

	if usage.EndpointScansUsed != 2 {
		t.Fatalf("EndpointScansUsed = %d, want 2", usage.EndpointScansUsed)
	}
	if usage.EndpointScansVisible != 1 {
		t.Fatalf("EndpointScansVisible = %d, want 1", usage.EndpointScansVisible)
	}
	if usage.EndpointScansDeletedByUser != 1 {
		t.Fatalf("EndpointScansDeletedByUser = %d, want 1", usage.EndpointScansDeletedByUser)
	}
	if usage.EndpointScansLeft != 1 {
		t.Fatalf("EndpointScansLeft = %d, want 1", usage.EndpointScansLeft)
	}
}

func TestGetPlanUsage_DeleteSuccessScanUsedStableVisibleDown(t *testing.T) {
	t.Parallel()

	planID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	plan := &domain.Plan{ID: planID, WalletScanLimit: 5}
	user := &domain.User{ID: userID, PlanID: planID}
	svc := NewPlanService(&planQuotaPlanRepo{plan: plan}, &planQuotaUserRepo{user: user})

	before := &usageAPILedgerStub{walletUsed: 2, walletVisible: 2}
	after := &usageAPILedgerStub{walletUsed: 2, walletVisible: 1}

	beforeUsage, err := svc.GetPlanUsage(userID, before)
	if err != nil {
		t.Fatalf("GetPlanUsage before: %v", err)
	}
	afterUsage, err := svc.GetPlanUsage(userID, after)
	if err != nil {
		t.Fatalf("GetPlanUsage after: %v", err)
	}

	if beforeUsage.WalletScansUsed != afterUsage.WalletScansUsed {
		t.Fatalf("used changed on delete: before=%d after=%d", beforeUsage.WalletScansUsed, afterUsage.WalletScansUsed)
	}
	if afterUsage.WalletScansVisible != beforeUsage.WalletScansVisible-1 {
		t.Fatalf("visible after delete = %d, want %d", afterUsage.WalletScansVisible, beforeUsage.WalletScansVisible-1)
	}
	if afterUsage.WalletScansDeletedByUser != beforeUsage.WalletScansDeletedByUser+1 {
		t.Fatalf("deleted_by_user after delete = %d, want %d", afterUsage.WalletScansDeletedByUser, beforeUsage.WalletScansDeletedByUser+1)
	}
}

func TestGetPlanUsage_RequiresLedger(t *testing.T) {
	t.Parallel()

	planID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	plan := &domain.Plan{ID: planID, WalletScanLimit: 5}
	user := &domain.User{ID: userID, PlanID: planID}
	svc := NewPlanService(&planQuotaPlanRepo{plan: plan}, &planQuotaUserRepo{user: user})

	if _, err := svc.GetPlanUsage(userID, nil); err == nil {
		t.Fatal("expected error when ledger is nil")
	}
}
