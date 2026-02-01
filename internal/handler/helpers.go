package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// getUserIDFromContext extracts and validates the user ID from the JWT context.
// Returns uuid.Nil when the user is anonymous (JWT with user_id = nil UUID).
func getUserIDFromContext(c *fiber.Ctx) (uuid.UUID, error) {
	userIDValue := c.Locals("user_id")
	if userIDValue == nil {
		return uuid.Nil, c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "user not authenticated",
		})
	}

	userID, ok := userIDValue.(uuid.UUID)
	if !ok {
		return uuid.Nil, c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "invalid user ID format",
		})
	}

	return userID, nil
}

// requireAuthenticatedUserID extracts user ID from JWT context and rejects anonymous users.
// Use for routes that must only allow signed-in users (e.g. wallets, persisted data).
// Anonymous tokens (user_id = uuid.Nil) get 403 Forbidden to avoid sharing data between anonymous sessions.
func requireAuthenticatedUserID(c *fiber.Ctx) (uuid.UUID, error) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		return uuid.Nil, err
	}
	if userID == uuid.Nil {
		return uuid.Nil, c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "sign in required for this action",
		})
	}
	return userID, nil
}

// parsePaginationParams parses and validates pagination parameters from the query string
func parsePaginationParams(c *fiber.Ctx) (limit, offset int) {
	limit = c.QueryInt("limit", 20)  // Default 20, max 100
	offset = c.QueryInt("offset", 0) // Default 0

	// Validate limit
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	return limit, offset
}
