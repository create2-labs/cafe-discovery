package handler

import (
	"errors"

	"cafe-discovery/internal/persistence/scanread"

	"github.com/gofiber/fiber/v2"
)

func respondScanReadUnavailable(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusServiceUnavailable).JSON(v1ErrorBody(fiber.Map{
		"error":   "service_unavailable",
		"message": message,
	}))
}

func respondScanReadError(c *fiber.Ctx, err error, message string) error {
	if errors.Is(err, scanread.ErrUnavailable) {
		return respondScanReadUnavailable(c, message)
	}
	return c.Status(fiber.StatusInternalServerError).JSON(v1ErrorBody(fiber.Map{
		"error":   "internal_error",
		"message": err.Error(),
	}))
}
