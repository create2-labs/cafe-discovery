package handlers

import (
	"cafe-discovery/internal/domain"
	"cafe-discovery/pkg/nats"
)

// CommitWalletCompletionForIntegrationTest exposes commitWalletCompletion for IMM-6b-8 cross-package tests.
func (h *ScanEventHandler) CommitWalletCompletionForIntegrationTest(
	msg *nats.ScanCompletedMessage,
	entity *domain.ScanResultEntity,
	result *domain.ScanResult,
) (bool, error) {
	return h.commitWalletCompletion(msg, entity, result)
}
