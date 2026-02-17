package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// getUserIDFromContext extracts and validates the user ID from the JWT context.
// Returns error with 401 when no user_id is present (e.g. missing or invalid token).
func getUserIDFromContext(c *fiber.Ctx) (uuid.UUID, error) {
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

	return userID, nil
}

// requireAuthenticatedUserID extracts user ID from JWT context and rejects unauthenticated or empty user.
// Use for routes that require a signed-in user (e.g. wallets, persisted data).
// Returns 401 when user is not authenticated so the frontend can redirect to sign-in.
func requireAuthenticatedUserID(c *fiber.Ctx) (uuid.UUID, error) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		return uuid.Nil, err
	}
	if userID == uuid.Nil {
		return uuid.Nil, c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "sign in required to access this resource",
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
