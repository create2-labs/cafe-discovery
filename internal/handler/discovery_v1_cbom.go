package handler

import (
	"strings"

	"cafe-discovery/internal/cbom"
	"cafe-discovery/internal/domain"
	"cafe-discovery/pkg/scan"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// GetDiscoveryV1WalletScanCBOM handles GET /discovery/v1/wallets/scans/:scan_id/cbom (W6).
func (h *DiscoveryHandler) GetDiscoveryV1WalletScanCBOM(c *fiber.Ctx) error {
	userID, err := h.getAuthenticatedUserID(c)
	if err != nil {
		return err
	}
	scanID, err := uuid.Parse(strings.TrimSpace(c.Params("scan_id")))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(v1ErrorBody(fiber.Map{
			"error":   "invalid_request",
			"message": "scan_id must be a UUID",
		}))
	}

	if h.scanResultRepo != nil {
		ent, qerr := h.scanResultRepo.FindOwnedWalletScanByID(userID, scanID)
		if qerr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(v1ErrorBody(fiber.Map{
				"error":   "internal_error",
				"message": qerr.Error(),
			}))
		}
		if ent != nil {
			if !scan.IsTerminal(ent.Status) {
				return c.Status(fiber.StatusConflict).JSON(v1ErrorBody(scanNotTerminalErrorBody(ent.Status)))
			}
			return c.JSON(h.walletCBOMFromEntity(ent))
		}
	}

	if h.pendingV1 != nil {
		rec, perr := h.pendingV1.Get(c.Context(), scanID)
		if perr != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(v1ErrorBody(fiber.Map{
				"error":   "service_unavailable",
				"message": "scan CBOM is temporarily unavailable",
			}))
		}
		if rec != nil && rec.UserID == userID && rec.Family == "wallet" {
			return c.Status(fiber.StatusConflict).JSON(v1ErrorBody(scanNotTerminalErrorBody("requested")))
		}
	}

	return c.Status(fiber.StatusNotFound).JSON(v1ErrorBody(fiber.Map{
		"error":   "not_found",
		"message": "scan not found",
	}))
}

func (h *DiscoveryHandler) walletCBOMFromEntity(ent *domain.ScanResultEntity) fiber.Map {
	sr := ent.ToScanResult()
	address := sr.Address
	if h.discoveryService != nil {
		if normalized, nerr := h.discoveryService.ValidateAndNormalizeAddress(sr.Address); nerr == nil {
			address = normalized
		}
	}
	return cbom.Wallet(sr, address)
}

func scanNotTerminalErrorBody(dbStatus string) fiber.Map {
	st := walletScanLifecycleStatusV1(dbStatus)
	return fiber.Map{
		"error":   "SCAN_NOT_TERMINAL",
		"message": "CBOM is available only after the scan reaches a terminal state (current status: " + st + ")",
	}
}

// GetDiscoveryV1TLSScanCBOM handles GET /discovery/v1/tls/scans/:scan_id/cbom (W6, TLS sibling).
func (h *TLSHandler) GetDiscoveryV1TLSScanCBOM(c *fiber.Ctx) error {
	userID, err := requireAuthenticatedUserID(c)
	if err != nil {
		return err
	}
	scanID, err := uuid.Parse(strings.TrimSpace(c.Params("scan_id")))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(v1ErrorBody(fiber.Map{
			"error":   "invalid_request",
			"message": "scan_id must be a UUID",
		}))
	}

	if h.tlsScanResultRepo != nil {
		ent, qerr := h.tlsScanResultRepo.FindOwnedUserTLSScanByID(userID, scanID)
		if qerr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(v1ErrorBody(fiber.Map{
				"error":   "internal_error",
				"message": qerr.Error(),
			}))
		}
		if ent == nil {
			ent, qerr = h.tlsScanResultRepo.FindDefaultTLSScanByID(scanID)
			if qerr != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(v1ErrorBody(fiber.Map{
					"error":   "internal_error",
					"message": qerr.Error(),
				}))
			}
		}
		if ent != nil {
			if !scan.IsTerminal(ent.Status) {
				return c.Status(fiber.StatusConflict).JSON(v1ErrorBody(scanNotTerminalErrorBody(ent.Status)))
			}
			return c.JSON(cbom.TLS(ent.ToTLSScanResult()))
		}
	}

	if h.pendingV1 != nil {
		rec, perr := h.pendingV1.Get(c.Context(), scanID)
		if perr != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(v1ErrorBody(fiber.Map{
				"error":   "service_unavailable",
				"message": "scan CBOM is temporarily unavailable",
			}))
		}
		if rec != nil && rec.UserID == userID && rec.Family == "tls" {
			return c.Status(fiber.StatusConflict).JSON(v1ErrorBody(scanNotTerminalErrorBody("requested")))
		}
	}

	return c.Status(fiber.StatusNotFound).JSON(v1ErrorBody(fiber.Map{
		"error":   "not_found",
		"message": "scan not found",
	}))
}
