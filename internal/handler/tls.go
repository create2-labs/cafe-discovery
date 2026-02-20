package handler

import (
	"fmt"
	"net/url"
	"strings"

	"cafe-discovery/internal/repository"
	"cafe-discovery/internal/service"
	"cafe-discovery/pkg/nats"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// TLSHandler handles TLS-related HTTP requests. Scan list uses read-through (Redis then Postgres).
type TLSHandler struct {
	tlsService    *service.TLSService
	natsConn      nats.Connection
	redisTLSRepo  repository.RedisTLSScanRepository
	planService   *service.PlanService
	userScanCache *service.UserScanCacheService
}

// NewTLSHandler creates a new TLS handler (read-through for scan list).
func NewTLSHandler(tlsService *service.TLSService, natsConn nats.Connection, redisTLSRepo repository.RedisTLSScanRepository, planService *service.PlanService, userScanCache *service.UserScanCacheService) *TLSHandler {
	return &TLSHandler{
		tlsService:    tlsService,
		natsConn:      natsConn,
		redisTLSRepo:  redisTLSRepo,
		planService:   planService,
		userScanCache: userScanCache,
	}
}

// ScanRequest represents the request body for scanning a TLS endpoint
type TLSScanRequest struct {
	URL string `json:"url"`
}

// Scan handles POST /discovery/scan/endpoints
func (h *TLSHandler) Scan(c *fiber.Ctx) error {
	var req TLSScanRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	userID, err := requireAuthenticatedUserID(c)
	if err != nil {
		return err
	}

	if req.URL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "url is required",
		})
	}

	// Validate URL format (should start with https:// and be a valid URL)
	if !strings.HasPrefix(req.URL, "https://") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "url must use https:// protocol",
		})
	}

	// Validate that the URL is well-formed and can be parsed
	// This catches issues like invalid hostnames before they reach the scanner
	parsedURL, err := url.Parse(req.URL)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("invalid URL format: %v", err),
		})
	}

	// Check that the URL has a valid hostname
	if parsedURL.Host == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "url must include a valid hostname",
		})
	}

	// Basic validation: hostname should not be empty after parsing
	hostname := parsedURL.Hostname()
	if hostname == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "url must include a valid hostname",
		})
	}

	if h.planService != nil {
		endpointCount, _ := h.redisTLSRepo.CountByUserID(c.Context(), userID.String())
		canScan, usage, err := h.planService.CheckScanLimitFromCounts(userID, "endpoint", 0, endpointCount)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("failed to check plan limits: %v", err)})
		}
		if !canScan {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": fmt.Sprintf("endpoint scan limit reached (%d/%d). Please upgrade your plan to continue", usage.EndpointScansUsed, usage.EndpointScanLimit),
			})
		}
	}

	// Publish scan request to NATS (scanners subscribe to scan.requested.tls)
	scanMsg := nats.TLSScanMessage{
		ScanID:   uuid.New(),
		UserID:   userID,
		Endpoint: req.URL,
	}
	log.Info().
		Str("subject", nats.SubjectScanRequestedTLS).
		Str("scan_id", scanMsg.ScanID.String()).
		Str("endpoint", scanMsg.Endpoint).
		Str("component", "backend").
		Msg("NATS → PUB scan.requested.tls")
	if err := nats.PublishJSON(h.natsConn, nats.SubjectScanRequestedTLS, scanMsg); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to queue scan request",
		})
	}

	// Return immediate response - scan will be processed asynchronously
	return c.JSON(fiber.Map{
		"message":  "scan queued successfully",
		"endpoint": req.URL,
		"status":   "processing",
	})
}

// ListScans handles GET /discovery/tls/scans. Read-through: Redis then Postgres (user + default endpoints).
func (h *TLSHandler) ListScans(c *fiber.Ctx) error {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		return err
	}
	limit, offset := parsePaginationParams(c)
	urls, total, err := h.userScanCache.ListTLSURLs(c.Context(), userID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	results := make([]fiber.Map, len(urls))
	for i, u := range urls {
		results[i] = fiber.Map{"id": u}
	}
	return c.JSON(fiber.Map{
		"results": results,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
		"count":   len(results),
	})
}
