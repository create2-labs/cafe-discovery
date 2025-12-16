package service

import (
	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"
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

	if plan.IsUnlimited("wallet") {
		usage.WalletScansLeft = -1
	} else {
		usage.WalletScansLeft = plan.WalletScanLimit - usage.WalletScansUsed
		if usage.WalletScansLeft < 0 {
			usage.WalletScansLeft = 0
		}
	}

	if plan.IsUnlimited("endpoint") {
		usage.EndpointScansLeft = -1
	} else {
		usage.EndpointScansLeft = plan.EndpointScanLimit - usage.EndpointScansUsed
		if usage.EndpointScansLeft < 0 {
			usage.EndpointScansLeft = 0
		}
	}

	return usage, nil
}

// CheckScanLimit checks if a user can perform a scan
func (s *PlanService) CheckScanLimit(userID uuid.UUID, scanType string, scanResultRepo repository.ScanResultRepository, tlsScanResultRepo repository.TLSScanResultRepository) (bool, *PlanUsage, error) {
	usage, err := s.GetPlanUsage(userID, scanResultRepo, tlsScanResultRepo)
	if err != nil {
		return false, nil, err
	}

	if scanType == "wallet" {
		if usage.WalletScanLimit == 0 {
			return true, usage, nil // Unlimited
		}
		return usage.WalletScansUsed < usage.WalletScanLimit, usage, nil
	}

	if scanType == "endpoint" {
		if usage.EndpointScanLimit == 0 {
			return true, usage, nil // Unlimited
		}
		return usage.EndpointScansUsed < usage.EndpointScanLimit, usage, nil
	}

	return false, usage, fmt.Errorf("unknown scan type: %s", scanType)
}

