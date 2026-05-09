package handler

import (
	"context"
	"log"
	"strings"
	"time"

	"cafe-discovery/internal/authz"
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

// ValidateSessionForCPM handles POST /internal/auth/session/validate (CPM AUTH-01).
// It is protected by InternalServiceAuth; CPM must send the shared service bearer token.
// Body: JSON {"token":"<Discovery-issued session token>"}.
// Success body matches cafe-crypto-policy-mgt discoveryValidationResponse (`accepted`, `claims`).
func (h *AuthHandler) ValidateSessionForCPM(c *fiber.Ctx) error {
	requestID := authz.EnsureRequestID(c.Get(authz.HeaderRequestID))
	c.Set(authz.HeaderRequestID, requestID)

	var body struct {
		Token string `json:"token"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"accepted":   false,
			"request_id": requestID,
			"error":      "invalid JSON body",
		})
	}
	raw := strings.TrimSpace(body.Token)
	if raw == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"accepted":   false,
			"request_id": requestID,
			"error":      "token is required",
		})
	}
	if h.authService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"accepted":   false,
			"request_id": requestID,
			"error":      "auth service unavailable",
		})
	}
	claims, err := h.authService.ValidateToken(raw)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"accepted":      false,
			"request_id":    requestID,
			"error_message": err.Error(),
		})
	}
	return c.JSON(fiber.Map{
		"accepted": true,
		"claims": fiber.Map{
			"user_id": claims.UserID.String(),
			"email":   claims.Email,
		},
		"request_id": requestID,
	})
}
