package handler

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"cafe-discovery/internal/config"
	"cafe-discovery/internal/domain"
	"cafe-discovery/pkg/scan"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// ListDiscoveryV1WalletScans handles GET /discovery/v1/wallets/scans (WORKPLAN_API §0.1, OpenAPI listWalletScans).
func (h *DiscoveryHandler) ListDiscoveryV1WalletScans(c *fiber.Ctx) error {
	userID, err := h.getAuthenticatedUserID(c)
	if err != nil {
		return err
	}

	chainQ := strings.TrimSpace(c.Query("chain_id"))
	addrQ := strings.TrimSpace(c.Query("address"))
	if chainQ != "" && addrQ == "" {
		return c.Status(fiber.StatusBadRequest).JSON(v1ErrorBody(fiber.Map{
			"error":   "invalid_request",
			"message": "chain_id requires address",
		}))
	}

	var chainID *int64
	if chainQ != "" {
		v, perr := strconv.ParseInt(chainQ, 10, 64)
		if perr != nil || v <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(v1ErrorBody(fiber.Map{
				"error":   "invalid_request",
				"message": "chain_id must be a positive integer",
			}))
		}
		chainID = &v
	}

	normalizedAddr := ""
	if addrQ != "" {
		n, verr := h.discoveryService.ValidateAndNormalizeAddress(addrQ)
		if verr != nil {
			return c.Status(fiber.StatusBadRequest).JSON(v1ErrorBody(fiber.Map{
				"error":   "invalid_request",
				"message": verr.Error(),
			}))
		}
		normalizedAddr = n
	}

	if h.scanResultRepo == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(v1ErrorBody(fiber.Map{
			"error":   "service_unavailable",
			"message": "wallet scan history is temporarily unavailable",
		}))
	}

	limit, offset := parsePaginationParams(c)

	var entities []*domain.ScanResultEntity
	var total int64

	if normalizedAddr != "" && chainID != nil {
		ent, lerr := h.scanResultRepo.FindByUserIDAndAddress(userID, normalizedAddr)
		if lerr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(v1ErrorBody(fiber.Map{
				"error":   "internal_error",
				"message": lerr.Error(),
			}))
		}
		if ent == nil || !walletEntityMatchesChainID(ent, *chainID, h.cfgChain) {
			return c.JSON(fiber.Map{"items": []fiber.Map{}, "total": 0, "limit": limit, "offset": offset})
		}
		entities = []*domain.ScanResultEntity{ent}
		total = 1
		if offset > 0 {
			entities = nil
		}
	} else if normalizedAddr != "" {
		var qerr error
		entities, total, qerr = h.scanResultRepo.ListOwnerWalletScansDiscoveryV1(userID, normalizedAddr, limit, offset)
		if qerr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(v1ErrorBody(fiber.Map{
				"error":   "internal_error",
				"message": qerr.Error(),
			}))
		}
	} else {
		var qerr error
		entities, total, qerr = h.scanResultRepo.ListOwnerWalletScansDiscoveryV1(userID, "", limit, offset)
		if qerr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(v1ErrorBody(fiber.Map{
				"error":   "internal_error",
				"message": qerr.Error(),
			}))
		}
	}

	items := make([]fiber.Map, 0, len(entities))
	for _, e := range entities {
		items = append(items, walletScanListItemV1(e, h.cfgChain))
	}
	return c.JSON(fiber.Map{
		"items":  items,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetDiscoveryV1WalletScan handles GET /discovery/v1/wallets/scans/:scan_id.
func (h *DiscoveryHandler) GetDiscoveryV1WalletScan(c *fiber.Ctx) error {
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
			return c.JSON(walletScanDetailV1(ent, h.cfgChain))
		}
	}

	if h.pendingV1 != nil {
		rec, perr := h.pendingV1.Get(c.Context(), scanID)
		if perr != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(v1ErrorBody(fiber.Map{
				"error":   "service_unavailable",
				"message": "scan detail is temporarily unavailable",
			}))
		}
		if rec != nil && rec.UserID == userID && rec.Family == "wallet" {
			return c.JSON(fiber.Map{
				"scan_id": rec.ScanID.String(),
				"status":  "requested",
			})
		}
	}

	return c.Status(fiber.StatusNotFound).JSON(v1ErrorBody(fiber.Map{
		"error":   "not_found",
		"message": "scan not found",
	}))
}

func walletEntityMatchesChainID(e *domain.ScanResultEntity, want int64, cfg *config.ChainConfig) bool {
	if e == nil || cfg == nil {
		return false
	}
	nets := parseNetworksFromEntity(e.Networks)
	if len(nets) == 0 {
		return false
	}
	byName := cfg.ChainIDByNetwork()
	wantStr := strconv.FormatInt(want, 10)
	for _, n := range nets {
		if n == wantStr {
			return true
		}
		if cid, ok := byName[n]; ok && cid == want {
			return true
		}
	}
	return false
}

func parseNetworksFromEntity(networksJSON string) []string {
	if networksJSON == "" || networksJSON == "[]" {
		return nil
	}
	var arr []string
	if err := json.Unmarshal([]byte(networksJSON), &arr); err != nil {
		return nil
	}
	return arr
}

func walletScanListItemV1(e *domain.ScanResultEntity, cfg *config.ChainConfig) fiber.Map {
	chainIDs := chainIDsForNetworks(e.Networks, cfg)
	out := fiber.Map{
		"scan_id":        e.ID.String(),
		"created_at":     e.CreatedAt.UTC().Format(time.RFC3339Nano),
		"status":         walletScanLifecycleStatusV1(e.Status),
		"target_address": e.Address,
		"chain_ids":      chainIDs,
	}
	return out
}

func chainIDsForNetworks(networksJSON string, cfg *config.ChainConfig) []int64 {
	names := parseNetworksFromEntity(networksJSON)
	if len(names) == 0 || cfg == nil {
		return []int64{}
	}
	byName := cfg.ChainIDByNetwork()
	seen := make(map[int64]struct{})
	var out []int64
	for _, n := range names {
		if id, err := strconv.ParseInt(strings.TrimSpace(n), 10, 64); err == nil && id > 0 {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				out = append(out, id)
			}
			continue
		}
		if cid, ok := byName[n]; ok && cid > 0 {
			if _, ok := seen[cid]; !ok {
				seen[cid] = struct{}{}
				out = append(out, cid)
			}
		}
	}
	return out
}

func walletScanLifecycleStatusV1(dbStatus string) string {
	switch strings.ToUpper(strings.TrimSpace(dbStatus)) {
	case scan.StatePENDING, "":
		return "requested"
	case scan.StateRUNNING:
		return "started"
	case scan.StateSUCCESS:
		return "completed"
	case scan.StateFAILED, scan.StateTIMEOUT, scan.StateUNREACHABLE:
		return "failed"
	default:
		return "started"
	}
}

func walletScanDetailV1(e *domain.ScanResultEntity, cfg *config.ChainConfig) fiber.Map {
	st := walletScanLifecycleStatusV1(e.Status)
	out := fiber.Map{
		"scan_id": e.ID.String(),
		"status":  st,
	}
	if scan.IsTerminal(e.Status) {
		out["result"] = walletScanResultV1(e, cfg)
	}
	return out
}

func walletScanResultV1(e *domain.ScanResultEntity, cfg *config.ChainConfig) fiber.Map {
	return fiber.Map{
		"target_address":      e.Address,
		"chain_ids":           chainIDsForNetworks(e.Networks, cfg),
		"wallet_type":         walletAccountTypeV1(e.Type, e.IsEOA, e.IsERC4337),
		"current_pq_posture":  nistLevelToPQPosture(e.NISTLevel),
		"observations":        []any{},
	}
}

func walletAccountTypeV1(t domain.AccountType, isEOA, is4337 bool) string {
	if is4337 || t == domain.AccountTypeAA {
		return "smart_account"
	}
	if t == domain.AccountTypeContract {
		return "contract"
	}
	if isEOA || t == domain.AccountTypeEOA {
		return "eoa"
	}
	return "unknown"
}

func nistLevelToPQPosture(l domain.NISTLevel) string {
	switch {
	case l >= 4:
		return "pq_ready"
	case l >= 2:
		return "hybrid"
	case l == 1:
		return "not_pq_ready"
	default:
		return "unknown"
	}
}

// ListDiscoveryV1TLSScans handles GET /discovery/v1/tls/scans.
func (h *TLSHandler) ListDiscoveryV1TLSScans(c *fiber.Ctx) error {
	userID, err := requireAuthenticatedUserID(c)
	if err != nil {
		return err
	}
	if discoveryV1TLSForbiddenQueryKeys(c) {
		return c.Status(fiber.StatusBadRequest).JSON(v1ErrorBody(fiber.Map{
			"error":   "invalid_request",
			"message": "address and chain_id are not applicable to TLS scan list",
		}))
	}
	if h.tlsScanResultRepo == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(v1ErrorBody(fiber.Map{
			"error":   "service_unavailable",
			"message": "TLS scan history is temporarily unavailable",
		}))
	}
	limit, offset := parsePaginationParams(c)
	entities, total, qerr := h.tlsScanResultRepo.ListOwnerUserTLSScansDiscoveryV1(userID, limit, offset)
	if qerr != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(v1ErrorBody(fiber.Map{
			"error":   "internal_error",
			"message": qerr.Error(),
		}))
	}
	items := make([]fiber.Map, 0, len(entities))
	for _, e := range entities {
		items = append(items, tlsScanListItemV1(e))
	}
	return c.JSON(fiber.Map{
		"items":  items,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetDiscoveryV1TLSScan handles GET /discovery/v1/tls/scans/:scan_id.
func (h *TLSHandler) GetDiscoveryV1TLSScan(c *fiber.Ctx) error {
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
		if ent != nil {
			return c.JSON(tlsScanDetailV1(ent))
		}
	}

	if h.pendingV1 != nil {
		rec, perr := h.pendingV1.Get(c.Context(), scanID)
		if perr != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(v1ErrorBody(fiber.Map{
				"error":   "service_unavailable",
				"message": "scan detail is temporarily unavailable",
			}))
		}
		if rec != nil && rec.UserID == userID && rec.Family == "tls" {
			return c.JSON(fiber.Map{
				"scan_id": rec.ScanID.String(),
				"status":  "requested",
			})
		}
	}

	return c.Status(fiber.StatusNotFound).JSON(v1ErrorBody(fiber.Map{
		"error":   "not_found",
		"message": "scan not found",
	}))
}

func discoveryV1TLSForbiddenQueryKeys(c *fiber.Ctx) bool {
	for k := range c.Queries() {
		kl := strings.ToLower(strings.TrimSpace(k))
		if kl == "address" || kl == "chain_id" {
			return true
		}
	}
	return false
}

func tlsScanListItemV1(e *domain.TLSScanResultEntity) fiber.Map {
	return fiber.Map{
		"scan_id":    e.ID.String(),
		"endpoint":   e.URL,
		"created_at": e.CreatedAt.UTC().Format(time.RFC3339Nano),
		"status":     walletScanLifecycleStatusV1(e.Status),
	}
}

func tlsScanDetailV1(e *domain.TLSScanResultEntity) fiber.Map {
	dto := e.ToTLSScanResult()
	st := walletScanLifecycleStatusV1(e.Status)
	out := fiber.Map{
		"scan_id": e.ID.String(),
		"status":  st,
	}
	if scan.IsTerminal(e.Status) {
		out["result"] = tlsScanResultBodyV1(dto)
	}
	return out
}

func tlsScanResultBodyV1(dto *domain.TLSScanResult) fiber.Map {
	certSummary := fiber.Map{
		"subject":            dto.Certificate.Subject,
		"issuer":             dto.Certificate.Issuer,
		"signature_algorithm": dto.Certificate.SignatureAlgorithm,
		"not_after":          dto.Certificate.NotAfter.UTC().Format(time.RFC3339Nano),
	}
	kex := ""
	if len(dto.CipherSuites) > 0 {
		kex = dto.CipherSuites[0].KeyExchange
	}
	cipher := ""
	if len(dto.CipherSuites) > 0 {
		cipher = dto.CipherSuites[0].Name
	}
	return fiber.Map{
		"endpoint":             dto.URL,
		"tls_version":          dto.ProtocolVersion,
		"cipher_suite":         cipher,
		"key_exchange":         kex,
		"certificate_summary":  certSummary,
		"current_pq_posture":   nistLevelToPQPosture(dto.NISTLevel),
		"observations":         []any{},
	}
}
