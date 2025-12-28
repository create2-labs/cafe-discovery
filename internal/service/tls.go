package service

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"strconv"
	"strings"
	"time"

	"cafe-discovery/internal/domain"
	"cafe-discovery/internal/repository"
	tlspkg "cafe-discovery/pkg/tls"

	"github.com/google/uuid"
)

// TLSService handles TLS certificate scanning and analysis
type TLSService struct {
	scanner           *tlspkg.Scanner
	pqcRules          *tlspkg.PQCRules
	tlsScanResultRepo repository.TLSScanResultRepository
	planService       *PlanService
}

// NewTLSService creates a new TLS service
func NewTLSService(tlsScanResultRepo repository.TLSScanResultRepository, planService *PlanService) *TLSService {
	return &TLSService{
		scanner:           tlspkg.NewScanner(),
		pqcRules:          tlspkg.NewPQCRules(),
		tlsScanResultRepo: tlsScanResultRepo,
		planService:       planService,
	}
}

// ScanTLS scans a URL for TLS certificate and cipher suite information and saves the result for the user
// userID can be nil for default endpoints (isDefault=true)
// isDefault indicates whether this is a default endpoint (default=false for user-scanned endpoints)
func (s *TLSService) ScanTLS(ctx context.Context, userID *uuid.UUID, targetURL string, isDefault bool) (*domain.TLSScanResult, error) {
	// Check plan limits only for user-scanned endpoints (not for default endpoints)
	if !isDefault && userID != nil && s.planService != nil {
		canScan, usage, err := s.planService.CheckScanLimit(*userID, "endpoint", nil, s.tlsScanResultRepo)
		if err != nil {
			return nil, fmt.Errorf("failed to check plan limits: %w", err)
		}
		if !canScan {
			return nil, fmt.Errorf("endpoint scan limit reached (%d/%d). Please upgrade your plan to continue", usage.EndpointScansUsed, usage.EndpointScanLimit)
		}
	}

	// Scan the URL
	info, err := s.scanner.ScanURL(ctx, targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to scan TLS: %w", err)
	}

	// Classify certificate
	certLevel, isPQCCert := s.pqcRules.ClassifyCertificate(info.Certificate)

	// Extract certificate information
	certInfo := s.extractCertificateInfo(info.Certificate, certLevel, isPQCCert)

	// Extract cipher suites information
	cipherSuites := s.extractCipherSuites(info)

	// Determine overall NIST level (worst case)
	overallLevel := certLevel
	for _, cs := range cipherSuites {
		if cs.NISTLevel < overallLevel {
			overallLevel = cs.NISTLevel
		}
	}

	// Calculate risk score
	riskScore := s.calculateTLSRiskScore(certLevel, cipherSuites)

	// Determine PQC risk level
	pqcRisk := s.determinePQCRisk(overallLevel, isPQCCert)

	// Generate recommendations
	recommendations := s.generateRecommendations(certInfo, cipherSuites, overallLevel)

	// Check for supported PQC algorithms
	supportedPQCs := s.detectSupportedPQC(certInfo, cipherSuites)

	port, _ := strconv.Atoi(info.Port)

	result := &domain.TLSScanResult{
		URL:             targetURL,
		Host:            info.Host,
		Port:            port,
		Certificate:     certInfo,
		CipherSuites:    cipherSuites,
		ProtocolVersion: tlspkg.GetProtocolVersion(info.ProtocolVersion),
		NISTLevel:       overallLevel,
		RiskScore:       riskScore,
		PQCRisk:         pqcRisk,
		SupportedPQCs:   supportedPQCs,
		Recommendations: recommendations,
		ScannedAt:       time.Now(),
	}

	// Integrate PQC information from OQS/OpenSSL scan if available
	if info.PQCInfo != nil {
		result.KexAlgorithm = info.PQCInfo.KexAlg
		if result.KexAlgorithm == "" {
			result.KexAlgorithm = info.PQCInfo.Group
		}
		result.KexPQCReady = info.PQCInfo.KexPQCReady || info.PQCInfo.PQC
		result.PQCMode = info.PQCInfo.PQCMode
		result.NISTLevels = info.PQCInfo.NISTLevels
		result.Curve = info.PQCInfo.CertPubkeyECGroup

		// Update PFS from cipher suite name
		if info.PQCInfo.CipherSuite != "" {
			result.PFS = s.hasPFSFromCipherName(info.PQCInfo.CipherSuite)
		}
	}

	// Set ALPN and OCSP from Go TLS scan (available in TLSInfo)
	result.ALPN = info.ALPN
	result.OCSPStapled = info.OCSPStapled

	// Set PFS if not already set
	if !result.PFS && len(cipherSuites) > 0 {
		cipherName := tlspkg.GetCipherSuiteName(cipherSuites[0].ID)
		result.PFS = s.hasPFSFromCipherName(cipherName)
	}

	// Update overall NIST level if PQC scan provides better information
	if info.PQCInfo != nil && info.PQCInfo.NISTLevels != nil {
		// Use the worst NIST level from all components
		if kexLevel, ok := info.PQCInfo.NISTLevels["kex"]; ok && domain.NISTLevel(kexLevel) < overallLevel {
			overallLevel = domain.NISTLevel(kexLevel)
		}
		if sigLevel, ok := info.PQCInfo.NISTLevels["sig"]; ok && domain.NISTLevel(sigLevel) < overallLevel {
			overallLevel = domain.NISTLevel(sigLevel)
		}
		if cipherLevel, ok := info.PQCInfo.NISTLevels["cipher"]; ok && domain.NISTLevel(cipherLevel) < overallLevel {
			overallLevel = domain.NISTLevel(cipherLevel)
		}
		result.NISTLevel = overallLevel
	}

	// Update PQC risk based on real PQC detection
	if info.PQCInfo != nil && (result.KexPQCReady || result.PQCMode == "hybrid" || result.PQCMode == "pure") {
		result.PQCRisk = "safe"
		// Add PQC algorithms to supported list
		if result.KexAlgorithm != "" {
			supportedPQCs = append(supportedPQCs, result.KexAlgorithm)
		}
	}
	result.SupportedPQCs = supportedPQCs

	// Save TLS scan result to database
	tlsScanResultEntity := domain.FromTLSScanResult(userID, result, isDefault)
	if err := s.tlsScanResultRepo.Create(tlsScanResultEntity); err != nil {
		// Log error but don't fail the request - scan was successful
		// In production, you might want to use a logger here
		_ = err
	}

	return result, nil
}

// ListTLSScanResults lists TLS scan results for a user with pagination
// Returns both user's scans and default endpoints (default=true)
func (s *TLSService) ListTLSScanResults(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.TLSScanResult, int64, error) {
	// Get TLS scan results from repository (user's scans + default endpoints)
	entities, err := s.tlsScanResultRepo.FindByUserIDOrDefault(userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch TLS scan results: %w", err)
	}

	// Get total count for pagination (user's scans + default endpoints)
	total, err := s.tlsScanResultRepo.CountByUserIDOrDefault(userID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count TLS scan results: %w", err)
	}

	// Convert entities to domain TLSScanResult DTOs
	results := make([]*domain.TLSScanResult, len(entities))
	for i, entity := range entities {
		results[i] = entity.ToTLSScanResult()
	}

	return results, total, nil
}

// extractCertificateInfo extracts certificate information
func (s *TLSService) extractCertificateInfo(cert *x509.Certificate, level domain.NISTLevel, isPQC bool) domain.CertificateInfo {
	keySize := 0
	pubKeyAlg := cert.PublicKeyAlgorithm.String()

	// Try to get key size for RSA
	if cert.PublicKey != nil {
		if rsaKey, ok := cert.PublicKey.(*rsa.PublicKey); ok {
			keySize = rsaKey.N.BitLen()
			pubKeyAlg = fmt.Sprintf("RSA-%d", keySize)
		}
	}

	return domain.CertificateInfo{
		Subject:            cert.Subject.String(),
		Issuer:             cert.Issuer.String(),
		SignatureAlgorithm: cert.SignatureAlgorithm.String(),
		PublicKeyAlgorithm: pubKeyAlg,
		KeySize:            keySize,
		NotBefore:          cert.NotBefore,
		NotAfter:           cert.NotAfter,
		SerialNumber:       cert.SerialNumber.String(),
		NISTLevel:          level,
		IsPQCReady:         isPQC,
	}
}

// extractCipherSuites extracts cipher suite information
func (s *TLSService) extractCipherSuites(info *tlspkg.TLSInfo) []domain.CipherSuiteInfo {
	var suites []domain.CipherSuiteInfo

	for _, cipherID := range info.CipherSuites {
		cipherName := tlspkg.GetCipherSuiteName(cipherID)
		keyEx, enc, mac := tlspkg.ParseCipherSuite(cipherName)

		level, isPQC := s.pqcRules.ClassifyCipherSuite(cipherName)

		suites = append(suites, domain.CipherSuiteInfo{
			ID:          cipherID,
			Name:        cipherName,
			KeyExchange: keyEx,
			Encryption:  enc,
			MAC:         mac,
			NISTLevel:   level,
			IsPQCReady:  isPQC,
		})
	}

	return suites
}

// calculateTLSRiskScore calculates the risk score for TLS configuration
func (s *TLSService) calculateTLSRiskScore(certLevel domain.NISTLevel, cipherSuites []domain.CipherSuiteInfo) float64 {
	// Base risk from certificate level
	baseRisk := 1.0 - (float64(certLevel) * 0.15)

	// Check cipher suites
	if len(cipherSuites) == 0 {
		return 1.0 // High risk if no cipher suites
	}

	// Find worst cipher suite level
	worstLevel := certLevel
	for _, cs := range cipherSuites {
		if cs.NISTLevel < worstLevel {
			worstLevel = cs.NISTLevel
		}
	}

	// Adjust risk based on worst cipher suite
	if worstLevel < certLevel {
		baseRisk += 0.2 // Additional risk if cipher suites are weaker
	}

	// Clamp between 0.0 and 1.0
	if baseRisk > 1.0 {
		baseRisk = 1.0
	}
	if baseRisk < 0.0 {
		baseRisk = 0.0
	}

	return baseRisk
}

// determinePQCRisk determines the PQC risk category
func (s *TLSService) determinePQCRisk(level domain.NISTLevel, isPQC bool) string {
	if isPQC || level >= domain.NISTLevel5 {
		return "safe"
	}
	if level >= domain.NISTLevel3 {
		return "warning"
	}
	return "critical"
}

// generateRecommendations generates recommendations based on scan results
func (s *TLSService) generateRecommendations(cert domain.CertificateInfo, suites []domain.CipherSuiteInfo, level domain.NISTLevel) []string {
	var recommendations []string

	if level <= domain.NISTLevel1 {
		recommendations = append(recommendations, "CRITICAL: Certificate uses quantum-vulnerable algorithms. Migrate to post-quantum cryptography immediately.")
	} else if level <= domain.NISTLevel2 {
		recommendations = append(recommendations, "WARNING: Certificate may be vulnerable to quantum attacks. Consider migrating to PQC.")
	}

	if !cert.IsPQCReady {
		recommendations = append(recommendations, "Upgrade certificate to use post-quantum signature algorithms (e.g., Dilithium, Falcon).")
	}

	// Check cipher suites
	hasWeakCipher := false
	for _, cs := range suites {
		if cs.NISTLevel <= domain.NISTLevel1 && !cs.IsPQCReady {
			hasWeakCipher = true
			break
		}
	}

	if hasWeakCipher {
		recommendations = append(recommendations, "Disable weak cipher suites and prefer TLS 1.3 with PQC key exchange algorithms.")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "TLS configuration appears quantum-resistant. Continue monitoring PQC standards updates.")
	}

	return recommendations
}

// detectSupportedPQC detects if any PQC algorithms are supported
func (s *TLSService) detectSupportedPQC(cert domain.CertificateInfo, suites []domain.CipherSuiteInfo) []string {
	var supported []string

	if cert.IsPQCReady {
		if s.pqcRules.IsPQCAlgorithm(cert.PublicKeyAlgorithm) {
			supported = append(supported, cert.PublicKeyAlgorithm)
		}
		if s.pqcRules.IsPQCAlgorithm(cert.SignatureAlgorithm) {
			supported = append(supported, cert.SignatureAlgorithm)
		}
	}

	for _, cs := range suites {
		if cs.IsPQCReady {
			if s.pqcRules.IsPQCAlgorithm(cs.Name) {
				supported = append(supported, cs.Name)
			}
		}
	}

	return supported
}

// hasPFSFromCipherName checks if a cipher suite name indicates Perfect Forward Secrecy
func (s *TLSService) hasPFSFromCipherName(cipherName string) bool {
	cipherUpper := strings.ToUpper(cipherName)
	return strings.Contains(cipherUpper, "ECDHE") || strings.Contains(cipherUpper, "DHE")
}
