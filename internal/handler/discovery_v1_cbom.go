package handler

import (
	"strings"

	"cafe-discovery/internal/cbom"
	"cafe-discovery/internal/domain"
	"cafe-discovery/pkg/scan"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// GetDiscoveryV1WalletScanCBOM handles GET /discovery/v1/wallets/scans/:scan_id/cbom (W6, G4).
func (h *DiscoveryHandler) GetDiscoveryV1WalletScanCBOM(c fiber.Ctx) error {
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

	if h.scanRead != nil {
		ent, qerr := h.scanRead.GetWalletScan(c.RequestCtx(), userID, tenantIDFromDiscoveryV1Request(c), scanID)
		if qerr != nil {
			return respondScanReadError(c, qerr, "scan CBOM is temporarily unavailable")
		}
		if ent != nil {
			if !scan.IsSuccess(ent.Status) {
				return scanCBOMNotFound(c)
			}
			return c.JSON(h.walletCBOMFromEntity(ent))
		}
	}

	if h.scanPending != nil {
		rec, perr := h.scanPending.Get(c.RequestCtx(), userID, tenantIDFromDiscoveryV1Request(c), scanID)
		if perr != nil {
			return respondScanPendingError(c, perr, "scan CBOM is temporarily unavailable")
		}
		if rec != nil && rec.UserID == userID && rec.Family == "wallet" {
			return scanCBOMNotFound(c)
		}
	}

	return scanCBOMNotFound(c)
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

func scanCBOMNotFound(c fiber.Ctx) error {
	return c.Status(fiber.StatusNotFound).JSON(v1ErrorBody(fiber.Map{
		"error":   "not_found",
		"message": "scan not found",
	}))
}

// GetDiscoveryV1TLSScanCBOM handles GET /discovery/v1/tls/scans/:scan_id/cbom (W6, G4, TLS sibling).
func (h *TLSHandler) GetDiscoveryV1TLSScanCBOM(c fiber.Ctx) error {
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

	if h.scanRead != nil {
		ent, qerr := h.scanRead.GetTLSScan(c.RequestCtx(), userID, tenantIDFromDiscoveryV1Request(c), scanID)
		if qerr != nil {
			return respondScanReadError(c, qerr, "scan CBOM is temporarily unavailable")
		}
		if ent != nil {
			if !scan.IsSuccess(ent.Status) {
				return scanCBOMNotFound(c)
			}
			return c.JSON(cbom.TLS(ent.ToTLSScanResult()))
		}
	}

	if h.scanPending != nil {
		rec, perr := h.scanPending.Get(c.RequestCtx(), userID, tenantIDFromDiscoveryV1Request(c), scanID)
		if perr != nil {
			return respondScanPendingError(c, perr, "scan CBOM is temporarily unavailable")
		}
		if rec != nil && rec.UserID == userID && rec.Family == "tls" {
			return scanCBOMNotFound(c)
		}
	}

	return scanCBOMNotFound(c)
}

// fiber:context-methods migrated
