package handler

import (
	"context"
	"log"
	"time"

	"cafe-discovery/internal/service"

	"github.com/gofiber/fiber/v2"
)

// AuthHandler handles authentication-related HTTP requests
type AuthHandler struct {
	authService    *service.AuthService
	userScanCache  *service.UserScanCacheService
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authService *service.AuthService, userScanCache *service.UserScanCacheService) *AuthHandler {
	return &AuthHandler{
		authService:   authService,
		userScanCache: userScanCache,
	}
}

// Signup handles POST /auth/signup
func (h *AuthHandler) Signup(c *fiber.Ctx) error {
	var req service.SignupRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	response, err := h.authService.Signup(req)
	if err != nil {
		if err == service.ErrUserAlreadyExists {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(response)
}

// Signin handles POST /auth/signin
func (h *AuthHandler) Signin(c *fiber.Ctx) error {
	var req service.SigninRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	response, err := h.authService.Signin(req)
	if err != nil {
		if err == service.ErrUserNotFound || err == service.ErrInvalidPassword {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid email or password",
			})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Warm user scan cache from Postgres so first list after sign-in is fast
	if h.userScanCache != nil && response.User != nil {
		ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
		defer cancel()
		if warmErr := h.userScanCache.WarmForUser(ctx, response.User.ID); warmErr != nil {
			log.Printf("auth: warm user cache after sign-in: %v", warmErr)
		}
	}

	return c.JSON(response)
}

