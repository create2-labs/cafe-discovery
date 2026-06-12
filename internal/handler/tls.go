package handler

import (
	"cafe-discovery/internal/policyref"
	"cafe-discovery/internal/repository"
)

// TLSHandler handles TLS-related HTTP requests.
type TLSHandler struct {
	redisTLSRepo      repository.RedisTLSScanRepository
	tlsScanResultRepo repository.TLSScanResultRepository
	pendingV1         repository.PendingV1ScanRepository
	policyRef         policyref.Checker
}

// NewTLSHandler creates a new TLS handler.
func NewTLSHandler(redisTLSRepo repository.RedisTLSScanRepository, tlsScanResultRepo repository.TLSScanResultRepository, pendingV1 repository.PendingV1ScanRepository, policyRef policyref.Checker) *TLSHandler {
	return &TLSHandler{
		redisTLSRepo:      redisTLSRepo,
		tlsScanResultRepo: tlsScanResultRepo,
		pendingV1:         pendingV1,
		policyRef:         policyRef,
	}
}
