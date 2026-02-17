package tls

import (
	"cafe-discovery/internal/domain"
	"cafe-discovery/pkg/scan"
	"time"
)

// tlsResultAdapter wraps *domain.TLSScanResult and implements scan.ScanResult.
type tlsResultAdapter struct {
	*domain.TLSScanResult
}

func (a *tlsResultAdapter) ScanKind() string { return scan.KindTLS }
func (a *tlsResultAdapter) ScannedAt() time.Time { return a.TLSScanResult.ScannedAt }
func (a *tlsResultAdapter) Findings() []scan.Finding {
	var out []scan.Finding
	cert := a.Certificate
	if cert.Subject != "" || cert.Issuer != "" {
		out = append(out, scan.Finding{
			Type:        "certificate",
			Name:        cert.SignatureAlgorithm,
			NISTLevel:   int(cert.NISTLevel),
			QuantumVuln: cert.NISTLevel <= 1,
		})
	}
	if a.KexAlgorithm != "" {
		kexLevel := 1
		if l, ok := a.NISTLevels["kex"]; ok {
			kexLevel = l
		}
		out = append(out, scan.Finding{
			Type:        "key-exchange",
			Name:        a.KexAlgorithm,
			NISTLevel:   kexLevel,
			QuantumVuln: kexLevel <= 1,
		})
	}
	return out
}
func (a *tlsResultAdapter) Classification() string {
	switch a.PQCRisk {
	case "critical", "warning":
		return "legacy"
	default:
		return "pq-ready"
	}
}

func (a *tlsResultAdapter) ToCBOM() (map[string]any, error) {
	return tlsResultToCBOM(a.TLSScanResult), nil
}

// Raw implements scan.RawResult for persistence (scan.completed payload).
func (a *tlsResultAdapter) Raw() interface{} {
	return a.TLSScanResult
}

// tlsResultToCBOM produces the same shape as handler's tlsScanResultToCBOM (API unchanged).
func tlsResultToCBOM(t *domain.TLSScanResult) map[string]any {
	components := []map[string]any{}
	cert := t.Certificate
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
	if t.KexAlgorithm != "" {
		kexLevel := 1
		if l, ok := t.NISTLevels["kex"]; ok {
			kexLevel = l
		}
		sigLevel := int(cert.NISTLevel)
		if l, ok := t.NISTLevels["sig"]; ok {
			sigLevel = l
		}
		components = append(components, map[string]any{
			"type":               "key-exchange",
			"algorithm":          t.KexAlgorithm,
			"pqc_ready":          t.KexPQCReady,
			"nist_level":         kexLevel,
			"quantum_vulnerable": kexLevel <= 1,
		})
		if cert.SignatureAlgorithm != "" {
			components = append(components, map[string]any{
				"type":               "signature-algorithm",
				"name":               cert.SignatureAlgorithm,
				"nist_level":         sigLevel,
				"quantum_vulnerable": sigLevel <= 1,
			})
		}
	}
	for _, cs := range t.CipherSuites {
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
	timestamp := t.ScannedAt.Format(time.RFC3339)
	if t.ScannedAt.IsZero() {
		timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	return map[string]any{
		"url":             t.URL,
		"host":            t.Host,
		"port":            t.Port,
		"protocol":        t.ProtocolVersion,
		"nist_level":      t.NISTLevel,
		"risk_score":      t.RiskScore,
		"pqc_risk":        t.PQCRisk,
		"pqc_mode":        t.PQCMode,
		"supported_pqc":   t.SupportedPQCs,
		"recommendations": t.Recommendations,
		"scanned_at":      t.ScannedAt,
		"certificate":     cert,
		"cipher_suites":   t.CipherSuites,
		"kex_algorithm":   t.KexAlgorithm,
		"kex_pqc_ready":   t.KexPQCReady,
		"pfs":             t.PFS,
		"ocsp_stapled":    t.OCSPStapled,
		"nist_levels":     t.NISTLevels,
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
