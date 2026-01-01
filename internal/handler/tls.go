package handler

import (
	"fmt"
	"net/url"
	"strings"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"
	"cafe-discovery/internal/service"
	"cafe-discovery/pkg/nats"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// TLSHandler handles TLS-related HTTP requests
type TLSHandler struct {
	tlsService        *service.TLSService
	natsConn          nats.Connection
	tlsScanResultRepo repository.TLSScanResultRepository
	redisScanRepo     repository.RedisTLSScanRepository
	planService       *service.PlanService
}

// NewTLSHandler creates a new TLS handler
func NewTLSHandler(tlsService *service.TLSService, natsConn nats.Connection, tlsScanResultRepo repository.TLSScanResultRepository, planService *service.PlanService, redisScanRepo repository.RedisTLSScanRepository) *TLSHandler {
	return &TLSHandler{
		tlsService:        tlsService,
		natsConn:          natsConn,
		tlsScanResultRepo: tlsScanResultRepo,
		redisScanRepo:     redisScanRepo,
		planService:       planService,
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
	// This catches issues like invalid hostnames before they reach the worker
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

	// Get user ID from JWT context (set by middleware)
	userIDValue := c.Locals("user_id")
	if userIDValue == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "user not authenticated",
		})
	}

	userID, ok := userIDValue.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "invalid user ID format",
		})
	}

	// Check plan limits before queuing the scan
	// This ensures we return an error immediately to the frontend if limits are reached
	if h.planService != nil {
		canScan, usage, err := h.planService.CheckScanLimit(userID, "endpoint", nil, h.tlsScanResultRepo)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("failed to check plan limits: %v", err),
			})
		}
		if !canScan {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": fmt.Sprintf("endpoint scan limit reached (%d/%d). Please upgrade your plan to continue", usage.EndpointScansUsed, usage.EndpointScanLimit),
			})
		}
	}

	// Publish scan request to NATS for async processing
	// Anonymous users (uuid.Nil) go to a different queue for Redis storage
	scanMsg := nats.TLSScanMessage{
		UserID:   userID,
		Endpoint: req.URL,
	}

	var subject string
	if userID == uuid.Nil {
		// Anonymous users: use Redis queue
		// Extract token from Authorization header for anonymous users to create unique Redis keys
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				scanMsg.Token = parts[1]
			}
		}
		subject = nats.SubjectTLSScanAnonymous
	} else {
		// Authenticated users: use PostgreSQL queue
		subject = nats.SubjectTLSScan
	}

	if err := nats.PublishJSON(h.natsConn, subject, scanMsg); err != nil {
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

// ListScans handles GET /discovery/tls/scans
// Returns the list of TLS scan results for the authenticated user with pagination
func (h *TLSHandler) ListScans(c *fiber.Ctx) error {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		return err
	}

	limit, offset := parsePaginationParams(c)

	// Get TLS scan results from service
	results, total, err := h.tlsService.ListTLSScanResults(c.Context(), userID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"results": results,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
		"count":   len(results),
	})
}

// ListAnonymousScans handles GET /discovery/tls/scans/anonymous
// Returns the list of anonymous TLS scan results from Redis for the current user's token
// Also includes default endpoints that are visible to everyone
func (h *TLSHandler) ListAnonymousScans(c *fiber.Ctx) error {
	// Extract token from Authorization header to get the unique session identifier
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "missing authorization header",
		})
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid authorization header format",
		})
	}

	token := parts[1]
	tokenHash := repository.HashToken(token)

	// Get anonymous scans from Redis for this specific token
	anonymousResults, err := h.redisScanRepo.ListAll(c.Context(), tokenHash)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to fetch anonymous scans: %v", err),
		})
	}

	// Get default endpoints from database (visible to everyone)
	defaultEntities, err := h.tlsScanResultRepo.FindAllDefault()
	if err != nil {
		// Log error but don't fail - continue without default endpoints
		_ = err
		defaultEntities = []*domain.TLSScanResultEntity{}
	}

	// Convert default entities to domain results
	defaultResults := make([]*domain.TLSScanResult, len(defaultEntities))
	for i, entity := range defaultEntities {
		defaultResults[i] = entity.ToTLSScanResult()
	}

	// Combine anonymous scans and default endpoints
	allResults := append(anonymousResults, defaultResults...)

	return c.JSON(fiber.Map{
		"results": allResults,
		"total":   len(allResults),
		"count":   len(allResults),
	})
}
