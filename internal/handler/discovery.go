package handler

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"cafe-discovery/internal/config"
	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"
	"cafe-discovery/internal/service"
	"cafe-discovery/pkg/nats"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// DiscoveryHandler handles discovery-related HTTP requests
type DiscoveryHandler struct {
	discoveryService    *service.DiscoveryService
	tlsService          *service.TLSService
	cfgChain            *config.ChainConfig
	natsConn            nats.Connection
	redisWalletScanRepo repository.RedisWalletScanRepository
	redisTLSScanRepo    repository.RedisTLSScanRepository
	planService         *service.PlanService
}

// NewDiscoveryHandler creates a new discovery handler
func NewDiscoveryHandler(discoveryService *service.DiscoveryService, tlsService *service.TLSService, cfgChain *config.ChainConfig, natsConn nats.Connection, redisWalletScanRepo repository.RedisWalletScanRepository, redisTLSScanRepo repository.RedisTLSScanRepository, planService *service.PlanService) *DiscoveryHandler {
	return &DiscoveryHandler{
		discoveryService:    discoveryService,
		tlsService:          tlsService,
		cfgChain:            cfgChain,
		natsConn:            natsConn,
		redisWalletScanRepo: redisWalletScanRepo,
		redisTLSScanRepo:    redisTLSScanRepo,
		planService:         planService,
	}
}

// ScanRequest represents the request body for scanning a wallet or TLS endpoint
type ScanRequest struct {
	Address string `json:"address,omitempty"` // For wallet scans
	URL     string `json:"url,omitempty"`     // For TLS endpoint scans
}

// UnifiedScan handles POST /discovery/scan
// Automatically detects if the request is for a wallet (address) or TLS endpoint (url)
func (h *DiscoveryHandler) UnifiedScan(c *fiber.Ctx) error {
	var req ScanRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Determine scan type based on provided fields
	if req.Address != "" && req.URL != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "cannot specify both address and url, please provide only one",
		})
	}

	if req.Address == "" && req.URL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "either address (for wallet) or url (for TLS endpoint) is required",
		})
	}

	// Route to appropriate handler based on what was provided
	if req.Address != "" {
		// Wallet scan
		return h.scanWallet(c, req.Address)
	} else {
		// TLS endpoint scan
		return h.scanTLS(c, req.URL)
	}
}

// scanWallet handles wallet scanning (extracted from original Scan method)
func (h *DiscoveryHandler) scanWallet(c *fiber.Ctx, address string) error {
	if address == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "address is required",
		})
	}

	// Get user ID from JWT context (set by middleware)
	// For anonymous users, userID will be uuid.Nil
	userIDValue := c.Locals("user_id")
	var userID uuid.UUID
	var isAnonymous bool

	if userIDValue == nil {
		// Anonymous user - use uuid.Nil
		userID = uuid.Nil
		isAnonymous = true
	} else {
		var ok bool
		userID, ok = userIDValue.(uuid.UUID)
		if !ok {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "invalid user ID format",
			})
		}
		isAnonymous = userID == uuid.Nil
	}

	// For anonymous users, check scan limit in Redis
	if isAnonymous {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "missing authorization header for anonymous scan",
			})
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid authorization header format",
			})
		}

		token := parts[1]
		tokenHash := repository.HashToken(token)

		// Check anonymous scan limit
		count, err := h.redisWalletScanRepo.Count(c.Context(), tokenHash)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("failed to check scan limit: %v", err),
			})
		}

		maxScans := repository.GetMaxAnonymousWalletScans()
		if count >= maxScans {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": fmt.Sprintf("anonymous wallet scan limit reached (%d/%d). Please sign in to continue", count, maxScans),
			})
		}
	} else {
		// For authenticated users, check plan limits
		if h.planService != nil {
			canScan, usage, err := h.planService.CheckScanLimit(userID, "wallet", nil, nil)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("failed to check plan limits: %v", err),
				})
			}
			if !canScan {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"error": fmt.Sprintf("wallet scan limit reached (%d/%d). Please upgrade your plan to continue", usage.WalletScansUsed, usage.WalletScanLimit),
				})
			}
		}
	}

	// Validate and normalize the Ethereum address before queuing
	// This ensures we return an error immediately if the address is invalid
	normalizedAddress, err := h.discoveryService.ValidateAndNormalizeAddress(address)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Publish scan request to NATS for async processing
	scanMsg := nats.WalletScanMessage{
		UserID:  userID,
		Address: normalizedAddress,
	}

	var subject string
	if isAnonymous {
		// Anonymous users: use Redis queue
		// Extract token from Authorization header for anonymous users to create unique Redis keys
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				scanMsg.Token = parts[1]
			}
		}
		subject = nats.SubjectWalletScanAnonymous
	} else {
		// Authenticated users: use PostgreSQL queue
		subject = nats.SubjectWalletScan
	}

	if err := nats.PublishJSON(h.natsConn, subject, scanMsg); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to queue scan request",
		})
	}

	// Return immediate response - scan will be processed asynchronously
	return c.JSON(fiber.Map{
		"message": "scan queued successfully",
		"address": normalizedAddress,
		"type":    "wallet",
		"status":  "processing",
	})
}

// scanTLS handles TLS endpoint scanning (uses TLSHandler logic)
func (h *DiscoveryHandler) scanTLS(c *fiber.Ctx, endpointURL string) error {
	// Validate URL format (should start with https:// or http://)
	if !strings.HasPrefix(endpointURL, "https://") && !strings.HasPrefix(endpointURL, "http://") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "url must use http:// or https:// protocol",
		})
	}

	// Validate that the URL is well-formed and can be parsed
	parsedURL, err := url.Parse(endpointURL)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("invalid URL format: %v", err),
		})
	}

	// Check that the URL has a valid hostname
	if parsedURL.Host == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "url must include a valid hostname",
		})
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "url must include a valid hostname",
		})
	}

	// Get user ID from JWT context
	userIDValue := c.Locals("user_id")
	var userID uuid.UUID
	var isAnonymous bool

	if userIDValue == nil {
		// Anonymous user - use uuid.Nil
		userID = uuid.Nil
		isAnonymous = true
	} else {
		var ok bool
		userID, ok = userIDValue.(uuid.UUID)
		if !ok {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "invalid user ID format",
			})
		}
		isAnonymous = userID == uuid.Nil
	}

	// For anonymous users, check scan limit in Redis
	if isAnonymous {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "missing authorization header for anonymous scan",
			})
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid authorization header format",
			})
		}

		token := parts[1]
		tokenHash := repository.HashToken(token)

		// Check anonymous scan limit (using TLS scan repo)
		count, err := h.redisTLSScanRepo.Count(c.Context(), tokenHash)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("failed to check scan limit: %v", err),
			})
		}

		maxScans := repository.GetMaxAnonymousWalletScans() // Use same limit as wallet scans
		if count >= maxScans {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": fmt.Sprintf("anonymous TLS scan limit reached (%d/%d). Please sign in to continue", count, maxScans),
			})
		}
	} else {
		// For authenticated users, check plan limits
		if h.planService != nil {
			canScan, usage, err := h.planService.CheckScanLimit(userID, "endpoint", nil, nil)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("failed to check plan limits: %v", err),
				})
			}
			if !canScan {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"error": fmt.Sprintf("endpoint scan limit reached (%d/%d). Please upgrade your plan to continue", usage.EndpointScansUsed, usage.EndpointScanLimit),
				})
			}
		}
	}

	// Publish TLS scan request to NATS for async processing
	scanMsg := nats.TLSScanMessage{
		UserID:   userID,
		Endpoint: endpointURL,
	}

	var subject string
	if isAnonymous {
		// Anonymous users: use Redis queue
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				scanMsg.Token = parts[1]
			}
		}
		subject = nats.SubjectTLSScanAnonymous
	} else {
		// Authenticated users: use PostgreSQL queue
		subject = nats.SubjectTLSScan
	}

	if err := nats.PublishJSON(h.natsConn, subject, scanMsg); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to queue scan request",
		})
	}

	// Return immediate response - scan will be processed asynchronously
	return c.JSON(fiber.Map{
		"message":  "scan queued successfully",
		"endpoint": endpointURL,
		"type":     "tls",
		"status":   "processing",
	})
}

// Scan handles POST /discovery/scan/wallet (kept for backward compatibility)
func (h *DiscoveryHandler) Scan(c *fiber.Ctx) error {
	var req ScanRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.Address == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "address is required",
		})
	}

	return h.scanWallet(c, req.Address)
}

// ListRPCs handles GET /discovery/rpcs
// Returns the list of configured RPC endpoints
func (h *DiscoveryHandler) ListRPCs(c *fiber.Ctx) error {
	rpcs := make([]fiber.Map, 0, len(h.cfgChain.Blockchains))
	for _, blockchain := range h.cfgChain.Blockchains {
		rpcs = append(rpcs, fiber.Map{
			"name": blockchain.Name,
			"rpc":  blockchain.RPC,
		})
	}

	return c.JSON(fiber.Map{
		"blockchains": rpcs,
		"count":       len(rpcs),
	})
}

// ListAnonymousScans handles GET /discovery/scans/anonymous
// Returns the list of CBOMs for anonymous wallet scan results from Redis for the current user's token
func (h *DiscoveryHandler) ListAnonymousScans(c *fiber.Ctx) error {
	// Extract token from Authorization header to get the unique session identifier
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "missing authorization header",
		})
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid authorization header format",
		})
	}

	token := parts[1]
	tokenHash := repository.HashToken(token)

	// Get anonymous scans from Redis for this specific token
	anonymousResults, err := h.redisWalletScanRepo.ListAll(c.Context(), tokenHash)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("failed to fetch anonymous scans: %v", err),
		})
	}

	// Convert scan results to CBOMs
	cboms := make([]fiber.Map, len(anonymousResults))
	for i, result := range anonymousResults {
		cboms[i] = h.scanResultToCBOM(result)
	}

	return c.JSON(fiber.Map{
		"results": cboms,
		"total":   len(cboms),
		"count":   len(cboms),
	})
}

// ListScans handles GET /discovery/scans
// Returns the list of CBOMs for wallet scans for the authenticated user with pagination
func (h *DiscoveryHandler) ListScans(c *fiber.Ctx) error {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		return err
	}

	limit, offset := parsePaginationParams(c)

	// Get scan results from service
	results, total, err := h.discoveryService.ListScanResults(c.Context(), userID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Convert scan results to CBOMs
	cboms := make([]fiber.Map, len(results))
	for i, result := range results {
		cboms[i] = h.scanResultToCBOM(result)
	}

	return c.JSON(fiber.Map{
		"results": cboms,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
		"count":   len(cboms),
	})
}

// scanResultToCBOM converts a ScanResult to a CBOM format (CycloneDX v1.7 compliant)
func (h *DiscoveryHandler) scanResultToCBOM(scanResult *domain.ScanResult) fiber.Map {
	// Build CBOM component with NIST SP 800-57 key states
	component := fiber.Map{
		"type":               "cryptographic-primitive",
		"name":               scanResult.Algorithm,
		"nist_level":         scanResult.NISTLevel,
		"quantum_vulnerable": scanResult.NISTLevel <= 1,
		"key_exposed":        scanResult.KeyExposed,
		"assetType":          "related-crypto-material",
		"state":              "active", // NIST SP 800-57 key state
	}

	// Add custom state for quantum-vulnerable keys (non-NIST extension)
	if scanResult.NISTLevel <= 1 {
		component["customStates"] = []fiber.Map{
			{
				"name":        "quantum-vulnerable",
				"description": "Key relies on cryptographic algorithms considered vulnerable to future cryptographic quantum attacks",
			},
		}
	}

	// Format timestamp for metadata (use scanned_at if available, otherwise current time)
	var timestamp string
	if !scanResult.ScannedAt.IsZero() {
		timestamp = scanResult.ScannedAt.Format(time.RFC3339)
	} else {
		timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	return fiber.Map{
		"address":     scanResult.Address,
		"type":        scanResult.Type,
		"algorithm":   scanResult.Algorithm,
		"nist_level":  scanResult.NISTLevel,
		"key_exposed": scanResult.KeyExposed,
		"risk_score":  scanResult.RiskScore,
		"first_seen":  scanResult.FirstSeen,
		"last_seen":   scanResult.LastSeen,
		"networks":    scanResult.Networks,
		"scanned_at":  scanResult.ScannedAt,
		"cbom": fiber.Map{
			"bomFormat":   "CycloneDX",
			"specVersion": "1.7",
			"version":     1,
			"metadata": fiber.Map{
				"timestamp": timestamp,
			},
			"type":       "wallet",
			"components": []fiber.Map{component},
		},
	}
}

// GetCBOM handles GET /discovery/cbom/*
// Returns the CBOM (Cryptographic Bill of Materials) for a wallet address or TLS endpoint
// Automatically detects if the parameter is a wallet address (0x...) or a URL (http://... or https://...)
func (h *DiscoveryHandler) GetCBOM(c *fiber.Ctx) error {
	// Get the wildcard parameter (everything after /cbom/)
	param := c.Params("*")
	if param == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "address or url parameter is required",
		})
	}

	// Remove leading slash if present
	param = strings.TrimPrefix(param, "/")

	// Get user ID from JWT context
	userID, err := getUserIDFromContext(c)
	if err != nil {
		return err
	}

	// Detect if it's a wallet address (starts with 0x) or a URL (starts with http:// or https://)
	if strings.HasPrefix(param, "0x") {
		// It's a wallet address
		return h.getWalletCBOM(c, param, userID)
	} else if strings.HasPrefix(param, "http://") || strings.HasPrefix(param, "https://") {
		// It's a TLS endpoint URL
		return h.getTLSCBOM(c, param, userID)
	} else {
		// Try to decode as URL-encoded (for URLs passed as path parameter)
		decodedParam, err := url.QueryUnescape(param)
		if err == nil && (strings.HasPrefix(decodedParam, "http://") || strings.HasPrefix(decodedParam, "https://")) {
			return h.getTLSCBOM(c, decodedParam, userID)
		}
		// If it doesn't start with 0x, try to treat it as a wallet address anyway
		return h.getWalletCBOM(c, param, userID)
	}
}

// getWalletCBOM retrieves CBOM for a wallet address
func (h *DiscoveryHandler) getWalletCBOM(c *fiber.Ctx, address string, userID uuid.UUID) error {
	// Normalize the address
	normalizedAddress, err := h.discoveryService.ValidateAndNormalizeAddress(address)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Get scan result for this address
	scanResult, err := h.discoveryService.GetScanByAddress(c.Context(), userID, normalizedAddress)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "scan result not found for this wallet address",
		})
	}

	// Build CBOM component with NIST SP 800-57 key states
	component := fiber.Map{
		"type":               "cryptographic-primitive",
		"name":               scanResult.Algorithm,
		"nist_level":         scanResult.NISTLevel,
		"quantum_vulnerable": scanResult.NISTLevel <= 1,
		"key_exposed":        scanResult.KeyExposed,
		"assetType":          "related-crypto-material",
		"state":              "active", // NIST SP 800-57 key state
	}

	// Add custom state for quantum-vulnerable keys (non-NIST extension)
	if scanResult.NISTLevel <= 1 {
		component["customStates"] = []fiber.Map{
			{
				"name":        "quantum-vulnerable",
				"description": "Key relies on cryptographic algorithms considered vulnerable to future cryptographic quantum attacks",
			},
		}
	}

	// Format timestamp for metadata (use scanned_at if available, otherwise current time)
	var timestamp string
	if !scanResult.ScannedAt.IsZero() {
		timestamp = scanResult.ScannedAt.Format(time.RFC3339)
	} else {
		timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	// Build CBOM response with CycloneDX v1.7 metadata
	cbom := fiber.Map{
		"address":     scanResult.Address,
		"type":        scanResult.Type,
		"algorithm":   scanResult.Algorithm,
		"nist_level":  scanResult.NISTLevel,
		"key_exposed": scanResult.KeyExposed,
		"risk_score":  scanResult.RiskScore,
		"first_seen":  scanResult.FirstSeen,
		"last_seen":   scanResult.LastSeen,
		"networks":    scanResult.Networks,
		"scanned_at":  scanResult.ScannedAt,
		"cbom": fiber.Map{
			"bomFormat":   "CycloneDX",
			"specVersion": "1.7",
			"version":     1,
			"metadata": fiber.Map{
				"timestamp": timestamp,
			},
			"type":       "wallet",
			"components": []fiber.Map{component},
		},
	}

	return c.JSON(cbom)
}

// getTLSCBOM retrieves CBOM for a TLS endpoint
func (h *DiscoveryHandler) getTLSCBOM(c *fiber.Ctx, url string, userID uuid.UUID) error {
	// Validate URL format
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "url must use http:// or https:// protocol",
		})
	}

	// Get TLS scan result for this URL
	tlsScanResult, err := h.tlsService.GetTLSScanByURL(c.Context(), userID, url)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "TLS scan result not found for this endpoint",
		})
	}

	// Build CBOM components from TLS scan result
	components := []fiber.Map{}

	// Add certificate component
	cert := tlsScanResult.Certificate
	if cert.Subject != "" || cert.Issuer != "" {
		components = append(components, fiber.Map{
			"type":                 "certificate",
			"subject":              cert.Subject,
			"issuer":               cert.Issuer,
			"signature_algorithm":  cert.SignatureAlgorithm,
			"public_key_algorithm": cert.PublicKeyAlgorithm,
			"key_size":             cert.KeySize,
			"nist_level":           cert.NISTLevel,
			"quantum_vulnerable":   cert.NISTLevel <= 1,
			"pqc_ready":            cert.IsPQCReady,
			"not_before":           cert.NotBefore,
			"not_after":            cert.NotAfter,
		})
	}

	// Add key exchange component
	if tlsScanResult.KexAlgorithm != "" {
		kexNISTLevel := 1 // Default
		if levels, ok := tlsScanResult.NISTLevels["kex"]; ok {
			kexNISTLevel = levels
		}
		components = append(components, fiber.Map{
			"type":               "key-exchange",
			"algorithm":          tlsScanResult.KexAlgorithm,
			"pqc_ready":          tlsScanResult.KexPQCReady,
			"nist_level":         kexNISTLevel,
			"quantum_vulnerable": kexNISTLevel <= 1,
		})
	}

	// Add signature algorithm component (from certificate)
	if cert.SignatureAlgorithm != "" {
		sigNISTLevel := cert.NISTLevel
		if levels, ok := tlsScanResult.NISTLevels["sig"]; ok {
			sigNISTLevel = domain.NISTLevel(levels)
		}
		components = append(components, fiber.Map{
			"type":               "signature-algorithm",
			"name":               cert.SignatureAlgorithm,
			"nist_level":         sigNISTLevel,
			"quantum_vulnerable": sigNISTLevel <= 1,
		})
	}

	// Add cipher suites
	if len(tlsScanResult.CipherSuites) > 0 {
		for _, cs := range tlsScanResult.CipherSuites {
			components = append(components, fiber.Map{
				"type":               "cipher-suite",
				"name":               cs.Name,
				"key_exchange":       cs.KeyExchange,
				"encryption":         cs.Encryption,
				"mac":                cs.MAC,
				"nist_level":         cs.NISTLevel,
				"quantum_vulnerable": cs.NISTLevel <= 1,
				"pqc_ready":          cs.IsPQCReady,
			})
		}
	}

	// Format timestamp for metadata (ISO-8601 UTC) - use scanned_at if available, otherwise current time
	var timestamp string
	if !tlsScanResult.ScannedAt.IsZero() {
		timestamp = tlsScanResult.ScannedAt.Format(time.RFC3339)
	} else {
		timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	// Build CBOM response with CycloneDX v1.7 metadata and lifecycle
	cbom := fiber.Map{
		"url":             tlsScanResult.URL,
		"host":            tlsScanResult.Host,
		"port":            tlsScanResult.Port,
		"protocol":        tlsScanResult.ProtocolVersion,
		"nist_level":      tlsScanResult.NISTLevel,
		"risk_score":      tlsScanResult.RiskScore,
		"pqc_risk":        tlsScanResult.PQCRisk,
		"pqc_mode":        tlsScanResult.PQCMode,
		"supported_pqc":   tlsScanResult.SupportedPQCs,
		"recommendations": tlsScanResult.Recommendations,
		"scanned_at":      tlsScanResult.ScannedAt,
		"certificate":     cert,
		"cipher_suites":   tlsScanResult.CipherSuites,
		"kex_algorithm":   tlsScanResult.KexAlgorithm,
		"kex_pqc_ready":   tlsScanResult.KexPQCReady,
		"pfs":             tlsScanResult.PFS,
		"ocsp_stapled":    tlsScanResult.OCSPStapled,
		"nist_levels":     tlsScanResult.NISTLevels,
		"cbom": fiber.Map{
			"bomFormat":   "CycloneDX",
			"specVersion": "1.7",
			"version":     1,
			"metadata": fiber.Map{
				"timestamp": timestamp,
				"lifecycles": []fiber.Map{
					{
						"phase":       "discovery",
						"description": "Point-in-time cryptographic discovery of live TLS endpoints observed over the network",
					},
				},
			},
			"type":       "tls-endpoint",
			"components": components,
		},
	}

	return c.JSON(cbom)
}
