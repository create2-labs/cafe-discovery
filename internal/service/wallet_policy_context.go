package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"cafe-discovery/internal/domain"

	"github.com/google/uuid"
)

// WalletPolicyContextDTO is the normalized wallet scan façade for CPM Option A
// over persisted wallet rows (authenticated user-owned data only).
//
// ScanID semantics (Option A, short-term):
// Today this is the primary key (UUID) of the wallet row in scan_results. It is
// owner-scoped via the authenticated user, opaque to other tenants, and stable
// for re-fetch and for CPM scan binding / AUTH-02 checks when that flow is wired.
// A future iteration may introduce a distinct identifier (e.g. walletObservationId
// or policySubjectRef); clients should treat scan_id as an opaque correlation id,
// not as a persistence schema leak.
//
// ChainIDs: populated only from known persisted network labels; unknown networks add
// no synthetic chain id (in particular never default to [1]).
type WalletPolicyContextDTO struct {
	ScanID           string `json:"scan_id"`
	WalletAddress    string `json:"wallet_address"`
	WalletType       string `json:"wallet_type"`
	ChainIDs         []int  `json:"chain_ids"`
	CurrentPQPosture string `json:"current_pq_posture"`
	ScannedAt        string `json:"scanned_at,omitempty"`
	// Status is normalized for consumers (e.g. SUCCESS stored in DB maps to completed).
	Status string `json:"status"`
}

// ListWalletPolicyContexts returns the current user's wallet scan rows as CPM-ready contexts.
// Pagination matches GET /discovery/scans (limit/offset from handler).
func (s *UserScanCacheService) ListWalletPolicyContexts(ctx context.Context, userID uuid.UUID, limit, offset int) ([]WalletPolicyContextDTO, int64, error) {
	if s == nil || s.scanResultRepo == nil {
		return nil, 0, errors.New("wallet policy contexts: scan repository not wired")
	}
	_ = ctx

	total, err := s.scanResultRepo.CountByUserID(userID)
	if err != nil {
		return nil, 0, err
	}
	entities, err := s.scanResultRepo.FindByUserID(userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	out := make([]WalletPolicyContextDTO, 0, len(entities))
	for _, e := range entities {
		if e == nil {
			continue
		}
		out = append(out, walletPolicyContextFromEntity(e))
	}
	return out, total, nil
}

func walletPolicyContextFromEntity(e *domain.ScanResultEntity) WalletPolicyContextDTO {
	networks := parseJSONStringArray(e.Networks)
	dto := WalletPolicyContextDTO{
		ScanID:           e.ID.String(),
		WalletAddress:    e.Address,
		WalletType:       string(e.Type),
		ChainIDs:         chainIDsFromNetworkNames(networks),
		CurrentPQPosture: pqPostureFromNIST(e.NISTLevel),
		Status:           normalizeWalletScanStatus(e.Status),
	}
	if !e.UpdatedAt.IsZero() {
		dto.ScannedAt = e.UpdatedAt.UTC().Format(time.RFC3339Nano)
	}
	return dto
}

func parseJSONStringArray(s string) []string {
	if s == "" || s == "[]" {
		return nil
	}
	var arr []string
	if err := json.Unmarshal([]byte(s), &arr); err != nil {
		return nil
	}
	return arr
}

func chainIDsFromNetworkNames(networks []string) []int {
	// Align with persisted network labels from scanners / Moralis-style names only.
	// Omit unknowns; never invent a default chain id.
	cm := map[string]int{
		"ethereum-mainnet": 1,
		"mainnet":          1,
		"ethereum":         1,
		"polygon":          137,
		"base":             8453,
		"arbitrum":         42161,
		"arbitrum-one":     42161,
		"optimism":         10,
		"bsc":              56,
		"avalanche":        43114,
		"sepolia":          11155111,
	}
	seen := make(map[int]struct{})
	var out []int
	for _, n := range networks {
		key := strings.ToLower(strings.TrimSpace(n))
		if id, ok := cm[key]; ok {
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}

func pqPostureFromNIST(n domain.NISTLevel) string {
	if n <= 1 {
		return "classical_only"
	}
	if n >= 5 {
		return "full_pq"
	}
	return "hybrid"
}

// normalizeWalletScanStatus maps persistence status to a coarse lifecycle for clients.
// "completed" matches CPM/scripts that filter eligible contexts.
func normalizeWalletScanStatus(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "SUCCESS":
		return "completed"
	case "PENDING", "RUNNING":
		return "running"
	case "FAILED", "TIMEOUT", "UNREACHABLE":
		return "failed"
	default:
		s := strings.ToLower(strings.TrimSpace(raw))
		if s == "" {
			return "unknown"
		}
		return s
	}
}
