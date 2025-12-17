package handler

import (
	"cafe-discovery/internal/config"
	"cafe-discovery/internal/service"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// DiscoveryHandler handles discovery-related HTTP requests
type DiscoveryHandler struct {
	discoveryService *service.DiscoveryService
	cfgChain         *config.ChainConfig
}

// NewDiscoveryHandler creates a new discovery handler
func NewDiscoveryHandler(discoveryService *service.DiscoveryService, cfgChain *config.ChainConfig) *DiscoveryHandler {
	return &DiscoveryHandler{
		discoveryService: discoveryService,
		cfgChain:         cfgChain,
	}
}

// ScanRequest represents the request body for scanning a wallet
type ScanRequest struct {
	Address string `json:"address"`
}

// Scan handles POST /discovery/scan
func (h *DiscoveryHandler) Scan(c *fiber.Ctx) error {
	var req ScanRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.Address == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "address is required",
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

	result, err := h.discoveryService.ScanWallet(c.Context(), userID, req.Address)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(result)
}

// ListRPCs handles GET /discovery/rpcs
// Returns the list of configured RPC endpoints
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

// ListScans handles GET /discovery/scans
// Returns the list of scan results for the authenticated user with pagination
func (h *DiscoveryHandler) ListScans(c *fiber.Ctx) error {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		return err
	}

	limit, offset := parsePaginationParams(c)

	// Get scan results from service
	results, total, err := h.discoveryService.ListScanResults(c.Context(), userID, limit, offset)
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
