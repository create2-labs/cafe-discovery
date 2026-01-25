package tls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// Scanner scans TLS certificates and cipher suites
type Scanner struct {
	timeout time.Duration
}

// NewScanner creates a new TLS scanner
func NewScanner() *Scanner {
	return &Scanner{
		timeout: 10 * time.Second,
	}
}

// ScanURL scans a URL and returns TLS connection information
func (s *Scanner) ScanURL(ctx context.Context, targetURL string) (*TLSInfo, error) {
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	host := parsedURL.Hostname()
	port := parsedURL.Port()

	if port == "" {
		if parsedURL.Scheme == "https" {
			port = "443"
		} else {
			return nil, fmt.Errorf("no port specified and scheme is not https")
		}
	}

	return s.ScanHost(ctx, host, port)
}

// TLSInfo contains information about a TLS connection
type TLSInfo struct {
	Host             string
	Port             string
	Certificate      *x509.Certificate
	CipherSuites     []uint16
	ProtocolVersion  uint16
	NegotiatedCipher uint16
	ALPN             string // Application-Layer Protocol Negotiation
	OCSPStapled      bool   // OCSP stapling
	// PQC information (from OQS/OpenSSL scan)
	PQCInfo *PQCInfo
}

// ScanHost scans a host:port for TLS information
func (s *Scanner) ScanHost(ctx context.Context, host, port string) (*TLSInfo, error) {
	addr := net.JoinHostPort(host, port)

	dialer := &net.Dialer{
		Timeout: s.timeout,
	}

	// Create TLS config to get detailed information
	// #nosec G402 -- We intentionally skip verification for scanning purposes
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, // We just want to analyze, not validate
		ServerName:         host,
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			// Log but don't fail on close errors
			_ = closeErr
		}
	}()

	state := conn.ConnectionState()

	// Get all supported cipher suites from the connection
	// Note: ConnectionState doesn't expose all cipher suites, only the negotiated one
	// We'll scan with different cipher suites if needed, but for now use the negotiated one
	cipherSuites := []uint16{state.CipherSuite}

	info := &TLSInfo{
		Host:             host,
		Port:             port,
		Certificate:      state.PeerCertificates[0], // First certificate in chain
		CipherSuites:     cipherSuites,
		ProtocolVersion:  state.Version,
		NegotiatedCipher: state.CipherSuite,
		ALPN:             state.NegotiatedProtocol,
		OCSPStapled:      len(state.OCSPResponse) > 0,
	}

	// Try to get PQC information using OQS/OpenSSL
	// This is done even if Go TLS scan succeeded to get PQC-specific info
	// CRITICAL: Always attempt PQC scan for TLS 1.3 to detect hybrid KEMs
	pqcInfo, errPQC := s.scanPQCInfo(host, port, state.Version)
	if errPQC == nil && pqcInfo != nil {
		info.PQCInfo = pqcInfo
	}
	// Don't fail if PQC scan fails - we still have the Go TLS info
	// But log that PQC scan was attempted

	return info, nil
}

// scanPQCInfo performs PQC-specific scanning using OQS/OpenSSL
func (s *Scanner) scanPQCInfo(host, port string, tlsVersion uint16) (*PQCInfo, error) {
	// Only attempt PQC scan for TLS 1.3 (PQC is mainly supported in TLS 1.3)
	if tlsVersion != tls.VersionTLS13 {
		// Try anyway, but with lower priority
		pqcInfo, err := ScanPQC(host, port, "", false)
		if err == nil {
			return pqcInfo, nil
		}
		return nil, fmt.Errorf("PQC scan not applicable for TLS < 1.3")
	}

	// First try without specifying a group (let server choose)
	// This may detect PQC if server proactively offers it
	pqcInfo, err := ScanPQC(host, port, "", false)
	initialScanSuccess := err == nil
	initialHasPQC := false
	
	if initialScanSuccess {
		// Check if PQC was detected (either via KexPQCReady, PQCMode, or PQC flag)
		if pqcInfo.KexPQCReady || pqcInfo.PQC ||
			pqcInfo.PQCMode == "hybrid" || pqcInfo.PQCMode == "pure" {
			initialHasPQC = true
		}
		// Also check if kex_alg contains hybrid indicators
		if !initialHasPQC && pqcInfo.KexAlg != "" {
			algUpper := strings.ToUpper(pqcInfo.KexAlg)
			if strings.Contains(algUpper, "MLKEM") || strings.Contains(algUpper, "KYBER") ||
				strings.Contains(algUpper, "FRODO") || strings.Contains(algUpper, "BIKE") {
				// PQC component detected in algorithm name
				pqcInfo.KexPQCReady = true
				if !strings.Contains(algUpper, "HYBRID") &&
					(strings.Contains(algUpper, "X25519") || strings.Contains(algUpper, "P256") ||
						strings.Contains(algUpper, "P384") || strings.Contains(algUpper, "SECP")) {
					pqcInfo.PQCMode = "hybrid"
				} else if !strings.Contains(algUpper, "X25519") &&
					!strings.Contains(algUpper, "P256") &&
					!strings.Contains(algUpper, "P384") &&
					!strings.Contains(algUpper, "SECP") {
					pqcInfo.PQCMode = "pure"
				}
				initialHasPQC = true
			}
		}
		// Also check offered_groups for PQC indicators (server proactively offers PQC)
		if !initialHasPQC && pqcInfo.OfferedGroups != "" {
			offeredUpper := strings.ToUpper(pqcInfo.OfferedGroups)
			if strings.Contains(offeredUpper, "MLKEM") || strings.Contains(offeredUpper, "KYBER") ||
				strings.Contains(offeredUpper, "FRODO") || strings.Contains(offeredUpper, "BIKE") {
				// Server offers PQC groups - mark as PQC ready
				pqcInfo.KexPQCReady = true
				pqcInfo.PQC = true
				// Check if hybrid (contains classical + PQC) or pure
				if strings.Contains(offeredUpper, "X25519") || strings.Contains(offeredUpper, "P256") ||
					strings.Contains(offeredUpper, "P384") || strings.Contains(offeredUpper, "SECP") {
					pqcInfo.PQCMode = "hybrid"
				} else {
					pqcInfo.PQCMode = "pure"
				}
				initialHasPQC = true
			}
		}
		
		if initialHasPQC {
			return pqcInfo, nil
		}
	}

	// If no PQC detected in initial scan, try with specific hybrid PQC groups
	// This is important because servers may not proactively offer PQC,
	// but will accept it if the client requests it
	// CRITICAL: Always try hybrid groups even if initial scan succeeded
	// because server may not proactively offer PQC but will accept it if requested
	for _, group := range DefaultPQCGroups {
		info, err := ScanPQC(host, port, group, false)
		
		// CRITICAL: Check if handshake succeeded
		// Handshake succeeded if: no error OR (info exists AND has TLS version)
		handshakeSucceeded := false
		if err == nil && info != nil {
			handshakeSucceeded = true
		} else if info != nil && info.TLSVersion != "" {
			// Even if there was an error, if we have TLS version, handshake succeeded
			handshakeSucceeded = true
		}
		
		if handshakeSucceeded {
			// PRIORITY 1: Check if the requested group name indicates hybrid/PQC
			// This MUST be checked first - it's the definitive proof
			// If we requested a hybrid group and handshake succeeded, it MUST be hybrid
			groupUpper := strings.ToUpper(group)
			isHybridGroup := strings.Contains(groupUpper, "MLKEM") || strings.Contains(groupUpper, "KYBER") ||
				strings.Contains(groupUpper, "FRODO") || strings.Contains(groupUpper, "BIKE")
			
			if isHybridGroup {
				// Handshake succeeded with hybrid group request = hybrid KEM negotiated
				// This is the definitive proof - server accepted our hybrid request
				info.KexPQCReady = true
				info.PQC = true
				// Check if it's hybrid (has classical component) or pure
				if strings.Contains(groupUpper, "X25519") || strings.Contains(groupUpper, "P256") ||
					strings.Contains(groupUpper, "P384") || strings.Contains(groupUpper, "SECP") ||
					strings.Contains(groupUpper, "P521") || strings.Contains(groupUpper, "SECP256") ||
					strings.Contains(groupUpper, "SECP384") {
					info.PQCMode = "hybrid"
				} else {
					info.PQCMode = "pure"
				}
				// Use requested group as algorithm name (full hybrid name)
				if info.KexAlg == "" {
					info.KexAlg = group
				}
				return info, nil
			}
			
			// PRIORITY 2: Check explicit flags from C code (should be set if C code detected it)
			if info.PQCMode == "hybrid" || info.PQCMode == "pure" ||
				info.KexPQCReady || info.PQC {
				return info, nil
			}
			
			// PRIORITY 3: Check kex_alg for hybrid indicators (fallback)
			if info.KexAlg != "" {
				algUpper := strings.ToUpper(info.KexAlg)
				if strings.Contains(algUpper, "MLKEM") || strings.Contains(algUpper, "KYBER") ||
					strings.Contains(algUpper, "FRODO") || strings.Contains(algUpper, "BIKE") {
					// Hybrid group was requested and handshake succeeded
					info.KexPQCReady = true
					info.PQC = true
					if info.PQCMode == "" {
						// Check if it's hybrid or pure based on algorithm name
						if strings.Contains(algUpper, "X25519") || strings.Contains(algUpper, "P256") ||
							strings.Contains(algUpper, "P384") || strings.Contains(algUpper, "SECP") {
							info.PQCMode = "hybrid"
						} else {
							info.PQCMode = "pure"
						}
					}
					return info, nil
				}
			}
		}
		// If handshake failed completely, continue to next group
	}

	// Return the best result we have (even if no PQC was detected)
	// Prefer the result from hybrid group attempts over initial scan
	// because it may have more complete information
	if initialScanSuccess {
		return pqcInfo, nil
	}
	
	// If initial scan failed, return nil (no PQC info available)
	return nil, fmt.Errorf("PQC scan failed: initial scan failed and no hybrid groups succeeded")
}

// GetProtocolVersion returns the TLS protocol version as string
func GetProtocolVersion(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("Unknown (0x%04x)", version)
	}
}

// GetCipherSuiteName returns the name of a cipher suite
func GetCipherSuiteName(id uint16) string {
	name := tls.CipherSuiteName(id)
	if name == "" {
		return fmt.Sprintf("Unknown (0x%04x)", id)
	}
	return name
}

// ParseCipherSuite parses a cipher suite name to extract components
func ParseCipherSuite(name string) (keyExchange, encryption, mac string) {
	// TLS 1.3 cipher suites have different format
	if strings.Contains(name, "TLS_AES") || strings.Contains(name, "TLS_CHACHA20") {
		return "TLS 1.3", "AEAD", "AEAD"
	}

	// Parse TLS 1.2 and earlier cipher suites
	parts := strings.Split(name, "_")
	if len(parts) < 3 {
		return "Unknown", "Unknown", "Unknown"
	}

	// Format: TLS_KEYEXCHANGE_ENCRYPTION_MAC
	keyExchange = parts[1]
	encryption = parts[2]
	if len(parts) > 3 {
		mac = parts[3]
	} else {
		mac = "None"
	}

	return keyExchange, encryption, mac
}
