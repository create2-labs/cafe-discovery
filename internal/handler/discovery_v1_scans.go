package handler

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"

	"cafe-discovery/internal/address"
	"cafe-discovery/internal/config"
	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/persistence/scanpending"
	"cafe-discovery/pkg/scan"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// ListDiscoveryV1WalletScans handles GET /discovery/v1/wallets/scans (WORKPLAN_API §0.1, OpenAPI listWalletScans).
func (h *DiscoveryHandler) ListDiscoveryV1WalletScans(c fiber.Ctx) error {
	userID, err := h.getAuthenticatedUserID(c)
	if err != nil {
		return err
	}

	chainQ := strings.TrimSpace(c.Query("chain_id"))
	addrQ := strings.TrimSpace(c.Query("address"))
	latest, latestErr := parseWalletScansLatestQuery(c.Query("latest"))
	if latestErr != nil {
		return c.Status(fiber.StatusBadRequest).JSON(v1ErrorBody(fiber.Map{
			"error":   "invalid_request",
			"message": latestErr.Error(),
		}))
	}
	if latest && addrQ == "" {
		return c.Status(fiber.StatusBadRequest).JSON(v1ErrorBody(fiber.Map{
			"error":   "invalid_request",
			"message": "latest requires address",
		}))
	}
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
		n, verr := address.ValidateAndNormalizeAddress(addrQ)
		if verr != nil {
			return c.Status(fiber.StatusBadRequest).JSON(v1ErrorBody(fiber.Map{
				"error":   "invalid_request",
				"message": verr.Error(),
			}))
		}
		normalizedAddr = n
	}

	if h.scanRead == nil {
		return respondScanReadUnavailable(c, "wallet scan history is temporarily unavailable")
	}

	limit, offset := parsePaginationParams(c)
	query := url.Values{}
	if normalizedAddr != "" {
		query.Set("address", normalizedAddr)
	}
	if chainID != nil {
		query.Set("chain_id", strconv.FormatInt(*chainID, 10))
	}
	if latest {
		query.Set("latest", "true")
	}
	query.Set("limit", strconv.Itoa(limit))
	query.Set("offset", strconv.Itoa(offset))

	entities, total, respLimit, respOffset, qerr := h.scanRead.ListWalletScans(c.RequestCtx(), userID, tenantIDFromDiscoveryV1Request(c), query)
	if qerr != nil {
		return respondScanReadError(c, qerr, "wallet scan history is temporarily unavailable")
	}

	items := make([]fiber.Map, 0, len(entities))
	for _, e := range entities {
		items = append(items, walletScanListItemV1(e, h.cfgChain))
	}
	return c.JSON(fiber.Map{
		"items":  items,
		"total":  total,
		"limit":  respLimit,
		"offset": respOffset,
	})
}

func parseWalletScansLatestQuery(raw string) (bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fiber.NewError(fiber.StatusBadRequest, "latest must be a boolean")
	}
	return v, nil
}

func walletEntityIsCompleted(e *domain.ScanResultEntity) bool {
	return e != nil && strings.ToUpper(strings.TrimSpace(e.Status)) == scan.StateSUCCESS
}

func paginateWalletScanEntities(in []*domain.ScanResultEntity, limit, offset int) []*domain.ScanResultEntity {
	if offset >= len(in) {
		return nil
	}
	end := offset + limit
	if end > len(in) {
		end = len(in)
	}
	return in[offset:end]
}

// GetDiscoveryV1WalletScan handles GET /discovery/v1/wallets/scans/:scan_id.
func (h *DiscoveryHandler) GetDiscoveryV1WalletScan(c fiber.Ctx) error {
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
			return respondScanReadError(c, qerr, "scan detail is temporarily unavailable")
		}
		if ent != nil {
			return c.JSON(walletScanDetailV1(ent, h.cfgChain))
		}
	}

	if h.scanPending != nil {
		rec, perr := h.scanPending.Get(c.RequestCtx(), userID, tenantIDFromDiscoveryV1Request(c), scanID)
		if perr != nil {
			return respondScanPendingError(c, perr, "scan detail is temporarily unavailable")
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

// walletScanListItemV1 maps a row to ScanListItem (WORKPLAN §5.4.2): lifecycle synopsis only.
// Crypto posture and business fields belong in GET …/wallets/scans/{scan_id} → result (terminal only).
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

// walletScanDetailV1 maps a row to WalletScanDetail. result is attached only at terminal states.
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

// ListDiscoveryV1TLSDefaultScans handles GET /discovery/v1/tls/scans/defaults.
// Returns the shared catalog of default TLS endpoints (not owner-scoped user scans).
func (h *TLSHandler) ListDiscoveryV1TLSDefaultScans(c fiber.Ctx) error {
	userID, err := requireAuthenticatedUserID(c)
	if err != nil {
		return err
	}
	if h.scanRead == nil {
		return respondScanReadUnavailable(c, "TLS default scan catalog is temporarily unavailable")
	}
	entities, err := h.scanRead.ListTLSDefaultScans(c.RequestCtx(), userID, tenantIDFromDiscoveryV1Request(c))
	if err != nil {
		return respondScanReadError(c, err, "TLS default scan catalog is temporarily unavailable")
	}
	items := make([]fiber.Map, 0, len(entities))
	for _, e := range entities {
		if e == nil {
			continue
		}
		item := tlsScanListItemV1(e)
		item["is_default"] = true
		items = append(items, item)
	}
	return c.JSON(fiber.Map{
		"items":  items,
		"total":  len(items),
		"limit":  len(items),
		"offset": 0,
	})
}

// ListDiscoveryV1TLSScans handles GET /discovery/v1/tls/scans.
func (h *TLSHandler) ListDiscoveryV1TLSScans(c fiber.Ctx) error {
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
	if h.scanRead == nil {
		return respondScanReadUnavailable(c, "TLS scan history is temporarily unavailable")
	}
	limit, offset := parsePaginationParams(c)
	entities, total, qerr := h.scanRead.ListTLSScans(c.RequestCtx(), userID, tenantIDFromDiscoveryV1Request(c), limit, offset)
	if qerr != nil {
		return respondScanReadError(c, qerr, "TLS scan history is temporarily unavailable")
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
func (h *TLSHandler) GetDiscoveryV1TLSScan(c fiber.Ctx) error {
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
			return respondScanReadError(c, qerr, "scan detail is temporarily unavailable")
		}
		if ent != nil {
			return c.JSON(tlsScanDetailV1(ent))
		}
	}

	if h.scanPending != nil {
		rec, perr := h.scanPending.Get(c.RequestCtx(), userID, tenantIDFromDiscoveryV1Request(c), scanID)
		if perr != nil {
			return respondScanPendingError(c, perr, "scan detail is temporarily unavailable")
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

func discoveryV1TLSForbiddenQueryKeys(c fiber.Ctx) bool {
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
	st := walletScanLifecycleStatusV1(e.Status)
	out := fiber.Map{
		"scan_id": e.ID.String(),
		"status":  st,
	}
	if e.Default {
		out["is_default"] = true
	}
	if scan.IsTerminal(e.Status) {
		out["result"] = tlsScanResultBodyV1(e)
	}
	return out
}

func tenantIDFromDiscoveryV1Request(c fiber.Ctx) string {
	return strings.TrimSpace(c.Get("X-Tenant-Id"))
}

func respondPolicyReferenceCheckUnavailable(c fiber.Ctx) error {
	return c.Status(fiber.StatusServiceUnavailable).JSON(v1ErrorBody(fiber.Map{
		"error":   "POLICY_REFERENCE_CHECK_UNAVAILABLE",
		"message": "The scan cannot be deleted because policy references could not be verified.",
	}))
}

// clearPendingV1ScanCorrelation removes pending v1 keys for scan_id so GET detail
// does not resurrect a deleted scan as status "requested" after the Postgres row is gone.
func (h *DiscoveryHandler) clearPendingV1ScanCorrelation(c fiber.Ctx, userID, scanID uuid.UUID, walletAddress string) error {
	if h.scanPending == nil {
		return nil
	}
	tenantID := tenantIDFromDiscoveryV1Request(c)
	if err := h.scanPending.Delete(c.RequestCtx(), userID, tenantID, scanID); err != nil {
		return err
	}
	if addr := strings.TrimSpace(walletAddress); addr != "" {
		_ = h.scanPending.DeleteWalletReservation(c.RequestCtx(), userID, tenantID, addr, scanID)
	}
	return nil
}

func (h *TLSHandler) clearPendingV1ScanCorrelation(c fiber.Ctx, userID, scanID uuid.UUID) error {
	if h.scanPending == nil {
		return nil
	}
	return h.scanPending.Delete(c.RequestCtx(), userID, tenantIDFromDiscoveryV1Request(c), scanID)
}

// DeleteDiscoveryV1WalletScan handles DELETE /discovery/v1/wallets/scans/:scan_id (WORKPLAN_API_PR PR6).
func (h *DiscoveryHandler) DeleteDiscoveryV1WalletScan(c fiber.Ctx) error {
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
	tenantID := tenantIDFromDiscoveryV1Request(c)

	if h.scanRead == nil {
		return respondScanReadUnavailable(c, "wallet scan delete is temporarily unavailable")
	}

	var walletEnt *domain.ScanResultEntity
	var pendingRec *scanpending.Record

	ent, qerr := h.scanRead.GetWalletScan(c.RequestCtx(), userID, tenantID, scanID)
	if qerr != nil {
		return respondScanReadError(c, qerr, "wallet scan delete is temporarily unavailable")
	}
	walletEnt = ent
	if walletEnt == nil && h.scanPending != nil {
		rec, perr := h.scanPending.Get(c.RequestCtx(), userID, tenantID, scanID)
		if perr != nil {
			return respondPolicyReferenceCheckUnavailable(c)
		}
		if rec != nil && rec.UserID == userID && rec.Family == "wallet" {
			pendingRec = rec
		}
	}
	if walletEnt == nil && pendingRec == nil {
		return c.Status(fiber.StatusNotFound).JSON(v1ErrorBody(fiber.Map{
			"error":   "not_found",
			"message": "scan not found",
		}))
	}

	if h.policyRef == nil {
		return respondPolicyReferenceCheckUnavailable(c)
	}
	ref, err := h.policyRef.PersistedPoliciesReferenceScan(c.RequestCtx(), userID, tenantID, scanID)
	if err != nil {
		return respondPolicyReferenceCheckUnavailable(c)
	}
	if ref {
		return c.Status(fiber.StatusConflict).JSON(v1ErrorBody(fiber.Map{
			"error":   "SCAN_REFERENCED_BY_POLICY",
			"message": "This scan is referenced by one or more persisted crypto policies.",
		}))
	}

	if walletEnt != nil {
		deleted, derr := h.scanRead.DeleteWalletScan(c.RequestCtx(), userID, tenantID, scanID)
		if derr != nil {
			return respondScanReadError(c, derr, "wallet scan delete is temporarily unavailable")
		}
		if !deleted {
			return c.Status(fiber.StatusNotFound).JSON(v1ErrorBody(fiber.Map{
				"error":   "not_found",
				"message": "scan not found",
			}))
		}
		if err := h.clearPendingV1ScanCorrelation(c, userID, scanID, walletEnt.Address); err != nil {
			return respondPolicyReferenceCheckUnavailable(c)
		}
		return c.SendStatus(fiber.StatusNoContent)
	}

	if h.scanPending == nil {
		return respondPolicyReferenceCheckUnavailable(c)
	}
	if err := h.scanPending.Delete(c.RequestCtx(), userID, tenantID, scanID); err != nil {
		return respondPolicyReferenceCheckUnavailable(c)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// DeleteDiscoveryV1TLSScan handles DELETE /discovery/v1/tls/scans/:scan_id (WORKPLAN_API_PR PR6).
func (h *TLSHandler) DeleteDiscoveryV1TLSScan(c fiber.Ctx) error {
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
	tenantID := tenantIDFromDiscoveryV1Request(c)

	if h.scanRead == nil {
		return respondScanReadUnavailable(c, "TLS scan delete is temporarily unavailable")
	}

	var tlsEnt *domain.TLSScanResultEntity
	var pendingRec *scanpending.Record

	ent, qerr := h.scanRead.GetTLSScan(c.RequestCtx(), userID, tenantID, scanID)
	if qerr != nil {
		return respondScanReadError(c, qerr, "TLS scan delete is temporarily unavailable")
	}
	if ent != nil && !ent.Default {
		tlsEnt = ent
	}
	if tlsEnt == nil && h.scanPending != nil {
		rec, perr := h.scanPending.Get(c.RequestCtx(), userID, tenantID, scanID)
		if perr != nil {
			return respondPolicyReferenceCheckUnavailable(c)
		}
		if rec != nil && rec.UserID == userID && rec.Family == "tls" {
			pendingRec = rec
		}
	}
	if tlsEnt == nil && pendingRec == nil {
		return c.Status(fiber.StatusNotFound).JSON(v1ErrorBody(fiber.Map{
			"error":   "not_found",
			"message": "scan not found",
		}))
	}

	if h.policyRef == nil {
		return respondPolicyReferenceCheckUnavailable(c)
	}
	ref, err := h.policyRef.PersistedPoliciesReferenceScan(c.RequestCtx(), userID, tenantID, scanID)
	if err != nil {
		return respondPolicyReferenceCheckUnavailable(c)
	}
	if ref {
		return c.Status(fiber.StatusConflict).JSON(v1ErrorBody(fiber.Map{
			"error":   "SCAN_REFERENCED_BY_POLICY",
			"message": "This scan is referenced by one or more persisted crypto policies.",
		}))
	}

	if tlsEnt != nil {
		deleted, derr := h.scanRead.DeleteTLSScan(c.RequestCtx(), userID, tenantID, scanID)
		if derr != nil {
			return respondScanReadError(c, derr, "TLS scan delete is temporarily unavailable")
		}
		if !deleted {
			return c.Status(fiber.StatusNotFound).JSON(v1ErrorBody(fiber.Map{
				"error":   "not_found",
				"message": "scan not found",
			}))
		}
		if err := h.clearPendingV1ScanCorrelation(c, userID, scanID); err != nil {
			return respondPolicyReferenceCheckUnavailable(c)
		}
		return c.SendStatus(fiber.StatusNoContent)
	}

	if h.scanPending == nil {
		return respondPolicyReferenceCheckUnavailable(c)
	}
	if err := h.scanPending.Delete(c.RequestCtx(), userID, tenantID, scanID); err != nil {
		return respondPolicyReferenceCheckUnavailable(c)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// fiber:context-methods migrated
