package handler

import (
	"testing"
	"time"

	"cafe-discovery/internal/config"
	"cafe-discovery/internal/domain"

	contracts "github.com/create2-labs/cafe-contracts/cafenatsv01"
	"github.com/google/uuid"
)

func TestBuildPolicyAssessmentRequestedEvent_DeterministicIDsAndValidContract(t *testing.T) {
	h := &DiscoveryHandler{
		cfgChain: &config.ChainConfig{
			Blockchains: []config.Blockchain{
				{Name: "ethereum", ChainID: 1},
				{Name: "polygon", ChainID: 137},
			},
		},
	}
	userID := uuid.MustParse("7b69cb08-6f46-48f9-a2f7-11f5f212f163")
	scannedAt := time.Date(2026, 4, 27, 16, 0, 0, 0, time.UTC)
	scan := &domain.ScanResult{
		Address:         "0x1234567890123456789012345678901234567890",
		Type:            domain.AccountTypeEOA,
		Algorithm:       domain.AlgorithmECDSAsecp256k1,
		NISTLevel:       domain.NISTLevel1,
		KeyExposed:      true,
		TransactionHash: "0xabc",
		Networks:        []string{"ethereum", "polygon"},
		ScannedAt:       scannedAt,
	}
	selection := contracts.PolicySelectionRequestWire{
		TargetPosture:             contracts.TargetPostureHybrid,
		TargetChainIDs:            []int64{137, 1, 1},
		AllowNewWallet:            true,
		AddressContinuityRequired: true,
		ApprovalMode:              "manual",
	}

	ev1, subject1, err := h.buildPolicyAssessmentRequestedEvent(userID, scan.Address, scan, selection, "req-42")
	if err != nil {
		t.Fatalf("buildPolicyAssessmentRequestedEvent() error = %v", err)
	}
	ev2, subject2, err := h.buildPolicyAssessmentRequestedEvent(userID, scan.Address, scan, selection, "req-42")
	if err != nil {
		t.Fatalf("buildPolicyAssessmentRequestedEvent() second call error = %v", err)
	}

	if subject1 != contracts.NATSSubjectPolicyAssessmentRequestedV01 || subject2 != subject1 {
		t.Fatalf("unexpected subject: got %q / %q", subject1, subject2)
	}
	if ev1.EventID != ev2.EventID || ev1.CorrelationID != ev2.CorrelationID || ev1.CausationID != ev2.CausationID {
		t.Fatalf("expected deterministic IDs, got different values")
	}
	if ev1.Payload.SelectionRequest.TargetChainIDs[0] != 1 || ev1.Payload.SelectionRequest.TargetChainIDs[1] != 137 {
		t.Fatalf("expected normalized sorted target_chain_ids, got %#v", ev1.Payload.SelectionRequest.TargetChainIDs)
	}
	if err := ev1.Validate(); err != nil {
		t.Fatalf("event should validate, got error: %v", err)
	}
	if ev1.Payload.Observation.EventType != contracts.EventTypeDiscoveryWalletObserved {
		t.Fatalf("unexpected embedded observation event_type: %s", ev1.Payload.Observation.EventType)
	}
}

func TestBuildPolicyAssessmentRequestedEvent_ChangesIDsWhenSelectionChanges(t *testing.T) {
	h := &DiscoveryHandler{
		cfgChain: &config.ChainConfig{
			Blockchains: []config.Blockchain{{Name: "ethereum", ChainID: 1}},
		},
	}
	userID := uuid.MustParse("7b69cb08-6f46-48f9-a2f7-11f5f212f163")
	scan := &domain.ScanResult{
		Address:         "0x1234567890123456789012345678901234567890",
		Type:            domain.AccountTypeEOA,
		Algorithm:       domain.AlgorithmECDSAsecp256k1,
		NISTLevel:       domain.NISTLevel1,
		KeyExposed:      true,
		TransactionHash: "0xabc",
		Networks:        []string{"ethereum"},
		ScannedAt:       time.Date(2026, 4, 27, 16, 0, 0, 0, time.UTC),
	}

	baseSelection := contracts.PolicySelectionRequestWire{
		TargetPosture: contracts.TargetPostureHybrid,
		ApprovalMode:  "manual",
	}
	changedSelection := contracts.PolicySelectionRequestWire{
		TargetPosture: contracts.TargetPostureFullPQ,
		ApprovalMode:  "manual",
	}

	ev1, _, err := h.buildPolicyAssessmentRequestedEvent(userID, scan.Address, scan, baseSelection, "req-42")
	if err != nil {
		t.Fatalf("first event build failed: %v", err)
	}
	ev2, _, err := h.buildPolicyAssessmentRequestedEvent(userID, scan.Address, scan, changedSelection, "req-42")
	if err != nil {
		t.Fatalf("second event build failed: %v", err)
	}
	if ev1.EventID == ev2.EventID {
		t.Fatalf("expected event_id to change when selection_request changes")
	}
}
