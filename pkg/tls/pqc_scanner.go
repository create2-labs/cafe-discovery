package tls

import (
	"encoding/json"
	"fmt"

	"cafe-discovery/native"
)

// PQCInfo contains PQC-specific information from OQS/OpenSSL scan
type PQCInfo struct {
	Host              string         `json:"host"`
	Port              string         `json:"port"`
	TLSVersion        string         `json:"tls_version,omitempty"`
	CipherSuite       string         `json:"cipher_suite,omitempty"`
	Group             string         `json:"group,omitempty"`
	KexAlg            string         `json:"kex_alg,omitempty"`
	KexPQCReady       bool           `json:"kex_pqc_ready,omitempty"`
	PQCMode           string         `json:"pqc_mode,omitempty"` // classical, hybrid, pure
	PQC               bool           `json:"pqc,omitempty"`
	NISTLevels        map[string]int `json:"nist_levels,omitempty"`
	CertSubject       string         `json:"cert_subject,omitempty"`
	CertIssuer        string         `json:"cert_issuer,omitempty"`
	CertNotBefore     string         `json:"cert_not_before,omitempty"`
	CertNotAfter      string         `json:"cert_not_after,omitempty"`
	CertSigAlg        string         `json:"cert_sig_alg,omitempty"`
	CertPubkeyType    string         `json:"cert_pubkey_type,omitempty"`
	CertPubkeyBits    int            `json:"cert_pubkey_bits,omitempty"`
	CertPubkeyECGroup string         `json:"cert_pubkey_ec_group,omitempty"`
	Error             string         `json:"error,omitempty"`
}

// ScanPQC scans a host:port using OQS/OpenSSL to detect PQC support
func ScanPQC(host, port, group string, trace bool) (*PQCInfo, error) {
	// Call C function get_pqc_info via native package
	jsonStr, err := native.GetPQCInfo(host, port, group, trace)
	if err != nil {
		return nil, fmt.Errorf("failed to call get_pqc_info: %w", err)
	}

	var info PQCInfo
	if err := json.Unmarshal([]byte(jsonStr), &info); err != nil {
		return nil, fmt.Errorf("failed to parse PQC info JSON: %w", err)
	}

	if info.Error != "" {
		return &info, fmt.Errorf("PQC scan error: %s", info.Error)
	}

	return &info, nil
}

// TryPQCGroups attempts to scan with different PQC groups if initial scan didn't detect PQC
func TryPQCGroups(host, port string, groups []string) (*PQCInfo, error) {
	// First try without specifying a group
	info, err := ScanPQC(host, port, "", false)
	if err == nil && (info.KexPQCReady || info.PQCMode == "hybrid" || info.PQCMode == "pure") {
		return info, nil
	}

	// Try each group in the list
	for _, group := range groups {
		info, err := ScanPQC(host, port, group, false)
		if err == nil && (info.KexPQCReady || info.PQCMode == "hybrid" || info.PQCMode == "pure") {
			return info, nil
		}
	}

	// Return the first result even if no PQC was detected
	return info, nil
}
