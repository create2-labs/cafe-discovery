package app

import (
	"cafe-discovery/internal/handler"

	"github.com/gofiber/fiber/v2"
)

// discoveryV1WalletHandlers is the subset of CAFE wallet HTTP handlers mounted under
// /discovery/v1/wallets (WORKPLAN_API.md §0.1). Implemented by *handler.CafeWalletHandler.
type discoveryV1WalletHandlers interface {
	GetAllWallets(c *fiber.Ctx) error
	CreateWallet(c *fiber.Ctx) error
	GetWallet(c *fiber.Ctx) error
	UpdateWallet(c *fiber.Ctx) error
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
// discovery may be nil in tests that only exercise wallet route wiring; POST /scan falls back to a 501 stub.
func registerDiscoveryV1Routes(
	v1 fiber.Router,
	discovery *handler.DiscoveryHandler,
	tls *handler.TLSHandler,
	wallets discoveryV1WalletHandlers,
) {
	w := v1.Group("/wallets")
	w.Get("/", wallets.GetAllWallets)
	w.Post("/", wallets.CreateWallet)
	if discovery != nil {
		w.Get("/scans", discovery.ListDiscoveryV1WalletScans)
		w.Get("/scans/:scan_id", discovery.GetDiscoveryV1WalletScan)
		w.Delete("/scans/:scan_id", discovery.DeleteDiscoveryV1WalletScan)
	} else {
		w.Get("/scans", discoveryV1NotImplemented("GET /discovery/v1/wallets/scans"))
		w.Get("/scans/:scan_id", discoveryV1NotImplemented("GET /discovery/v1/wallets/scans/:scan_id"))
		w.Delete("/scans/:scan_id", discoveryV1NotImplemented("DELETE /discovery/v1/wallets/scans/:scan_id"))
	}
	w.Get(walletPubKeyHashPath, wallets.GetWallet)
	w.Put(walletPubKeyHashPath, wallets.UpdateWallet)
	w.Delete(walletPubKeyHashPath, wallets.DeleteWallet)

	tlsGroup := v1.Group("/tls")
	if tls != nil {
		tlsGroup.Get("/scans", tls.ListDiscoveryV1TLSScans)
		tlsGroup.Get("/scans/:scan_id", tls.GetDiscoveryV1TLSScan)
		tlsGroup.Delete("/scans/:scan_id", tls.DeleteDiscoveryV1TLSScan)
	} else {
		tlsGroup.Get("/scans", discoveryV1NotImplemented("GET /discovery/v1/tls/scans"))
		tlsGroup.Get("/scans/:scan_id", discoveryV1NotImplemented("GET /discovery/v1/tls/scans/:scan_id"))
		tlsGroup.Delete("/scans/:scan_id", discoveryV1NotImplemented("DELETE /discovery/v1/tls/scans/:scan_id"))
	}

	if discovery != nil {
		v1.Post("/scan", discovery.PostDiscoveryScanV1)
	} else {
		v1.Post("/scan", discoveryV1NotImplemented("POST /discovery/v1/scan"))
	}
}
