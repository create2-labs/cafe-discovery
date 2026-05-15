package handler

import (
	"cafe-discovery/internal/policyref"
	"cafe-discovery/internal/repository"
	"cafe-discovery/internal/service"
	"cafe-discovery/pkg/nats"
)

// TLSHandler handles TLS-related HTTP requests. Scan list uses read-through (Redis then Postgres).
type TLSHandler struct {
	tlsService        *service.TLSService
	natsConn          nats.Connection
	redisTLSRepo      repository.RedisTLSScanRepository
	planService       *service.PlanService
	userScanCache     *service.UserScanCacheService
	tlsScanResultRepo repository.TLSScanResultRepository
	pendingV1         repository.PendingV1ScanRepository
	policyRef         policyref.Checker
}

// NewTLSHandler creates a new TLS handler (read-through for scan list).
func NewTLSHandler(tlsService *service.TLSService, natsConn nats.Connection, redisTLSRepo repository.RedisTLSScanRepository, planService *service.PlanService, userScanCache *service.UserScanCacheService, tlsScanResultRepo repository.TLSScanResultRepository, pendingV1 repository.PendingV1ScanRepository, policyRef policyref.Checker) *TLSHandler {
	return &TLSHandler{
		tlsService:        tlsService,
		natsConn:          natsConn,
		redisTLSRepo:      redisTLSRepo,
		planService:       planService,
		userScanCache:     userScanCache,
		tlsScanResultRepo: tlsScanResultRepo,
		pendingV1:         pendingV1,
		policyRef:         policyRef,
	}
}
