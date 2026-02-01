package handler

import (
	"cafe-discovery/internal/service"

	"github.com/gofiber/fiber/v2"
)

const errPubKeyHashRequired = "pub_key_hash is required"

// CafeWalletHandler handles cafe wallet-related HTTP requests
type CafeWalletHandler struct {
	walletService *service.CafeWalletService
}

// NewCafeWalletHandler creates a new cafe wallet handler
func NewCafeWalletHandler(walletService *service.CafeWalletService) *CafeWalletHandler {
	return &CafeWalletHandler{
		walletService: walletService,
	}
}

// CreateWallet handles POST /wallets
func (h *CafeWalletHandler) CreateWallet(c *fiber.Ctx) error {
	userID, err := requireAuthenticatedUserID(c)
	if err != nil {
		return err
	}

	var req service.CreateWalletRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	wallet, err := h.walletService.CreateWallet(userID, req)
	if err != nil {
		if err == service.ErrWalletExists {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(wallet)
}

// GetWallet handles GET /wallets/:pubKeyHash
func (h *CafeWalletHandler) GetWallet(c *fiber.Ctx) error {
	userID, err := requireAuthenticatedUserID(c)
	if err != nil {
		return err
	}

	pubKeyHash := c.Params("pubKeyHash")
	if pubKeyHash == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": errPubKeyHashRequired,
		})
	}

	wallet, err := h.walletService.GetWallet(userID, pubKeyHash)
	if err != nil {
		if err == service.ErrWalletNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(wallet)
}

// GetAllWallets handles GET /wallets
func (h *CafeWalletHandler) GetAllWallets(c *fiber.Ctx) error {
	userID, err := requireAuthenticatedUserID(c)
	if err != nil {
		return err
	}

	wallets, err := h.walletService.GetAllWallets(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"wallets": wallets,
		"count":   len(wallets),
	})
}

// UpdateWallet handles PUT /wallets/:pubKeyHash
func (h *CafeWalletHandler) UpdateWallet(c *fiber.Ctx) error {
	userID, err := requireAuthenticatedUserID(c)
	if err != nil {
		return err
	}

	pubKeyHash := c.Params("pubKeyHash")
	if pubKeyHash == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": errPubKeyHashRequired,
		})
	}

	var req service.UpdateWalletRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	wallet, err := h.walletService.UpdateWallet(userID, pubKeyHash, req)
	if err != nil {
		if err == service.ErrWalletNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(wallet)
}

// DeleteWallet handles DELETE /wallets/:pubKeyHash
func (h *CafeWalletHandler) DeleteWallet(c *fiber.Ctx) error {
	userID, err := requireAuthenticatedUserID(c)
	if err != nil {
		return err
	}

	pubKeyHash := c.Params("pubKeyHash")
	if pubKeyHash == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": errPubKeyHashRequired,
		})
	}

	if err := h.walletService.DeleteWallet(userID, pubKeyHash); err != nil {
		if err == service.ErrWalletNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusNoContent).Send(nil)
}
