package handler

import (
	"cafe-discovery/internal/persistence/scanread"
	"cafe-discovery/internal/policyref"
	"cafe-discovery/internal/repository"
)

// TLSHandler handles TLS-related HTTP requests.
type TLSHandler struct {
	scanRead  scanread.Store
	pendingV1 repository.PendingV1ScanRepository
	policyRef policyref.Checker
}

// NewTLSHandler creates a new TLS handler.
func NewTLSHandler(scanRead scanread.Store, pendingV1 repository.PendingV1ScanRepository, policyRef policyref.Checker) *TLSHandler {
	return &TLSHandler{
		scanRead:  scanRead,
		pendingV1: pendingV1,
		policyRef: policyRef,
	}
}
