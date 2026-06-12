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

// GetPlanUsageFromCounts returns plan usage when scan counts are provided externally (tests or legacy callers).
func (s *PlanService) GetPlanUsageFromCounts(userID uuid.UUID, walletCount, endpointCount int64) (*PlanUsage, error) {
	if userID == uuid.Nil {
		return nil, errors.New("user not authenticated")
	}
	plan, err := s.GetUserPlan(userID)
	if err != nil {
		return nil, err
	}
	usage := &PlanUsage{
		WalletScansUsed:   int(walletCount),
		EndpointScansUsed: int(endpointCount),
		WalletScanLimit:   plan.WalletScanLimit,
		EndpointScanLimit: plan.EndpointScanLimit,
	}
	if plan.IsUnlimited(scan.PlanLimitKeyWallet) {
		usage.WalletScansLeft = -1
	} else {
		usage.WalletScansLeft = plan.WalletScanLimit - usage.WalletScansUsed
		if usage.WalletScansLeft < 0 {
			usage.WalletScansLeft = 0
		}
	}
	if plan.IsUnlimited(scan.PlanLimitKeyEndpoint) {
		usage.EndpointScansLeft = -1
	} else {
		usage.EndpointScansLeft = plan.EndpointScanLimit - usage.EndpointScansUsed
		if usage.EndpointScansLeft < 0 {
			usage.EndpointScansLeft = 0
		}
	}
	return usage, nil
}

// CheckScanLimitFromCounts checks if a user can perform a scan when counts are provided externally.
func (s *PlanService) CheckScanLimitFromCounts(userID uuid.UUID, scanType string, walletCount, endpointCount int64) (bool, *PlanUsage, error) {
	usage, err := s.GetPlanUsageFromCounts(userID, walletCount, endpointCount)
	if err != nil {
		return false, nil, err
	}
	var canScan bool
	switch scanType {
	case scan.PlanLimitKeyWallet:
		canScan = usage.WalletScanLimit == 0 || usage.WalletScansUsed < usage.WalletScanLimit
	case scan.PlanLimitKeyEndpoint:
		canScan = usage.EndpointScanLimit == 0 || usage.EndpointScansUsed < usage.EndpointScanLimit
	default:
		return false, usage, fmt.Errorf("unknown scan type: %s", scanType)
	}
	return canScan, usage, nil
}

// CheckScanLimit checks if a user can perform a scan using legacy row counts.
// Superseded for POST by CheckPostScanQuota (IMM-6b G1/G2).
func (s *PlanService) CheckScanLimit(userID uuid.UUID, scanType string, scanResultRepo repository.ScanResultRepository, tlsScanResultRepo repository.TLSScanResultRepository) (bool, *PlanUsage, error) {
	var walletCount, endpointCount int64
	if scanResultRepo != nil {
		var err error
		walletCount, err = scanResultRepo.CountByUserID(userID)
		if err != nil {
			return false, nil, fmt.Errorf("failed to count wallet scans: %w", err)
		}
	}
	if tlsScanResultRepo != nil {
		var err error
		endpointCount, err = tlsScanResultRepo.CountByUserID(userID)
		if err != nil {
			return false, nil, fmt.Errorf("failed to count endpoint scans: %w", err)
		}
	}
	return s.CheckScanLimitFromCounts(userID, scanType, walletCount, endpointCount)
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

// CheckPostScanQuota enforces IMM-6b POST guards (G1 success+in-flight, G2 parallel cap) via the ledger.
func (s *PlanService) CheckPostScanQuota(
	userID uuid.UUID,
	scanType string,
	ledger repository.ScanUsageLedgerRepository,
) (bool, *PlanUsage, PostScanQuotaDenyReason, error) {
	if userID == uuid.Nil {
		return false, nil, PostScanQuotaOK, errors.New("user not authenticated")
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

	successful, err := ledger.CountSuccessUsage(userID, kind)
	if err != nil {
		return false, nil, PostScanQuotaOK, fmt.Errorf("count successful usage: %w", err)
	}
	inFlight, err := ledger.CountInFlightScans(userID, kind)
	if err != nil {
		return false, nil, PostScanQuotaOK, fmt.Errorf("count in-flight scans: %w", err)
	}

	usage := &PlanUsage{
		WalletScanLimit:   plan.WalletScanLimit,
		EndpointScanLimit: plan.EndpointScanLimit,
	}
	switch scanType {
	case scan.PlanLimitKeyWallet:
		usage.WalletScansUsed = int(successful)
		usage.WalletScansInFlight = int(inFlight)
		usage.WalletScansLeft = planScansLeft(plan.WalletScanLimit, successful, plan.IsUnlimited(scan.PlanLimitKeyWallet))
	case scan.PlanLimitKeyEndpoint:
		usage.EndpointScansUsed = int(successful)
		usage.EndpointScansInFlight = int(inFlight)
		usage.EndpointScansLeft = planScansLeft(plan.EndpointScanLimit, successful, plan.IsUnlimited(scan.PlanLimitKeyEndpoint))
	default:
		return false, usage, PostScanQuotaOK, fmt.Errorf("unknown scan type: %s", scanType)
	}

	unlimited := plan.IsUnlimited(scanType)
	var limit int
	switch scanType {
	case scan.PlanLimitKeyWallet:
		limit = plan.WalletScanLimit
	case scan.PlanLimitKeyEndpoint:
		limit = plan.EndpointScanLimit
	}

	parallelCap := planParallelScanCap(limit, unlimited)
	deny := PostScanQuotaOK
	if !unlimited && successful+inFlight >= int64(limit) {
		deny = PostScanQuotaDenyQuota
	}
	if inFlight >= int64(parallelCap) {
		if deny == PostScanQuotaOK {
			deny = PostScanQuotaDenyParallel
		}
	}
	return deny == PostScanQuotaOK, usage, deny, nil
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
