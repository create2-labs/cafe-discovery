package tls

import (
	"crypto/rsa"
	"crypto/x509"
	"strings"

	"cafe-discovery/internal/domain"
)

// PQCRules defines PQC (Post-Quantum Cryptography) classification rules
type PQCRules struct {
	// Map of signature algorithms to NIST levels
	SignatureAlgorithms map[string]domain.NISTLevel
	// Map of public key algorithms to NIST levels
	PublicKeyAlgorithms map[string]domain.NISTLevel
	// Map of key exchange methods to NIST levels
	KeyExchangeMethods map[string]domain.NISTLevel
	// PQC algorithm identifiers
	PQCAlgorithms []string
}

// NewPQCRules creates default PQC classification rules
func NewPQCRules() *PQCRules {
	return &PQCRules{
		SignatureAlgorithms: map[string]domain.NISTLevel{
			"SHA1WithRSA":     domain.NISTLevel1, // Quantum-broken
			"SHA256WithRSA":   domain.NISTLevel1, // Quantum-broken
			"SHA384WithRSA":   domain.NISTLevel1, // Quantum-broken
			"SHA512WithRSA":   domain.NISTLevel1, // Quantum-broken
			"ECDSAWithSHA1":   domain.NISTLevel1, // Quantum-broken
			"ECDSAWithSHA256": domain.NISTLevel1, // Quantum-broken
			"ECDSAWithSHA384": domain.NISTLevel1, // Quantum-broken
			"ECDSAWithSHA512": domain.NISTLevel1, // Quantum-broken
			"Ed25519":         domain.NISTLevel3, // Quantum-resistant (not standardized yet)
			// PQC algorithms (will be added as they become standard)
			"Dilithium": domain.NISTLevel5, // NIST PQC Standard
			"Falcon":    domain.NISTLevel5, // NIST PQC Standard
			"SPHINCS":   domain.NISTLevel5, // NIST PQC Standard
		},
		PublicKeyAlgorithms: map[string]domain.NISTLevel{
			"RSA":      domain.NISTLevel1, // Quantum-broken (< 3072 bits)
			"RSA-2048": domain.NISTLevel1, // Quantum-broken
			"RSA-3072": domain.NISTLevel2, // Better but still vulnerable
			"RSA-4096": domain.NISTLevel2, // Better but still vulnerable
			"ECDSA":    domain.NISTLevel1, // Quantum-broken (all curves)
			"Ed25519":  domain.NISTLevel3, // Quantum-resistant
			"Ed448":    domain.NISTLevel3, // Quantum-resistant
			// PQC algorithms
			"Dilithium": domain.NISTLevel5,
			"Falcon":    domain.NISTLevel5,
			"SPHINCS":   domain.NISTLevel5,
		},
		KeyExchangeMethods: map[string]domain.NISTLevel{
			"RSA":      domain.NISTLevel1, // Quantum-broken
			"DH":       domain.NISTLevel1, // Quantum-broken
			"ECDH":     domain.NISTLevel1, // Quantum-broken
			"ECDHE":    domain.NISTLevel1, // Quantum-broken
			"DHE":      domain.NISTLevel1, // Quantum-broken
			"TLS 1.3":  domain.NISTLevel3, // Supports PQC in future
			"Kyber":    domain.NISTLevel5, // NIST PQC Standard
			"FrodoKEM": domain.NISTLevel5, // Alternative PQC
			"SIKE":     domain.NISTLevel5, // NIST PQC Standard (deprecated)
			"BIKE":     domain.NISTLevel5, // Alternative PQC
		},
		PQCAlgorithms: []string{
			"Dilithium", "Falcon", "SPHINCS", "Kyber", "FrodoKEM", "SIKE", "BIKE",
		},
	}
}

// ClassifyCertificate classifies a certificate according to PQC rules
func (r *PQCRules) ClassifyCertificate(cert *x509.Certificate) (domain.NISTLevel, bool) {
	// Check signature algorithm
	sigAlg := cert.SignatureAlgorithm.String()
	level, exists := r.SignatureAlgorithms[sigAlg]
	if exists {
		return level, level >= domain.NISTLevel5
	}

	// Check public key algorithm
	pubKeyAlg := cert.PublicKeyAlgorithm.String()
	level, exists = r.PublicKeyAlgorithms[pubKeyAlg]
	if exists {
		return level, level >= domain.NISTLevel5
	}

	// Determine from key size for RSA
	if pubKeyAlg == "RSA" {
		keySize := 0
		if rsaKey, ok := cert.PublicKey.(*rsa.PublicKey); ok {
			keySize = rsaKey.N.BitLen()
		}

		if keySize < 2048 {
			return domain.NISTLevel1, false
		} else if keySize < 3072 {
			return domain.NISTLevel1, false
		} else if keySize < 4096 {
			return domain.NISTLevel2, false
		} else {
			return domain.NISTLevel2, false
		}
	}

	// Default: assume quantum-broken
	return domain.NISTLevel1, false
}

// ClassifyCipherSuite classifies a cipher suite according to PQC rules
func (r *PQCRules) ClassifyCipherSuite(cipherName string) (domain.NISTLevel, bool) {
	keyExchange, _, _ := ParseCipherSuite(cipherName)

	// Check if it's a PQC cipher suite
	for _, pqc := range r.PQCAlgorithms {
		if strings.Contains(cipherName, pqc) {
			return domain.NISTLevel5, true
		}
	}

	// Check key exchange method
	level, exists := r.KeyExchangeMethods[keyExchange]
	if exists {
		return level, level >= domain.NISTLevel5
	}

	// TLS 1.3 uses different key exchange, generally more secure
	if strings.Contains(cipherName, "TLS_AES") || strings.Contains(cipherName, "TLS_CHACHA20") {
		return domain.NISTLevel3, false // TLS 1.3 supports PQC extensions
	}

	// Default: assume quantum-broken
	return domain.NISTLevel1, false
}

// IsPQCAlgorithm checks if an algorithm name is a known PQC algorithm
func (r *PQCRules) IsPQCAlgorithm(alg string) bool {
	for _, pqc := range r.PQCAlgorithms {
		if strings.Contains(strings.ToLower(alg), strings.ToLower(pqc)) {
			return true
		}
	}
	return false
}
