package service

import (
	"testing"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type postScanLedgerStub struct {
	reservation repository.ScanUsageReservation
	reserveErr  error
	gotQuota    int
	gotParallel int
	gotKind     domain.ScanUsageKind
}

func (s *postScanLedgerStub) RecordSuccessUsage(uuid.UUID, uuid.UUID, domain.ScanUsageKind) error {
	return nil
}
func (s *postScanLedgerStub) CountSuccessUsage(uuid.UUID, domain.ScanUsageKind) (int64, error) {
	return 0, nil
}
func (s *postScanLedgerStub) CountInFlightScans(uuid.UUID, domain.ScanUsageKind) (int64, error) {
	return 0, nil
}
func (s *postScanLedgerStub) CountVisibleSuccessScans(uuid.UUID, domain.ScanUsageKind) (int64, error) {
	return 0, nil
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
func (s *postScanLedgerStub) ReserveScanUsage(_ uuid.UUID, _ uuid.UUID, kind domain.ScanUsageKind, quotaLimit, parallelCap int) (repository.ScanUsageReservation, error) {
	s.gotKind = kind
	s.gotQuota = quotaLimit
	s.gotParallel = parallelCap
	if s.reserveErr != nil {
		return repository.ScanUsageReservation{}, s.reserveErr
	}
	if s.reservation.Reserved || s.reservation.Deny != "" {
		return s.reservation, nil
	}
	return repository.ScanUsageReservation{Reserved: true}, nil
}
func (s *postScanLedgerStub) ReleaseSuccessUsageByScanID(uuid.UUID) error { return nil }
func (s *postScanLedgerStub) ListOpenScanReservations() ([]repository.OpenScanReservation, error) {
	return nil, nil
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

func TestReservePostScanQuota_PassesQuotaAndParallelCap(t *testing.T) {
	t.Parallel()

	planID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	plan := &domain.Plan{ID: planID, WalletScanLimit: 5, EndpointScanLimit: 2}
	user := &domain.User{ID: userID, PlanID: planID}
	svc := NewPlanService(&planQuotaPlanRepo{plan: plan}, &planQuotaUserRepo{user: user})
	ledger := &postScanLedgerStub{}

	reserved, _, deny, err := svc.ReservePostScanQuota(userID, uuid.New(), "wallet", ledger)
	if err != nil {
		t.Fatalf("ReservePostScanQuota: %v", err)
	}
	if !reserved || deny != PostScanQuotaOK {
		t.Fatalf("reserved=%v deny=%q", reserved, deny)
	}
	if ledger.gotKind != domain.ScanUsageKindWallet || ledger.gotQuota != 5 || ledger.gotParallel != 3 {
		t.Fatalf("wallet reserve args kind=%s quota=%d parallel=%d", ledger.gotKind, ledger.gotQuota, ledger.gotParallel)
	}

	reserved, _, _, err = svc.ReservePostScanQuota(userID, uuid.New(), "endpoint", ledger)
	if err != nil {
		t.Fatalf("ReservePostScanQuota endpoint: %v", err)
	}
	if !reserved {
		t.Fatal("expected endpoint reservation")
	}
	if ledger.gotKind != domain.ScanUsageKindEndpoint || ledger.gotQuota != 2 || ledger.gotParallel != 2 {
		t.Fatalf("endpoint reserve args kind=%s quota=%d parallel=%d", ledger.gotKind, ledger.gotQuota, ledger.gotParallel)
	}
}

func TestReservePostScanQuota_UnlimitedSkipsQuotaKeepsParallelCap(t *testing.T) {
	t.Parallel()

	planID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	plan := &domain.Plan{ID: planID, WalletScanLimit: 0}
	user := &domain.User{ID: userID, PlanID: planID}
	svc := NewPlanService(&planQuotaPlanRepo{plan: plan}, &planQuotaUserRepo{user: user})
	ledger := &postScanLedgerStub{
		reservation: repository.ScanUsageReservation{
			Deny:             repository.ScanUsageDenyParallel,
			ParallelInFlight: 3,
		},
	}

	reserved, usage, deny, err := svc.ReservePostScanQuota(userID, uuid.New(), "wallet", ledger)
	if err != nil {
		t.Fatalf("ReservePostScanQuota: %v", err)
	}
	if reserved {
		t.Fatal("expected parallel refusal")
	}
	if deny != PostScanQuotaDenyParallel {
		t.Fatalf("deny = %q, want parallel", deny)
	}
	if ledger.gotQuota != 0 || ledger.gotParallel != 3 {
		t.Fatalf("unlimited args quota=%d parallel=%d", ledger.gotQuota, ledger.gotParallel)
	}
	if usage.WalletScansInFlight != 3 {
		t.Fatalf("in-flight shown = %d, want 3", usage.WalletScansInFlight)
	}
}

func TestReservePostScanQuota_MapsQuotaDeny(t *testing.T) {
	t.Parallel()

	planID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	plan := &domain.Plan{ID: planID, WalletScanLimit: 5}
	user := &domain.User{ID: userID, PlanID: planID}
	svc := NewPlanService(&planQuotaPlanRepo{plan: plan}, &planQuotaUserRepo{user: user})
	ledger := &postScanLedgerStub{
		reservation: repository.ScanUsageReservation{
			Deny:          repository.ScanUsageDenyQuota,
			LedgerCount:   4,
			ExtraInFlight: 1,
		},
	}

	reserved, usage, deny, err := svc.ReservePostScanQuota(userID, uuid.New(), "wallet", ledger)
	if err != nil {
		t.Fatalf("ReservePostScanQuota: %v", err)
	}
	if reserved || deny != PostScanQuotaDenyQuota {
		t.Fatalf("reserved=%v deny=%q", reserved, deny)
	}
	if usage.WalletScansUsed != 4 || usage.WalletScansInFlight != 1 {
		t.Fatalf("usage used=%d inFlight=%d", usage.WalletScansUsed, usage.WalletScansInFlight)
	}
}
