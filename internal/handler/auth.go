package handler

import (
	"cafe-discovery/internal/service"

	"github.com/gofiber/fiber/v2"
)

// AuthHandler handles authentication-related HTTP requests
type AuthHandler struct {
	authService *service.AuthService
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
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

	return c.JSON(response)
}

// GetAnonymousToken handles GET /auth/anonymous
// Returns a JWT token for anonymous (non-authenticated) users
// This allows users to use the service without creating an account
func (h *AuthHandler) GetAnonymousToken(c *fiber.Ctx) error {
	response, err := h.authService.GetAnonymousToken()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to generate anonymous token",
		})
	}

	return c.JSON(response)
}

