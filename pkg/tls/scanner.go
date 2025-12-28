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
	pqcInfo, errPQC := s.scanPQCInfo(host, port, state.Version)
	if errPQC == nil {
		info.PQCInfo = pqcInfo
	}
	// Don't fail if PQC scan fails - we still have the Go TLS info

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
	pqcInfo, err := ScanPQC(host, port, "", false)
	if err == nil && (pqcInfo.KexPQCReady || pqcInfo.PQCMode == "hybrid" || pqcInfo.PQCMode == "pure") {
		return pqcInfo, nil
	}

	// If no PQC detected, try with specific PQC groups
	for _, group := range DefaultPQCGroups {
		info, err := ScanPQC(host, port, group, false)
		if err == nil && (info.KexPQCReady || info.PQCMode == "hybrid" || info.PQCMode == "pure") {
			return info, nil
		}
	}

	// Return the first result even if no PQC was detected
	return pqcInfo, nil
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
