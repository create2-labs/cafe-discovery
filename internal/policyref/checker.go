package policyref

import (
	"context"

	"github.com/google/uuid"
)

// Checker asks CPM whether any persisted policy instances for this owner reference scan_id
// (POST /internal/policies/references/scan, WORKPLAN_API_PR PR5/6).
// When nil, Discovery fails closed on scan delete (503).
type Checker interface {
	PersistedPoliciesReferenceScan(ctx context.Context, userID uuid.UUID, tenantID string, scanID uuid.UUID) (referenced bool, err error)
}
