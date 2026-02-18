package handler

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"
	"cafe-discovery/internal/service"
	"cafe-discovery/pkg/nats"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// TLSHandler handles TLS-related HTTP requests. Redis only for scan data; no Postgres.
type TLSHandler struct {
	tlsService   *service.TLSService
	natsConn     nats.Connection
	redisTLSRepo repository.RedisTLSScanRepository
	planService  *service.PlanService
}

// NewTLSHandler creates a new TLS handler (Redis-only for scan data).
func NewTLSHandler(tlsService *service.TLSService, natsConn nats.Connection, redisTLSRepo repository.RedisTLSScanRepository, planService *service.PlanService) *TLSHandler {
	return &TLSHandler{
		tlsService:   tlsService,
		natsConn:     natsConn,
		redisTLSRepo: redisTLSRepo,
		planService:  planService,
	}
}

// ScanRequest represents the request body for scanning a TLS endpoint
type TLSScanRequest struct {
	URL string `json:"url"`
}

// Scan handles POST /discovery/scan/endpoints
func (h *TLSHandler) Scan(c *fiber.Ctx) error {
	var req TLSScanRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	userID, err := requireAuthenticatedUserID(c)
	if err != nil {
		return err
	}

	if req.URL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "url is required",
		})
	}

	// Validate URL format (should start with https:// and be a valid URL)
	if !strings.HasPrefix(req.URL, "https://") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "url must use https:// protocol",
		})
	}

	// Validate that the URL is well-formed and can be parsed
	// This catches issues like invalid hostnames before they reach the scanner
	parsedURL, err := url.Parse(req.URL)
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

	// Basic validation: hostname should not be empty after parsing
	hostname := parsedURL.Hostname()
	if hostname == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "url must include a valid hostname",
		})
	}

	if h.planService != nil {
		endpointCount, _ := h.redisTLSRepo.CountByUserID(c.Context(), userID.String())
		canScan, usage, err := h.planService.CheckScanLimitFromCounts(userID, "endpoint", 0, endpointCount)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("failed to check plan limits: %v", err)})
		}
		if !canScan {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": fmt.Sprintf("endpoint scan limit reached (%d/%d). Please upgrade your plan to continue", usage.EndpointScansUsed, usage.EndpointScanLimit),
			})
		}
	}

	// Publish scan request to NATS (scanners subscribe to scan.requested.tls)
	scanMsg := nats.TLSScanMessage{
		ScanID:   uuid.New(),
		UserID:   userID,
		Endpoint: req.URL,
	}
	log.Info().
		Str("subject", nats.SubjectScanRequestedTLS).
		Str("scan_id", scanMsg.ScanID.String()).
		Str("endpoint", scanMsg.Endpoint).
		Str("component", "backend").
		Msg("NATS → PUB scan.requested.tls")
	if err := nats.PublishJSON(h.natsConn, nats.SubjectScanRequestedTLS, scanMsg); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to queue scan request",
		})
	}

	// Return immediate response - scan will be processed asynchronously
	return c.JSON(fiber.Map{
		"message":  "scan queued successfully",
		"endpoint": req.URL,
		"status":   "processing",
	})
}

// ListScans handles GET /discovery/tls/scans. Redis only; no Postgres fallback.
func (h *TLSHandler) ListScans(c *fiber.Ctx) error {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		return err
	}
	limit, offset := parsePaginationParams(c)
	urls, err := h.redisTLSRepo.ListURLsByUserID(c.Context(), userID.String())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	total := int64(len(urls))
	if offset > len(urls) {
		urls = nil
	} else {
		urls = urls[offset:]
	}
	if limit > 0 && len(urls) > limit {
		urls = urls[:limit]
	}
	ids := make([]fiber.Map, len(urls))
	for i, u := range urls {
		ids[i] = fiber.Map{"id": u}
	}
	return c.JSON(fiber.Map{
		"results": ids,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
		"count":   len(ids),
	})
}

// tlsScanResultToCBOM converts a TLSScanResult to a CBOM format
func (h *TLSHandler) tlsScanResultToCBOM(tlsScanResult *domain.TLSScanResult) fiber.Map {
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
	return fiber.Map{
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
}
