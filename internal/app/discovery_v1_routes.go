package app

import (
	"cafe-discovery/internal/handler"

	"github.com/gofiber/fiber/v2"
)

// discoveryV1WalletHandlers is the subset of CAFE wallet HTTP handlers mounted under
// /discovery/v1/wallets (WORKPLAN_API.md §0.1). Implemented by *handler.CafeWalletHandler.
type discoveryV1WalletHandlers interface {
	GetAllWallets(c *fiber.Ctx) error
	GetWallet(c *fiber.Ctx) error
	DeleteWallet(c *fiber.Ctx) error
}

func discoveryV1NotImplemented(feature string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
			"error":   "not_implemented",
			"message": feature + " (see WORKPLAN_API_PR.md PR3–PR6)",
		})
	}
}

// registerDiscoveryV1Routes registers WORKPLAN §0.1 paths under prefix (typically /discovery/v1 with JWT).
// Static segments /wallets/scans and /wallets/scans/:scan_id MUST be registered before /wallets/:pubKeyHash
// so "scans" is never captured as a wallet id (Fiber route order).
func registerDiscoveryV1Routes(
	v1 fiber.Router,
	_ *handler.DiscoveryHandler,
	_ *handler.TLSHandler,
	wallets discoveryV1WalletHandlers,
) {
	w := v1.Group("/wallets")
	w.Get("/", wallets.GetAllWallets)
	w.Get("/scans", discoveryV1NotImplemented("GET /discovery/v1/wallets/scans"))
	w.Get("/scans/:scan_id", discoveryV1NotImplemented("GET /discovery/v1/wallets/scans/:scan_id"))
	w.Delete("/scans/:scan_id", discoveryV1NotImplemented("DELETE /discovery/v1/wallets/scans/:scan_id"))
	w.Get(walletPubKeyHashPath, wallets.GetWallet)
	w.Delete(walletPubKeyHashPath, wallets.DeleteWallet)

	tls := v1.Group("/tls")
	tls.Get("/scans", discoveryV1NotImplemented("GET /discovery/v1/tls/scans"))
	tls.Get("/scans/:scan_id", discoveryV1NotImplemented("GET /discovery/v1/tls/scans/:scan_id"))
	tls.Delete("/scans/:scan_id", discoveryV1NotImplemented("DELETE /discovery/v1/tls/scans/:scan_id"))

	v1.Post("/scan", discoveryV1NotImplemented("POST /discovery/v1/scan"))
}
