package handler

import (
	"fmt"
	"strings"

	"cafe-discovery/internal/config"
	"cafe-discovery/internal/repository"
	"cafe-discovery/internal/service"
	"cafe-discovery/pkg/nats"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// DiscoveryHandler handles discovery-related HTTP requests
type DiscoveryHandler struct {
	discoveryService *service.DiscoveryService
	cfgChain         *config.ChainConfig
	natsConn         nats.Connection
	redisScanRepo    repository.RedisWalletScanRepository
	planService      *service.PlanService
}

// NewDiscoveryHandler creates a new discovery handler
func NewDiscoveryHandler(discoveryService *service.DiscoveryService, cfgChain *config.ChainConfig, natsConn nats.Connection, redisScanRepo repository.RedisWalletScanRepository, planService *service.PlanService) *DiscoveryHandler {
	return &DiscoveryHandler{
		discoveryService: discoveryService,
		cfgChain:         cfgChain,
		natsConn:         natsConn,
		redisScanRepo:    redisScanRepo,
		planService:      planService,
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
	// For anonymous users, userID will be uuid.Nil
	userIDValue := c.Locals("user_id")
	var userID uuid.UUID
	var isAnonymous bool

	if userIDValue == nil {
		// Anonymous user - use uuid.Nil
		userID = uuid.Nil
		isAnonymous = true
	} else {
		var ok bool
		userID, ok = userIDValue.(uuid.UUID)
		if !ok {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "invalid user ID format",
			})
		}
		isAnonymous = userID == uuid.Nil
	}

	// For anonymous users, check scan limit in Redis
	if isAnonymous {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "missing authorization header for anonymous scan",
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

		// Check anonymous scan limit
		count, err := h.redisScanRepo.Count(c.Context(), tokenHash)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("failed to check scan limit: %v", err),
			})
		}

		maxScans := repository.GetMaxAnonymousWalletScans()
		if count >= maxScans {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": fmt.Sprintf("anonymous wallet scan limit reached (%d/%d). Please sign in to continue", count, maxScans),
			})
		}
	} else {
		// For authenticated users, check plan limits
		if h.planService != nil {
			canScan, usage, err := h.planService.CheckScanLimit(userID, "wallet", nil, nil)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("failed to check plan limits: %v", err),
				})
			}
			if !canScan {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"error": fmt.Sprintf("wallet scan limit reached (%d/%d). Please upgrade your plan to continue", usage.WalletScansUsed, usage.WalletScanLimit),
				})
			}
		}
	}

	// Validate and normalize the Ethereum address before queuing
	// This ensures we return an error immediately if the address is invalid
	normalizedAddress, err := h.discoveryService.ValidateAndNormalizeAddress(req.Address)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Publish scan request to NATS for async processing
	scanMsg := nats.WalletScanMessage{
		UserID:  userID,
		Address: normalizedAddress,
	}

	var subject string
	if isAnonymous {
		// Anonymous users: use Redis queue
		// Extract token from Authorization header for anonymous users to create unique Redis keys
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				scanMsg.Token = parts[1]
			}
		}
		subject = nats.SubjectWalletScanAnonymous
	} else {
		// Authenticated users: use PostgreSQL queue
		subject = nats.SubjectWalletScan
	}

	if err := nats.PublishJSON(h.natsConn, subject, scanMsg); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to queue scan request",
		})
	}

	// Return immediate response - scan will be processed asynchronously
	return c.JSON(fiber.Map{
		"message": "scan queued successfully",
		"address": normalizedAddress,
		"status":  "processing",
	})
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

// ListAnonymousScans handles GET /discovery/scans/anonymous
// Returns the list of anonymous wallet scan results from Redis for the current user's token
func (h *DiscoveryHandler) ListAnonymousScans(c *fiber.Ctx) error {
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

	return c.JSON(fiber.Map{
		"results": anonymousResults,
		"total":   len(anonymousResults),
		"count":   len(anonymousResults),
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
