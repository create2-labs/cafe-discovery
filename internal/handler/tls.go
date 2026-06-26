package handler

import (
	"cafe-discovery/internal/persistence/scanpending"
	"cafe-discovery/internal/persistence/scanread"
	"cafe-discovery/internal/policyref"
)

// TLSHandler handles TLS-related HTTP requests.
type TLSHandler struct {
	scanRead    scanread.Store
	scanPending scanpending.Store
	policyRef   policyref.Checker
}

// NewTLSHandler creates a new TLS handler.
func NewTLSHandler(scanRead scanread.Store, scanPending scanpending.Store, policyRef policyref.Checker) *TLSHandler {
	return &TLSHandler{
		scanRead:    scanRead,
		scanPending: scanPending,
		policyRef:   policyRef,
	}
}
