package handler

import (
	"cafe-discovery/internal/repository"
	"cafe-discovery/internal/service"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// PlanHandler handles plan-related HTTP requests
type PlanHandler struct {
	planService      *service.PlanService
	scanResultRepo   repository.ScanResultRepository
	tlsScanResultRepo repository.TLSScanResultRepository
}

// NewPlanHandler creates a new plan handler
func NewPlanHandler(planService *service.PlanService, scanResultRepo repository.ScanResultRepository, tlsScanResultRepo repository.TLSScanResultRepository) *PlanHandler {
	return &PlanHandler{
		planService:      planService,
		scanResultRepo:   scanResultRepo,
		tlsScanResultRepo: tlsScanResultRepo,
	}
}

// GetUserPlan handles GET /plans/current
func (h *PlanHandler) GetUserPlan(c *fiber.Ctx) error {
	// Get user ID from JWT context
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

	plan, err := h.planService.GetUserPlan(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(plan)
}

// GetAllPlans handles GET /plans
func (h *PlanHandler) GetAllPlans(c *fiber.Ctx) error {
	plans, err := h.planService.GetAllPlans()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"plans": plans,
		"count": len(plans),
	})
}

// GetPlanUsage handles GET /plans/usage
func (h *PlanHandler) GetPlanUsage(c *fiber.Ctx) error {
	// Get user ID from JWT context
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

	usage, err := h.planService.GetPlanUsage(userID, h.scanResultRepo, h.tlsScanResultRepo)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(usage)
}

