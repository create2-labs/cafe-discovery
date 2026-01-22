package service

import (
	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrPlanNotFound = errors.New("plan not found")
)

// PlanService handles plan operations
type PlanService struct {
	planRepo repository.PlanRepository
	userRepo repository.UserRepository

	// Anonymous rate limiting: track scans per time window
	anonymousScans     map[string][]time.Time // key: scanType, value: timestamps
	anonymousScansLock sync.RWMutex
	rateLimitWindow    time.Duration // Time window for rate limiting (e.g., 1 hour)
	rateLimitMaxScans  int           // Max scans per window for anonymous users
}

// NewPlanService creates a new plan service
func NewPlanService(planRepo repository.PlanRepository, userRepo repository.UserRepository) *PlanService {
	return &PlanService{
		planRepo:          planRepo,
		userRepo:          userRepo,
		anonymousScans:    make(map[string][]time.Time),
		rateLimitWindow:   time.Hour, // 1 hour window
		rateLimitMaxScans: 5,         // 5 scans per hour for anonymous users (same as FREE plan)
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
	// Handle anonymous users (uuid.Nil) - use FREE plan with rate limiting
	if userID == uuid.Nil {
		return s.getAnonymousPlanUsage(scanResultRepo, tlsScanResultRepo)
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

// getAnonymousPlanUsage returns plan usage for anonymous users using rate limiting
func (s *PlanService) getAnonymousPlanUsage(scanResultRepo repository.ScanResultRepository, tlsScanResultRepo repository.TLSScanResultRepository) (*PlanUsage, error) {
	// Get FREE plan to use its limits
	freePlan, err := s.planRepo.FindByType(domain.PlanTypeFree)
	if err != nil {
		return nil, fmt.Errorf("failed to get free plan: %w", err)
	}
	if freePlan == nil {
		return nil, errors.New("free plan not found")
	}

	// Count scans from rate limiting cache (only scans in the current time window)
	now := time.Now()
	windowStart := now.Add(-s.rateLimitWindow)

	s.anonymousScansLock.RLock()
	walletScans := s.countScansInWindow("wallet", windowStart)
	endpointScans := s.countScansInWindow("endpoint", windowStart)
	s.anonymousScansLock.RUnlock()

	usage := &PlanUsage{
		WalletScansUsed:   walletScans,
		EndpointScansUsed: endpointScans,
		WalletScanLimit:   freePlan.WalletScanLimit,
		EndpointScanLimit: freePlan.EndpointScanLimit,
	}

	if freePlan.IsUnlimited("wallet") {
		usage.WalletScansLeft = -1
	} else {
		usage.WalletScansLeft = freePlan.WalletScanLimit - usage.WalletScansUsed
		if usage.WalletScansLeft < 0 {
			usage.WalletScansLeft = 0
		}
	}

	if freePlan.IsUnlimited("endpoint") {
		usage.EndpointScansLeft = -1
	} else {
		usage.EndpointScansLeft = freePlan.EndpointScanLimit - usage.EndpointScansUsed
		if usage.EndpointScansLeft < 0 {
			usage.EndpointScansLeft = 0
		}
	}

	return usage, nil
}

// countScansInWindow counts scans of a given type within the time window
func (s *PlanService) countScansInWindow(scanType string, windowStart time.Time) int {
	scans, exists := s.anonymousScans[scanType]
	if !exists {
		return 0
	}

	count := 0
	for _, timestamp := range scans {
		if timestamp.After(windowStart) {
			count++
		}
	}
	return count
}

// recordAnonymousScan records a scan for an anonymous user (for rate limiting)
func (s *PlanService) recordAnonymousScan(scanType string) {
	now := time.Now()
	windowStart := now.Add(-s.rateLimitWindow)

	s.anonymousScansLock.Lock()
	defer s.anonymousScansLock.Unlock()

	// Clean up old scans outside the window
	scans := s.anonymousScans[scanType]
	validScans := make([]time.Time, 0, len(scans))
	for _, timestamp := range scans {
		if timestamp.After(windowStart) {
			validScans = append(validScans, timestamp)
		}
	}

	// Add new scan
	validScans = append(validScans, now)
	s.anonymousScans[scanType] = validScans
}

// CheckScanLimit checks if a user can perform a scan
func (s *PlanService) CheckScanLimit(userID uuid.UUID, scanType string, scanResultRepo repository.ScanResultRepository, tlsScanResultRepo repository.TLSScanResultRepository) (bool, *PlanUsage, error) {
	usage, err := s.GetPlanUsage(userID, scanResultRepo, tlsScanResultRepo)
	if err != nil {
		return false, nil, err
	}

	var canScan bool
	switch scanType {
	case "wallet":
		if usage.WalletScanLimit == 0 {
			canScan = true // Unlimited
		} else {
			canScan = usage.WalletScansUsed < usage.WalletScanLimit
		}
	case "endpoint":
		if usage.EndpointScanLimit == 0 {
			canScan = true // Unlimited
		} else {
			canScan = usage.EndpointScansUsed < usage.EndpointScanLimit
		}
	default:
		return false, usage, fmt.Errorf("unknown scan type: %s", scanType)
	}

	// If scan is allowed for anonymous users, record it for rate limiting
	if canScan && userID == uuid.Nil {
		s.recordAnonymousScan(scanType)
	}

	return canScan, usage, nil
}
