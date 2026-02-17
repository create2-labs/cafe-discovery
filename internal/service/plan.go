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
	WalletScansUsed   int `json:"wallet_scans_used"`
	EndpointScansUsed int `json:"endpoint_scans_used"`
	WalletScanLimit   int `json:"wallet_scan_limit"`
	EndpointScanLimit int `json:"endpoint_scan_limit"`
	WalletScansLeft   int `json:"wallet_scans_left"`   // -1 if unlimited
	EndpointScansLeft int `json:"endpoint_scans_left"` // -1 if unlimited
}

func (s *PlanService) GetPlanUsage(userID uuid.UUID, scanResultRepo repository.ScanResultRepository, tlsScanResultRepo repository.TLSScanResultRepository) (*PlanUsage, error) {
	if userID == uuid.Nil {
		return nil, errors.New("user not authenticated")
	}

	plan, err := s.GetUserPlan(userID)
	if err != nil {
		return nil, err
	}

	// Count wallet scans
	var walletCount int64
	if scanResultRepo != nil {
		var err error
		walletCount, err = scanResultRepo.CountByUserID(userID)
		if err != nil {
			return nil, fmt.Errorf("failed to count wallet scans: %w", err)
		}
	}

	// Count endpoint scans
	var endpointCount int64
	if tlsScanResultRepo != nil {
		var err error
		endpointCount, err = tlsScanResultRepo.CountByUserID(userID)
		if err != nil {
			return nil, fmt.Errorf("failed to count endpoint scans: %w", err)
		}
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

// GetPlanUsageFromCounts returns plan usage when scan counts are provided by the backend (e.g. from Redis).
// Used when backend has no Postgres scan repos.
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

// CheckScanLimitFromCounts checks if a user can perform a scan when counts come from Redis (backend Redis-only).
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

// CheckScanLimit checks if a user can perform a scan
func (s *PlanService) CheckScanLimit(userID uuid.UUID, scanType string, scanResultRepo repository.ScanResultRepository, tlsScanResultRepo repository.TLSScanResultRepository) (bool, *PlanUsage, error) {
	usage, err := s.GetPlanUsage(userID, scanResultRepo, tlsScanResultRepo)
	if err != nil {
		return false, nil, err
	}

	var canScan bool
	switch scanType {
	case scan.PlanLimitKeyWallet:
		if usage.WalletScanLimit == 0 {
			canScan = true // Unlimited
		} else {
			canScan = usage.WalletScansUsed < usage.WalletScanLimit
		}
	case scan.PlanLimitKeyEndpoint:
		if usage.EndpointScanLimit == 0 {
			canScan = true // Unlimited
		} else {
			canScan = usage.EndpointScansUsed < usage.EndpointScanLimit
		}
		default:
		return false, usage, fmt.Errorf("unknown scan type: %s", scanType)
	}

	return canScan, usage, nil
}

// CheckEndpointScanLimitWithCount checks if the user can perform an endpoint (TLS) scan when the caller
// has already computed the endpoint count. Use this when the handler has direct access to the TLS repo
// to avoid any ambiguity with repo injection order. Returns (canScan, usage, error).
func (s *PlanService) CheckEndpointScanLimitWithCount(userID uuid.UUID, endpointCount int) (bool, *PlanUsage, error) {
	if userID == uuid.Nil {
		return false, nil, errors.New("user not authenticated")
	}
	plan, err := s.GetUserPlan(userID)
	if err != nil {
		return false, nil, err
	}
	limit := plan.EndpointScanLimit
	unlimited := plan.IsUnlimited(scan.PlanLimitKeyEndpoint)
	canScan := unlimited || endpointCount < limit
	usage := &PlanUsage{
		WalletScansUsed:   0, // not needed for endpoint check
		EndpointScansUsed: endpointCount,
		WalletScanLimit:   plan.WalletScanLimit,
		EndpointScanLimit: limit,
	}
	if unlimited {
		usage.EndpointScansLeft = -1
	} else {
		usage.EndpointScansLeft = limit - endpointCount
		if usage.EndpointScansLeft < 0 {
			usage.EndpointScansLeft = 0
		}
	}
	return canScan, usage, nil
}
