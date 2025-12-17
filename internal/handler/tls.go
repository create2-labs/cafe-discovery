package handler

import (
	"strings"

	"cafe-discovery/internal/service"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// TLSHandler handles TLS-related HTTP requests
type TLSHandler struct {
	tlsService *service.TLSService
}

// NewTLSHandler creates a new TLS handler
func NewTLSHandler(tlsService *service.TLSService) *TLSHandler {
	return &TLSHandler{
		tlsService: tlsService,
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

	// Validate URL format (should start with https://)
	if !strings.HasPrefix(req.URL, "https://") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "url must use https:// protocol",
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

	result, err := h.tlsService.ScanTLS(c.Context(), userID, req.URL)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(result)
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
