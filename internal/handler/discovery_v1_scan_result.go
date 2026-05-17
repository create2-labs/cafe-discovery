package handler

import (
	"time"

	"cafe-discovery/internal/config"
	"cafe-discovery/internal/domain"

	"github.com/gofiber/fiber/v2"
)

func walletScanResultV1(e *domain.ScanResultEntity, cfg *config.ChainConfig) fiber.Map {
	dto := e.ToScanResult()
	networks := parseNetworksFromEntity(e.Networks)
	if networks == nil {
		networks = []string{}
	}
	return fiber.Map{
		"target_address":     e.Address,
		"chain_ids":          chainIDsForNetworks(e.Networks, cfg),
		"wallet_type":        walletAccountTypeV1(e.Type, e.IsEOA, e.IsERC4337),
		"current_pq_posture": nistLevelToPQPosture(e.NISTLevel),
		"observations":       []any{},
		// UI parity (fields historically served by GET /discovery/cbom/{address})
		"algorithm":   string(e.Algorithm),
		"nist_level":  int(e.NISTLevel),
		"risk_score":  e.RiskScore,
		"key_exposed": e.KeyExposed,
		"type":        string(e.Type),
		"networks":    networks,
		"first_seen":  formatTimeRFC3339Nano(dto.FirstSeen),
		"last_seen":   formatTimeRFC3339Nano(dto.LastSeen),
		"scanned_at":  formatTimeRFC3339Nano(&dto.ScannedAt),
	}
}

func tlsScanResultBodyV1(ent *domain.TLSScanResultEntity) fiber.Map {
	dto := ent.ToTLSScanResult()
	certSummary := tlsCertificateSummaryV1(dto.Certificate)
	cipher := ""
	kex := ""
	if len(dto.CipherSuites) > 0 {
		cipher = dto.CipherSuites[0].Name
		kex = dto.CipherSuites[0].KeyExchange
	}
	cipherSuites := dto.CipherSuites
	if cipherSuites == nil {
		cipherSuites = []domain.CipherSuiteInfo{}
	}
	supportedPQCs := dto.SupportedPQCs
	if supportedPQCs == nil {
		supportedPQCs = []string{}
	}
	recommendations := dto.Recommendations
	if recommendations == nil {
		recommendations = []string{}
	}
	nistLevels := dto.NISTLevels
	if nistLevels == nil {
		nistLevels = map[string]int{}
	}
	return fiber.Map{
		"endpoint":            dto.URL,
		"tls_version":         dto.ProtocolVersion,
		"cipher_suite":        cipher,
		"key_exchange":        kex,
		"certificate_summary": certSummary,
		"current_pq_posture":  nistLevelToPQPosture(dto.NISTLevel),
		"observations":        []any{},
		// UI parity (fields historically served by GET /discovery/cbom/{url})
		"url":              dto.URL,
		"host":             dto.Host,
		"port":             dto.Port,
		"protocol_version": dto.ProtocolVersion,
		"nist_level":       int(dto.NISTLevel),
		"risk_score":       dto.RiskScore,
		"pqc_risk":         dto.PQCRisk,
		"pqc_mode":         dto.PQCMode,
		"supported_pqc":    supportedPQCs,
		"recommendations":  recommendations,
		"scanned_at":       formatTimeRFC3339Nano(&dto.ScannedAt),
		"default":          dto.Default,
		"certificate":      dto.Certificate,
		"cipher_suites":    cipherSuites,
		"kex_algorithm":    dto.KexAlgorithm,
		"kex_pqc_ready":    dto.KexPQCReady,
		"pfs":              dto.PFS,
		"ocsp_stapled":     dto.OCSPStapled,
		"nist_levels":      nistLevels,
		"alpn":             dto.ALPN,
		"curve":            dto.Curve,
	}
}

func tlsCertificateSummaryV1(cert domain.CertificateInfo) fiber.Map {
	return fiber.Map{
		"subject":             cert.Subject,
		"issuer":              cert.Issuer,
		"signature_algorithm": cert.SignatureAlgorithm,
		"not_after":           formatTimeRFC3339Nano(&cert.NotAfter),
	}
}

func formatTimeRFC3339Nano(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
