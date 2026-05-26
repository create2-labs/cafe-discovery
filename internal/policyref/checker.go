package policyref

import (
	"context"

	"github.com/google/uuid"
)

// WalletTargetContext is the minimal IMM-9b lookup result for a normalized wallet target_address.
type WalletTargetContext struct {
	Exists      bool
	PolicyCount int
	DraftCount  int
}

// Checker asks CPM internal reference endpoints (service token).
// When nil, Discovery fails closed on scan delete and wallet POST /scan W1 (503).
type Checker interface {
	PersistedPoliciesReferenceScan(ctx context.Context, userID uuid.UUID, tenantID string, scanID uuid.UUID) (referenced bool, err error)
	ActiveWalletCPMContextForTarget(ctx context.Context, userID uuid.UUID, tenantID string, normalizedTargetAddress string) (WalletTargetContext, error)
}
