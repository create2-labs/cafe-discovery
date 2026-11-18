package service

import (
	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"
	"cafe-discovery/pkg/scan"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	ErrPlanNotFound = errors.New("plan not found")
)

// PlanService handles plan operations
type PlanService struct {
	planRepo repository.PlanRepository
	userRepo repository.UserRepository
}

// NewPlanService creates a new plan service
func NewPlanService(planRepo repository.PlanRepository, userRepo repository.UserRepository) *PlanService {
	return &PlanService{
		planRepo: planRepo,
		userRepo: userRepo,
	}
}

// GetUserPlan retrieves the plan for a user
func (s *PlanService) GetUserPlan(userID uuid.UUID) (*domain.Plan, error) {
	user, err := s.userRepo.FindByID(userID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	plan, err := s.planRepo.FindByID(user.PlanID)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan: %w", err)
	}
	if plan == nil {
		return nil, ErrPlanNotFound
	}

	return plan, nil
}

// GetAllPlans retrieves all available plans
func (s *PlanService) GetAllPlans() ([]*domain.Plan, error) {
	plans, err := s.planRepo.FindActive()
	if err != nil {
		return nil, fmt.Errorf("failed to get plans: %w", err)
	}
	return plans, nil
}

// GetPlanUsage retrieves the current usage for a user
type PlanUsage struct {
	WalletScansUsed            int `json:"wallet_scans_used"`
	EndpointScansUsed          int `json:"endpoint_scans_used"`
	WalletScansVisible         int `json:"wallet_scans_visible"`
	EndpointScansVisible       int `json:"endpoint_scans_visible"`
	WalletScansDeletedByUser   int `json:"wallet_scans_deleted_by_user"`
	EndpointScansDeletedByUser int `json:"endpoint_scans_deleted_by_user"`
	WalletScansInFlight        int `json:"wallet_scans_in_flight,omitempty"`
	EndpointScansInFlight      int `json:"endpoint_scans_in_flight,omitempty"`
	WalletScanLimit            int `json:"wallet_scan_limit"`
	EndpointScanLimit          int `json:"endpoint_scan_limit"`
	WalletScansLeft            int `json:"wallet_scans_left"`   // -1 if unlimited
	EndpointScansLeft          int `json:"endpoint_scans_left"` // -1 if unlimited
}

// PostScanQuotaDenyReason explains why POST scan was rejected (IMM-6b G1/G2).
type PostScanQuotaDenyReason string

const (
	PostScanQuotaOK           PostScanQuotaDenyReason = ""
	PostScanQuotaDenyQuota    PostScanQuotaDenyReason = "quota"
	PostScanQuotaDenyParallel PostScanQuotaDenyReason = "parallel"
)

// GetPlanUsage returns plan usage from the success-only ledger (IMM-6b P1).
// used = ledger success count; visible = active success rows; deleted_by_user = used - visible.
func (s *PlanService) GetPlanUsage(userID uuid.UUID, ledger repository.ScanUsageLedgerRepository) (*PlanUsage, error) {
	if userID == uuid.Nil {
		return nil, errors.New("user not authenticated")
	}
	if ledger == nil {
		return nil, errors.New("scan usage ledger required")
	}

	plan, err := s.GetUserPlan(userID)
	if err != nil {
		return nil, err
	}

	walletUsed, err := ledger.CountSuccessUsage(userID, domain.ScanUsageKindWallet)
	if err != nil {
		return nil, fmt.Errorf("count wallet ledger usage: %w", err)
	}
	walletVisible, err := ledger.CountVisibleSuccessScans(userID, domain.ScanUsageKindWallet)
	if err != nil {
		return nil, fmt.Errorf("count visible wallet successes: %w", err)
	}
	walletInFlight, err := ledger.CountInFlightScans(userID, domain.ScanUsageKindWallet)
	if err != nil {
		return nil, fmt.Errorf("count in-flight wallet scans: %w", err)
	}

	endpointUsed, err := ledger.CountSuccessUsage(userID, domain.ScanUsageKindEndpoint)
	if err != nil {
		return nil, fmt.Errorf("count endpoint ledger usage: %w", err)
	}
	endpointVisible, err := ledger.CountVisibleSuccessScans(userID, domain.ScanUsageKindEndpoint)
	if err != nil {
		return nil, fmt.Errorf("count visible endpoint successes: %w", err)
	}
	endpointInFlight, err := ledger.CountInFlightScans(userID, domain.ScanUsageKindEndpoint)
	if err != nil {
		return nil, fmt.Errorf("count in-flight endpoint scans: %w", err)
	}

	usage := &PlanUsage{
		WalletScansUsed:            int(walletUsed),
		EndpointScansUsed:          int(endpointUsed),
		WalletScansVisible:         int(walletVisible),
		EndpointScansVisible:       int(endpointVisible),
		WalletScansDeletedByUser:   int(walletUsed - walletVisible),
		EndpointScansDeletedByUser: int(endpointUsed - endpointVisible),
		WalletScansInFlight:        int(walletInFlight),
		EndpointScansInFlight:      int(endpointInFlight),
		WalletScanLimit:            plan.WalletScanLimit,
		EndpointScanLimit:          plan.EndpointScanLimit,
	}

	usage.WalletScansLeft = planScansLeft(plan.WalletScanLimit, walletUsed, plan.IsUnlimited(scan.PlanLimitKeyWallet))
	usage.EndpointScansLeft = planScansLeft(plan.EndpointScanLimit, endpointUsed, plan.IsUnlimited(scan.PlanLimitKeyEndpoint))

	return usage, nil
}

// planParallelScanCap is G2: min(limit, 3) when limited, else 3 when unlimited (limit 0).
func planParallelScanCap(limit int, unlimited bool) int {
	if unlimited || limit <= 0 {
		return 3
	}
	if limit < 3 {
		return limit
	}
	return 3
}

// ReservePostScanQuota inserts a ledger row for scanID when the plan quota and the parallel cap allow it.
// The row counts as a success. Quota and the parallel cap are decided in that same insert.
func (s *PlanService) ReservePostScanQuota(
	userID, scanID uuid.UUID,
	scanType string,
	ledger repository.ScanUsageLedgerRepository,
) (bool, *PlanUsage, PostScanQuotaDenyReason, error) {
	if userID == uuid.Nil {
		return false, nil, PostScanQuotaOK, errors.New("user not authenticated")
	}
	if scanID == uuid.Nil {
		return false, nil, PostScanQuotaOK, errors.New("scan id required")
	}
	if ledger == nil {
		return false, nil, PostScanQuotaOK, errors.New("scan usage ledger required")
	}

	kind, err := scanUsageKindForPlanLimitKey(scanType)
	if err != nil {
		return false, nil, PostScanQuotaOK, err
	}

	plan, err := s.GetUserPlan(userID)
	if err != nil {
		return false, nil, PostScanQuotaOK, err
	}

	unlimited := plan.IsUnlimited(scanType)
	var limit int
	switch scanType {
	case scan.PlanLimitKeyWallet:
		limit = plan.WalletScanLimit
	case scan.PlanLimitKeyEndpoint:
		limit = plan.EndpointScanLimit
	default:
		return false, nil, PostScanQuotaOK, fmt.Errorf("unknown scan type: %s", scanType)
	}
	quotaLimit := limit
	if unlimited {
		quotaLimit = 0
	}

	res, err := ledger.ReserveScanUsage(userID, scanID, kind, quotaLimit, planParallelScanCap(limit, unlimited))
	if err != nil {
		return false, nil, PostScanQuotaOK, err
	}

	usage := usageFromReservation(scanType, plan, res)
	if !res.Reserved {
		deny := PostScanQuotaDenyQuota
		if res.Deny == repository.ScanUsageDenyParallel {
			deny = PostScanQuotaDenyParallel
		}
		return false, usage, deny, nil
	}
	return true, usage, PostScanQuotaOK, nil
}

func usageFromReservation(scanType string, plan *domain.Plan, res repository.ScanUsageReservation) *PlanUsage {
	inFlight := res.ExtraInFlight
	if res.Deny == repository.ScanUsageDenyParallel {
		inFlight = res.ParallelInFlight
	}
	usage := &PlanUsage{
		WalletScanLimit:   plan.WalletScanLimit,
		EndpointScanLimit: plan.EndpointScanLimit,
	}
	switch scanType {
	case scan.PlanLimitKeyWallet:
		usage.WalletScansUsed = int(res.LedgerCount)
		usage.WalletScansInFlight = int(inFlight)
		usage.WalletScansLeft = planScansLeft(plan.WalletScanLimit, res.LedgerCount, plan.IsUnlimited(scan.PlanLimitKeyWallet))
	case scan.PlanLimitKeyEndpoint:
		usage.EndpointScansUsed = int(res.LedgerCount)
		usage.EndpointScansInFlight = int(inFlight)
		usage.EndpointScansLeft = planScansLeft(plan.EndpointScanLimit, res.LedgerCount, plan.IsUnlimited(scan.PlanLimitKeyEndpoint))
	}
	return usage
}

func planScansLeft(limit int, successful int64, unlimited bool) int {
	if unlimited {
		return -1
	}
	left := limit - int(successful)
	if left < 0 {
		return 0
	}
	return left
}

func scanUsageKindForPlanLimitKey(scanType string) (domain.ScanUsageKind, error) {
	switch scanType {
	case scan.PlanLimitKeyWallet:
		return domain.ScanUsageKindWallet, nil
	case scan.PlanLimitKeyEndpoint:
		return domain.ScanUsageKindEndpoint, nil
	default:
		return "", fmt.Errorf("unknown scan type: %s", scanType)
	}
}
