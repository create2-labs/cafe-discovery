package handler

import (
	"cafe-discovery/internal/repository"
	"cafe-discovery/internal/service"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// PlanHandler handles plan-related HTTP requests. Usage counts come from the success-only ledger (IMM-6b P1).
type PlanHandler struct {
	planService     *service.PlanService
	scanUsageLedger repository.ScanUsageLedgerRepository
}

// NewPlanHandler creates a new plan handler.
func NewPlanHandler(planService *service.PlanService, scanUsageLedger repository.ScanUsageLedgerRepository) *PlanHandler {
	return &PlanHandler{
		planService:     planService,
		scanUsageLedger: scanUsageLedger,
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
			"error": "invalid user id format",
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
			"error": "invalid user id format",
		})
	}

	usage, err := h.planService.GetPlanUsage(userID, h.scanUsageLedger)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(usage)
}

