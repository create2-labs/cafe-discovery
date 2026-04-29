package handler

import (
	"testing"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/service"
)

func TestScanResultToCBOMNormalizesAddress(t *testing.T) {
	t.Parallel()

	h := &DiscoveryHandler{
		discoveryService: service.NewDiscoveryService(nil, nil, nil, nil),
	}
	scan := &domain.ScanResult{
		Address: "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
	}

	cbom := h.scanResultToCBOM(scan)
	got, ok := cbom["address"].(string)
	if !ok {
		t.Fatalf("address field missing or not a string")
	}
	want := "0x742d35cc6634c0532925a3b844bc454e4438f44e"
	if got != want {
		t.Fatalf("address = %q, want %q", got, want)
	}
}
