package cbom

import (
	"time"

	"cafe-discovery/internal/domain"
)

// TLS builds the TLS endpoint CBOM JSON envelope from a scan result DTO (W6, on-demand).
func TLS(sr *domain.TLSScanResult) map[string]any {
	if sr == nil {
		return nil
	}

	components := []map[string]any{}
	cert := sr.Certificate
	if cert.Subject != "" || cert.Issuer != "" {
		components = append(components, map[string]any{
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

	if sr.KexAlgorithm != "" {
		kexNISTLevel := 1
		if levels, ok := sr.NISTLevels["kex"]; ok {
			kexNISTLevel = levels
		}
		components = append(components, map[string]any{
			"type":               "key-exchange",
			"algorithm":          sr.KexAlgorithm,
			"pqc_ready":          sr.KexPQCReady,
			"nist_level":         kexNISTLevel,
			"quantum_vulnerable": kexNISTLevel <= 1,
		})
	}

	if cert.SignatureAlgorithm != "" {
		sigNISTLevel := cert.NISTLevel
		if levels, ok := sr.NISTLevels["sig"]; ok {
			sigNISTLevel = domain.NISTLevel(levels)
		}
		components = append(components, map[string]any{
			"type":               "signature-algorithm",
			"name":               cert.SignatureAlgorithm,
			"nist_level":         sigNISTLevel,
			"quantum_vulnerable": sigNISTLevel <= 1,
		})
	}

	for _, cs := range sr.CipherSuites {
		components = append(components, map[string]any{
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

	timestamp := sr.ScannedAt.UTC().Format(time.RFC3339)
	if sr.ScannedAt.IsZero() {
		timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	return map[string]any{
		"url":              sr.URL,
		"host":             sr.Host,
		"port":             sr.Port,
		"protocol_version": sr.ProtocolVersion,
		"nist_level":       sr.NISTLevel,
		"risk_score":       sr.RiskScore,
		"pqc_risk":         sr.PQCRisk,
		"pqc_mode":         sr.PQCMode,
		"supported_pqc":    sr.SupportedPQCs,
		"recommendations":  sr.Recommendations,
		"scanned_at":       sr.ScannedAt,
		"default":          sr.Default,
		"certificate":      cert,
		"cipher_suites":    sr.CipherSuites,
		"kex_algorithm":    sr.KexAlgorithm,
		"kex_pqc_ready":    sr.KexPQCReady,
		"pfs":              sr.PFS,
		"ocsp_stapled":     sr.OCSPStapled,
		"nist_levels":      sr.NISTLevels,
		"cbom": map[string]any{
			"bomFormat":   "CycloneDX",
			"specVersion": "1.7",
			"version":     1,
			"metadata": map[string]any{
				"timestamp": timestamp,
				"lifecycles": []map[string]any{{
					"phase":       "discovery",
					"description": "Point-in-time cryptographic discovery of live TLS endpoints observed over the network",
				}},
			},
			"type":       "tls-endpoint",
			"components": components,
		},
	}
}
