package handler

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"cafe-discovery/internal/config"
	"cafe-discovery/internal/discoveryroutes"
	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/policyref"
	"cafe-discovery/internal/repository"
	"cafe-discovery/internal/service"
	"cafe-discovery/pkg/nats"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const (
	schemeHTTP  = "http://"
	schemeHTTPS = "https://"
)

// ScannerPresenceChecker is used to know if a scanner is available before accepting scan requests.
type ScannerPresenceChecker interface {
	HasScanner(scannerType string) bool
	ListScanners() []service.ScannerInfo
}

// DiscoveryHandler handles discovery-related HTTP requests.
// Wallet v1 list/get/delete and plan limits use Postgres as source of truth for wallet scans.
type DiscoveryHandler struct {
	discoveryService  *service.DiscoveryService
	tlsService        *service.TLSService
	cfgChain          *config.ChainConfig
	natsConn          nats.Connection
	planService       *service.PlanService
	scannerPresence   ScannerPresenceChecker
	redisWalletRepo   repository.RedisWalletScanRepository
	redisTLSRepo      repository.RedisTLSScanRepository
	userScanCache     *service.UserScanCacheService
	scanResultRepo    repository.ScanResultRepository
	tlsScanResultRepo repository.TLSScanResultRepository
	pendingV1         repository.PendingV1ScanRepository
	policyRef         policyref.Checker
}

// NewDiscoveryHandler creates a new discovery handler.
func NewDiscoveryHandler(discoveryService *service.DiscoveryService, tlsService *service.TLSService, cfgChain *config.ChainConfig, natsConn nats.Connection, planService *service.PlanService, scannerPresence ScannerPresenceChecker, redisWalletRepo repository.RedisWalletScanRepository, redisTLSRepo repository.RedisTLSScanRepository, userScanCache *service.UserScanCacheService, scanResultRepo repository.ScanResultRepository, tlsScanResultRepo repository.TLSScanResultRepository, pendingV1 repository.PendingV1ScanRepository, policyRef policyref.Checker) *DiscoveryHandler {
	return &DiscoveryHandler{
		discoveryService:  discoveryService,
		tlsService:        tlsService,
		cfgChain:          cfgChain,
		natsConn:          natsConn,
		planService:       planService,
		scannerPresence:   scannerPresence,
		redisWalletRepo:   redisWalletRepo,
		redisTLSRepo:      redisTLSRepo,
		userScanCache:     userScanCache,
		scanResultRepo:    scanResultRepo,
		tlsScanResultRepo: tlsScanResultRepo,
		pendingV1:         pendingV1,
		policyRef:         policyRef,
	}
}

// ScanRequest represents the request body for scanning a wallet or TLS endpoint
type ScanRequest struct {
	Address string `json:"address,omitempty"` // For wallet scans
	URL     string `json:"url,omitempty"`     // For TLS endpoint scans
}

// ListAvailableScanners returns the list of scanner types currently available (announced via NATS).
// GET /discovery/v1/scanners
func (h *DiscoveryHandler) ListAvailableScanners(c *fiber.Ctx) error {
	scanners := []service.ScannerInfo{}
	if h.scannerPresence != nil {
		scanners = h.scannerPresence.ListScanners()
	}
	return c.JSON(fiber.Map{"scanners": scanners})
}

// getAuthenticatedUserID extracts user ID from JWT context. Call only on routes protected by JWTMiddleware.
// Returns 401/403 with explicit error so the frontend can redirect to sign-in.
func (h *DiscoveryHandler) getAuthenticatedUserID(c *fiber.Ctx) (uuid.UUID, error) {
	userIDValue := c.Locals("user_id")
	if userIDValue == nil {
		return uuid.Nil, c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "sign in required to access this resource",
		})
	}
	userID, ok := userIDValue.(uuid.UUID)
	if !ok {
		return uuid.Nil, c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "invalid user id format",
		})
	}
	if userID == uuid.Nil {
		return uuid.Nil, c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "sign in required to access this resource",
		})
	}
	return userID, nil
}

// checkScanLimits validates scan limits using persisted scan execution rows (Postgres).
func (h *DiscoveryHandler) checkScanLimits(userID uuid.UUID, scanType string) (limitReached bool, errorMsg string, err error) {
	if h.planService == nil {
		return false, "", nil
	}
	canScan, usage, err := h.planService.CheckScanLimit(userID, scanType, h.scanResultRepo, h.tlsScanResultRepo)
	if err != nil {
		return false, "", err
	}
	if !canScan {
		return true, scanLimitErrorMessage(scanType, usage), nil
	}
	return false, "", nil
}

func scanLimitErrorMessage(scanType string, usage *service.PlanUsage) string {
	if scanType == "wallet" {
		return fmt.Sprintf("wallet scan limit reached (%d/%d). Please upgrade your plan to continue", usage.WalletScansUsed, usage.WalletScanLimit)
	}
	return fmt.Sprintf("endpoint scan limit reached (%d/%d). Please upgrade your plan to continue", usage.EndpointScansUsed, usage.EndpointScanLimit)
}

// queueScanError carries HTTP error details from tryQueue* helpers.
// committed is true when the response was already written (e.g. 401 from JWT context).
type queueScanError struct {
	committed bool
	status    int
	body      fiber.Map
}

// PostDiscoveryScanV1 handles POST /discovery/v1/scan (WORKPLAN_API.md §0.1, OpenAPI postScan).
// Body matches legacy POST /discovery/scan; acceptance returns scan_id, scan_family, status requested, and location.
func (h *DiscoveryHandler) PostDiscoveryScanV1(c *fiber.Ctx) error {
	var req ScanRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "invalid_request",
			"message": "invalid request body",
		})
	}
	if req.Address != "" && req.URL != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "invalid_request",
			"message": "cannot specify both address and url, please provide only one",
		})
	}
	if req.Address == "" && req.URL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "invalid_request",
			"message": "either address (for wallet) or url (for TLS endpoint) is required",
		})
	}
	if req.Address != "" {
		scanID, userID, normalized, qe := h.prepareWalletScanQueue(c, req.Address)
		if qe != nil {
			if qe.committed {
				return nil
			}
			return c.Status(qe.status).JSON(v1ErrorBody(qe.body))
		}
		if h.pendingV1 != nil {
			reserved, err := h.pendingV1.PutWallet(c.Context(), &repository.PendingV1ScanRecord{
				ScanID:    scanID,
				UserID:    userID,
				Family:    "wallet",
				Address:   normalized,
				CreatedAt: time.Now().UTC(),
			})
			if err != nil {
				log.Error().Err(err).Str("scan_id", scanID.String()).Msg("pending v1 wallet scan put failed before NATS")
				return c.Status(fiber.StatusServiceUnavailable).JSON(v1ErrorBody(fiber.Map{
					"error":   "service_unavailable",
					"message": "The scan could not be accepted; please try again.",
				}))
			}
			if !reserved {
				return c.Status(fiber.StatusConflict).JSON(v1ErrorBody(scanInProgressErrorBody()))
			}
		}
		if qe := h.publishWalletScanRequested(scanID, userID, normalized); qe != nil {
			if h.pendingV1 != nil {
				_ = h.pendingV1.Delete(c.Context(), scanID)
			}
			return c.Status(qe.status).JSON(v1ErrorBody(qe.body))
		}
		return c.JSON(postScanV1AcceptedJSON(scanID, "wallet"))
	}
	scanID, userID, endpoint, qe := h.prepareTLSScanQueue(c, req.URL)
	if qe != nil {
		if qe.committed {
			return nil
		}
		return c.Status(qe.status).JSON(v1ErrorBody(qe.body))
	}
	if h.pendingV1 != nil {
		if err := h.pendingV1.Put(c.Context(), &repository.PendingV1ScanRecord{
			ScanID:    scanID,
			UserID:    userID,
			Family:    "tls",
			Endpoint:  endpoint,
			CreatedAt: time.Now().UTC(),
		}); err != nil {
			log.Error().Err(err).Str("scan_id", scanID.String()).Msg("pending v1 TLS scan put failed before NATS")
			return c.Status(fiber.StatusServiceUnavailable).JSON(v1ErrorBody(fiber.Map{
				"error":   "service_unavailable",
				"message": "The scan could not be accepted; please try again.",
			}))
		}
	}
	if qe := h.publishTLSScanRequested(scanID, userID, endpoint); qe != nil {
		return c.Status(qe.status).JSON(v1ErrorBody(qe.body))
	}
	return c.JSON(postScanV1AcceptedJSON(scanID, "tls"))
}

func postScanV1AcceptedJSON(scanID uuid.UUID, family string) fiber.Map {
	var base string
	if family == "wallet" {
		base = discoveryroutes.EdgeWalletScans
	} else {
		base = discoveryroutes.EdgeTLSScans
	}
	return fiber.Map{
		"scan_id":     scanID.String(),
		"scan_family": family,
		"status":      "requested",
		"location":    base + scanID.String(),
	}
}

func v1ErrorBody(body fiber.Map) fiber.Map {
	if body == nil {
		return fiber.Map{"error": "error", "message": "request failed"}
	}
	if msg, ok := body["message"].(string); ok {
		if code, ok := body["error"].(string); ok {
			return fiber.Map{"error": code, "message": msg}
		}
		return fiber.Map{"error": "error", "message": msg}
	}
	if errStr, ok := body["error"].(string); ok {
		return fiber.Map{"error": "error", "message": errStr}
	}
	return fiber.Map{"error": "error", "message": "request failed"}
}

// prepareWalletScanQueue validates and allocates scan_id; does not publish to NATS.
func (h *DiscoveryHandler) prepareWalletScanQueue(c *fiber.Ctx, address string) (scanID uuid.UUID, userID uuid.UUID, normalized string, qe *queueScanError) {
	userID, err := h.getAuthenticatedUserID(c)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", &queueScanError{committed: true}
	}

	if address == "" {
		return uuid.Nil, uuid.Nil, "", &queueScanError{
			status: fiber.StatusBadRequest,
			body:   fiber.Map{"error": "address is required"},
		}
	}

	if h.scannerPresence != nil && !h.scannerPresence.HasScanner("wallet") {
		return uuid.Nil, uuid.Nil, "", &queueScanError{
			status: fiber.StatusServiceUnavailable,
			body:   fiber.Map{"error": "no wallet scanner available, please try again later"},
		}
	}

	limitReached, limitMsg, err := h.checkScanLimits(userID, "wallet")
	if err != nil {
		return uuid.Nil, uuid.Nil, "", &queueScanError{
			status: fiber.StatusInternalServerError,
			body:   fiber.Map{"error": fmt.Sprintf("failed to check plan limits: %v", err)},
		}
	}
	if limitReached {
		return uuid.Nil, uuid.Nil, "", &queueScanError{
			status: fiber.StatusForbidden,
			body:   fiber.Map{"error": limitMsg},
		}
	}

	normalizedAddress, err := h.discoveryService.ValidateAndNormalizeAddress(address)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", &queueScanError{
			status: fiber.StatusBadRequest,
			body:   fiber.Map{"error": err.Error()},
		}
	}

	if qe := h.checkWalletScanInFlight(c.Context(), userID, normalizedAddress); qe != nil {
		return uuid.Nil, uuid.Nil, "", qe
	}

	return uuid.New(), userID, normalizedAddress, nil
}

func (h *DiscoveryHandler) checkWalletScanInFlight(ctx context.Context, userID uuid.UUID, address string) *queueScanError {
	if h.pendingV1 != nil {
		rec, err := h.pendingV1.GetWalletByOwnerAddress(ctx, userID, address)
		if err != nil {
			return &queueScanError{
				status: fiber.StatusServiceUnavailable,
				body: fiber.Map{
					"error":   "service_unavailable",
					"message": "wallet scan queue state is temporarily unavailable",
				},
			}
		}
		if rec != nil {
			block, stale, qe := h.pendingWalletReservationState(ctx, userID, address, rec)
			if qe != nil {
				return qe
			}
			if block {
				return scanInProgressQueueError()
			}
			if stale {
				_ = h.pendingV1.DeleteWalletReservation(ctx, userID, address, rec.ScanID)
			}
		}
	}

	if h.scanResultRepo == nil {
		return nil
	}
	rows, err := h.scanResultRepo.ListOwnerWalletScansByAddress(userID, address)
	if err != nil {
		return &queueScanError{
			status: fiber.StatusInternalServerError,
			body: fiber.Map{
				"error":   "internal_error",
				"message": err.Error(),
			},
		}
	}
	if len(rows) > 0 && walletScanEntityIsInFlight(rows[0]) {
		return scanInProgressQueueError()
	}
	return nil
}

func (h *DiscoveryHandler) pendingWalletReservationState(ctx context.Context, userID uuid.UUID, address string, rec *repository.PendingV1ScanRecord) (block bool, stale bool, qe *queueScanError) {
	if rec.ScanID == uuid.Nil {
		return false, true, nil
	}
	if h.scanResultRepo != nil {
		ent, err := h.scanResultRepo.FindOwnedWalletScanByID(userID, rec.ScanID)
		if err != nil {
			return false, false, &queueScanError{
				status: fiber.StatusInternalServerError,
				body: fiber.Map{
					"error":   "internal_error",
					"message": err.Error(),
				},
			}
		}
		if ent != nil {
			return walletScanEntityIsInFlight(ent), !walletScanEntityIsInFlight(ent), nil
		}
	}
	if rec.CreatedAt.IsZero() {
		return false, true, nil
	}
	if rec.UserID == userID && rec.Family == "wallet" && strings.EqualFold(rec.Address, address) {
		return true, false, nil
	}
	return false, true, nil
}

func walletScanEntityIsInFlight(e *domain.ScanResultEntity) bool {
	if e == nil {
		return false
	}
	st := walletScanLifecycleStatusV1(e.Status)
	return st == "requested" || st == "started"
}

func scanInProgressQueueError() *queueScanError {
	return &queueScanError{
		status: fiber.StatusConflict,
		body:   scanInProgressErrorBody(),
	}
}

func scanInProgressErrorBody() fiber.Map {
	return fiber.Map{
		"error":   "SCAN_IN_PROGRESS",
		"message": "A wallet scan is already in progress for this target.",
	}
}

func (h *DiscoveryHandler) publishWalletScanRequested(scanID, userID uuid.UUID, normalized string) *queueScanError {
	scanMsg := nats.WalletScanMessage{
		ScanID:  scanID,
		UserID:  userID,
		Address: normalized,
	}
	log.Info().
		Str("subject", nats.SubjectScanRequestedWallet).
		Str("scan_id", scanMsg.ScanID.String()).
		Str("address", normalized).
		Str("component", "backend").
		Msg("NATS → PUB scan.requested.wallet")
	if err := nats.PublishJSON(h.natsConn, nats.SubjectScanRequestedWallet, scanMsg); err != nil {
		return &queueScanError{
			status: fiber.StatusInternalServerError,
			body:   fiber.Map{"error": "failed to queue scan request"},
		}
	}
	return nil
}

// prepareTLSScanQueue validates and allocates scan_id; does not publish to NATS.
func (h *DiscoveryHandler) prepareTLSScanQueue(c *fiber.Ctx, endpointURL string) (scanID uuid.UUID, userID uuid.UUID, endpoint string, qe *queueScanError) {
	userID, err := h.getAuthenticatedUserID(c)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", &queueScanError{committed: true}
	}

	if h.scannerPresence != nil && !h.scannerPresence.HasScanner("tls") {
		return uuid.Nil, uuid.Nil, "", &queueScanError{
			status: fiber.StatusServiceUnavailable,
			body:   fiber.Map{"error": "no TLS scanner available, please try again later"},
		}
	}

	if !strings.HasPrefix(endpointURL, schemeHTTPS) && !strings.HasPrefix(endpointURL, schemeHTTP) {
		return uuid.Nil, uuid.Nil, "", &queueScanError{
			status: fiber.StatusBadRequest,
			body:   fiber.Map{"error": "url must use http:// or https:// protocol"},
		}
	}

	parsedURL, err := url.Parse(endpointURL)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", &queueScanError{
			status: fiber.StatusBadRequest,
			body:   fiber.Map{"error": fmt.Sprintf("invalid URL format: %v", err)},
		}
	}

	if parsedURL.Host == "" {
		return uuid.Nil, uuid.Nil, "", &queueScanError{
			status: fiber.StatusBadRequest,
			body:   fiber.Map{"error": "url must include a valid hostname"},
		}
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		return uuid.Nil, uuid.Nil, "", &queueScanError{
			status: fiber.StatusBadRequest,
			body:   fiber.Map{"error": "url must include a valid hostname"},
		}
	}

	limitReached, limitMsg, err := h.checkScanLimits(userID, "endpoint")
	if err != nil {
		return uuid.Nil, uuid.Nil, "", &queueScanError{
			status: fiber.StatusInternalServerError,
			body:   fiber.Map{"error": fmt.Sprintf("failed to check plan limits: %v", err)},
		}
	}
	if limitReached {
		return uuid.Nil, uuid.Nil, "", &queueScanError{
			status: fiber.StatusForbidden,
			body:   fiber.Map{"error": limitMsg},
		}
	}

	return uuid.New(), userID, endpointURL, nil
}

func (h *DiscoveryHandler) publishTLSScanRequested(scanID, userID uuid.UUID, endpointURL string) *queueScanError {
	scanMsg := nats.TLSScanMessage{
		ScanID:   scanID,
		UserID:   userID,
		Endpoint: endpointURL,
	}
	log.Info().
		Str("subject", nats.SubjectScanRequestedTLS).
		Str("scan_id", scanMsg.ScanID.String()).
		Str("endpoint", scanMsg.Endpoint).
		Str("component", "backend").
		Msg("NATS → PUB scan.requested.tls")
	if err := nats.PublishJSON(h.natsConn, nats.SubjectScanRequestedTLS, scanMsg); err != nil {
		return &queueScanError{
			status: fiber.StatusInternalServerError,
			body:   fiber.Map{"error": "failed to queue scan request"},
		}
	}
	return nil
}

// ListRPCs handles GET /discovery/v1/rpcs.
// Returns the list of configured RPC endpoints.
func (h *DiscoveryHandler) ListRPCs(c *fiber.Ctx) error {
	rpcs := make([]fiber.Map, 0, len(h.cfgChain.Blockchains))
	for _, blockchain := range h.cfgChain.Blockchains {
		rpcs = append(rpcs, fiber.Map{
			"name": blockchain.Name,
			"rpc":  blockchain.RPC,
		})
	}

	return c.JSON(fiber.Map{
		"blockchains": rpcs,
		"count":       len(rpcs),
	})
}
