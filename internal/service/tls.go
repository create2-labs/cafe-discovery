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
	"cafe-discovery/internal/metrics"
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
func (s *TLSService) ScanTLS(ctx context.Context, userID *uuid.UUID, targetURL string, isDefault bool) (result *domain.TLSScanResult, err error) {
	// Record metrics for TLS scan
	startTime := time.Now()
	m := metrics.Get()
	defer func() {
		duration := time.Since(startTime)
		// Record success if no error occurred, failure otherwise
		success := err == nil
		m.RecordTLSScan(duration, success)
	}()
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

	protocolVersionStr := tlspkg.GetProtocolVersion(info.ProtocolVersion)

	// Security model: TLS < 1.3 is fundamentally quantum-unsafe
	// Classify immediately without detailed PQC analysis
	if info.ProtocolVersion < 0x0304 { // TLS 1.3 = 0x0304
		// TLS < 1.3: Immediate quantum-unsafe classification
		// Still extract basic info for completeness, but mark as quantum-unsafe
		certLevel, isPQCCert := s.pqcRules.ClassifyCertificate(info.Certificate)
		certInfo := s.extractCertificateInfo(info.Certificate, certLevel, isPQCCert)
		cipherSuites := s.extractCipherSuites(info)

		port, _ := strconv.Atoi(info.Port)
		return &domain.TLSScanResult{
			URL:             targetURL,
			Host:            info.Host,
			Port:            port,
			Certificate:     certInfo,
			CipherSuites:    cipherSuites,
			ProtocolVersion: protocolVersionStr,
			NISTLevel:       certLevel,  // Use cert level (likely low for TLS < 1.3)
			RiskScore:       1.0,        // Maximum risk for quantum-unsafe protocol
			PQCRisk:         "critical", // Quantum-unsafe
			SupportedPQCs:   []string{},
			Recommendations: []string{
				"CRITICAL: TLS protocol version is below 1.3. TLS versions prior to 1.3 are fundamentally unsafe against quantum computing threats. Upgrade to TLS 1.3 immediately to enable quantum-resistant cryptography.",
			},
			ScannedAt:   time.Now(),
			KexPQCReady: false,
			PQCMode:     "classical",
			PFS:         false, // TLS < 1.3 may not have PFS
		}, nil
	}

	// TLS 1.3: Proceed with detailed PQC analysis
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

	// Calculate risk score (will be updated after PQC info is integrated)
	riskScore := s.calculateTLSRiskScore(certLevel, cipherSuites, protocolVersionStr, false, false, false, false, nil)

	// Determine PQC risk level (will be updated after PQC detection)
	pqcRisk := "critical" // Default for TLS 1.3 without PQC

	// Generate recommendations (will be updated after PQC info is integrated)
	recommendations := s.generateRecommendations(certInfo, cipherSuites, overallLevel, protocolVersionStr, false, false, false, "classical")

	// Check for supported PQC algorithms
	supportedPQCs := s.detectSupportedPQC(certInfo, cipherSuites)

	port, _ := strconv.Atoi(info.Port)

	result = &domain.TLSScanResult{
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
		// Prefer kex_alg over group (kex_alg may contain full hybrid name)
		result.KexAlgorithm = info.PQCInfo.KexAlg
		if result.KexAlgorithm == "" {
			result.KexAlgorithm = info.PQCInfo.Group
		}

		// Enhanced PQC detection: check multiple indicators
		// Priority: 1) kex_pqc_ready flag, 2) pqc flag, 3) pqc_mode, 4) algorithm name analysis
		result.KexPQCReady = info.PQCInfo.KexPQCReady || info.PQCInfo.PQC

		// Set PQC mode - prefer from scan, but infer if needed
		result.PQCMode = info.PQCInfo.PQCMode

		// If mode is not set but we have an algorithm name, analyze it
		if result.PQCMode == "" && result.KexAlgorithm != "" {
			algUpper := strings.ToUpper(result.KexAlgorithm)
			hasClassical := strings.Contains(algUpper, "X25519") ||
				strings.Contains(algUpper, "P256") ||
				strings.Contains(algUpper, "P384") ||
				strings.Contains(algUpper, "SECP")
			hasPQC := strings.Contains(algUpper, "MLKEM") ||
				strings.Contains(algUpper, "KYBER") ||
				strings.Contains(algUpper, "FRODO") ||
				strings.Contains(algUpper, "BIKE")
			if hasClassical && hasPQC {
				result.PQCMode = "hybrid"
				result.KexPQCReady = true
			} else if hasPQC {
				result.PQCMode = "pure"
				result.KexPQCReady = true
			} else {
				result.PQCMode = "classical"
			}
		}

		// If KexPQCReady is still false but mode indicates PQC, set it to true
		if !result.KexPQCReady && (result.PQCMode == "hybrid" || result.PQCMode == "pure") {
			result.KexPQCReady = true
		}

		// Final check: if algorithm name contains PQC indicators, ensure flags are set
		if result.KexAlgorithm != "" {
			algUpper := strings.ToUpper(result.KexAlgorithm)
			if strings.Contains(algUpper, "MLKEM") || strings.Contains(algUpper, "KYBER") ||
				strings.Contains(algUpper, "FRODO") || strings.Contains(algUpper, "BIKE") {
				result.KexPQCReady = true
				// If mode is still classical but we have PQC in name, it's likely hybrid
				if result.PQCMode == "classical" || result.PQCMode == "" {
					if strings.Contains(algUpper, "X25519") || strings.Contains(algUpper, "P256") ||
						strings.Contains(algUpper, "P384") || strings.Contains(algUpper, "SECP") {
						result.PQCMode = "hybrid"
					} else {
						result.PQCMode = "pure"
					}
				}
			}
		}

		result.NISTLevels = info.PQCInfo.NISTLevels
		result.Curve = info.PQCInfo.CertPubkeyECGroup

		// TLS 1.3 always has PFS by design
		if strings.Contains(strings.ToUpper(protocolVersionStr), "1.3") {
			result.PFS = true
		} else if info.PQCInfo.CipherSuite != "" {
			// For TLS < 1.3, check cipher suite
			result.PFS = s.hasPFSFromCipherName(info.PQCInfo.CipherSuite)
		}
	}

	// Set ALPN and OCSP from Go TLS scan (available in TLSInfo)
	result.ALPN = info.ALPN
	result.OCSPStapled = info.OCSPStapled

	// Set PFS if not already set
	// TLS 1.3 always has PFS by design
	if !result.PFS {
		if strings.Contains(strings.ToUpper(protocolVersionStr), "1.3") {
			result.PFS = true
		} else if len(cipherSuites) > 0 {
			cipherName := tlspkg.GetCipherSuiteName(cipherSuites[0].ID)
			result.PFS = s.hasPFSFromCipherName(cipherName)
		}
	}

	// Update risk score with PFS and OCSP information if not already updated by PQC scan
	if info.PQCInfo == nil || info.PQCInfo.NISTLevels == nil {
		result.RiskScore = s.calculateTLSRiskScore(
			certLevel,
			cipherSuites,
			protocolVersionStr,
			result.PFS,
			result.OCSPStapled,
			result.KexPQCReady,
			result.PQCMode == "hybrid" || result.PQCMode == "pure",
			nil,
		)
	}

	// Update overall NIST level using all available information
	// Take the minimum (worst) level from certificate, cipher suites, and detailed levels
	if info.PQCInfo != nil && info.PQCInfo.NISTLevels != nil {
		// Check all detailed NIST levels and take the minimum
		for _, level := range info.PQCInfo.NISTLevels {
			if domain.NISTLevel(level) < overallLevel {
				overallLevel = domain.NISTLevel(level)
			}
		}
		result.NISTLevel = overallLevel

		// Recalculate risk score with detailed NIST levels and updated PQC info
		result.RiskScore = s.calculateTLSRiskScore(
			certLevel,
			cipherSuites,
			protocolVersionStr,
			result.PFS,
			result.OCSPStapled,
			result.KexPQCReady,
			result.PQCMode == "hybrid" || result.PQCMode == "pure",
			info.PQCInfo.NISTLevels,
		)

		// Regenerate recommendations with updated information
		result.Recommendations = s.generateRecommendations(
			certInfo,
			cipherSuites,
			overallLevel,
			protocolVersionStr,
			result.PFS,
			result.OCSPStapled,
			result.KexPQCReady,
			result.PQCMode, // Pass the mode string directly
		)
	}

	// Update PQC risk based on real PQC detection and protocol version
	// TLS < 1.3 is quantum-unsafe (should be filtered earlier, but handle gracefully)
	if !strings.Contains(strings.ToUpper(protocolVersionStr), "1.3") {
		result.PQCRisk = "critical"
	} else {
		// TLS 1.3: Check for PQC KEM using multiple indicators
		hasPQCKEM := false

		// Check 1: Explicit flags from scan
		if result.KexPQCReady || result.PQCMode == "hybrid" || result.PQCMode == "pure" {
			hasPQCKEM = true
		}

		// Check 2: Algorithm name contains PQC indicators (even if flags weren't set)
		if !hasPQCKEM && result.KexAlgorithm != "" {
			algUpper := strings.ToUpper(result.KexAlgorithm)
			if strings.Contains(algUpper, "MLKEM") || strings.Contains(algUpper, "KYBER") ||
				strings.Contains(algUpper, "FRODO") || strings.Contains(algUpper, "BIKE") {
				hasPQCKEM = true
				// Update flags to reflect reality
				result.KexPQCReady = true
				if result.PQCMode == "" || result.PQCMode == "classical" {
					if strings.Contains(algUpper, "X25519") || strings.Contains(algUpper, "P256") ||
						strings.Contains(algUpper, "P384") || strings.Contains(algUpper, "SECP") {
						result.PQCMode = "hybrid"
					} else {
						result.PQCMode = "pure"
					}
				}
			}
		}

		// Check 3: PQCInfo flags (fallback)
		if !hasPQCKEM && info.PQCInfo != nil {
			if info.PQCInfo.PQC || info.PQCInfo.KexPQCReady {
				hasPQCKEM = true
				result.KexPQCReady = true
			}
		}

		if hasPQCKEM {
			// TLS 1.3 with hybrid or pure PQC KEM → safe (HN-DL mitigated)
			result.PQCRisk = "safe"
			// Add PQC algorithms to supported list
			if result.KexAlgorithm != "" {
				supportedPQCs = append(supportedPQCs, result.KexAlgorithm)
			}
		} else {
			// TLS 1.3 without PQC KEM → critical (HN-DL vulnerability)
			result.PQCRisk = "critical"
		}
	}
	result.SupportedPQCs = supportedPQCs

	// Recalculate risk score with complete information (PQC, PFS, OCSP, etc.)
	result.RiskScore = s.calculateTLSRiskScore(
		result.NISTLevel,
		cipherSuites,
		protocolVersionStr,
		result.PFS,
		result.OCSPStapled,
		result.KexPQCReady,
		result.PQCMode == "hybrid" || result.PQCMode == "pure",
		result.NISTLevels,
	)

	// Regenerate recommendations with complete information
	result.Recommendations = s.generateRecommendations(
		certInfo,
		cipherSuites,
		result.NISTLevel,
		protocolVersionStr,
		result.PFS,
		result.OCSPStapled,
		result.KexPQCReady,
		result.PQCMode, // Pass the mode string directly
	)

	// Save TLS scan result to database for authenticated users or default endpoints
	// Anonymous users (uuid.Nil) can scan but results are not saved to DB (they go to Redis)
	// Default endpoints (isDefault=true, userID=nil) should be saved to DB
	if s.tlsScanResultRepo != nil && (isDefault || (userID != nil && *userID != uuid.Nil)) {
		tlsScanResultEntity := domain.FromTLSScanResult(userID, result, isDefault)
		if err := s.tlsScanResultRepo.Create(tlsScanResultEntity); err != nil {
			// Log error but don't fail the request - scan was successful
			// In production, you might want to use a logger here
			_ = err
		}
	}

	return result, nil
}

// GetTLSScanByURL retrieves a TLS scan result by URL for a specific user
func (s *TLSService) GetTLSScanByURL(ctx context.Context, userID uuid.UUID, url string) (*domain.TLSScanResult, error) {
	// Try to find scan result for this user first
	entity, err := s.tlsScanResultRepo.FindByUserIDAndURL(userID, url)
	if err == nil && entity != nil {
		return entity.ToTLSScanResult(), nil
	}

	// If not found for user, try to find default endpoint
	entity, err = s.tlsScanResultRepo.FindDefaultByURL(url)
	if err != nil {
		return nil, fmt.Errorf("TLS scan result not found for URL: %w", err)
	}

	return entity.ToTLSScanResult(), nil
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

// calculateTLSRiskScore calculates a comprehensive risk score for TLS configuration
// The score ranges from 0.0 (lowest risk) to 1.0 (highest risk)
//
// Factors considered:
//   - NIST security levels (certificate, cipher suites, and detailed component levels)
//   - TLS protocol version (TLS 1.3 is preferred)
//   - Perfect Forward Secrecy (PFS) support
//   - OCSP stapling support
//   - Post-Quantum Cryptography (PQC) readiness
//   - PQC mode (hybrid or pure PQC)
//
// The calculation uses weighted components:
//   - Base risk (40%): Based on worst NIST level across all components
//   - Cipher suite risk (25%): Based on weakest cipher suite
//   - Protocol risk (15%): TLS 1.2 or older increases risk
//   - Security features (10%): PFS and OCSP stapling reduce risk
//   - PQC readiness (10%): PQC support significantly reduces risk
func (s *TLSService) calculateTLSRiskScore(
	certLevel domain.NISTLevel,
	cipherSuites []domain.CipherSuiteInfo,
	protocolVersion string,
	hasPFS bool,
	hasOCSPStapling bool,
	kexPQCReady bool,
	isPQCMode bool,
	detailedNISTLevels map[string]int,
) float64 {
	// 1. Base risk from NIST levels (40% weight)
	// Use weighted average of all components to better reflect overall security
	// Critical components (certificate/signature) have more weight
	worstNISTLevel := certLevel
	avgNISTLevel := float64(certLevel)
	componentCount := 1.0

	if len(detailedNISTLevels) > 0 {
		// Find the worst level and calculate weighted average
		// Certificate/signature are critical (weight 2x), others are standard (weight 1x)
		for key, level := range detailedNISTLevels {
			nistLevel := domain.NISTLevel(level)
			if nistLevel < worstNISTLevel {
				worstNISTLevel = nistLevel
			}

			// Weight critical components more heavily
			weight := 1.0
			if key == "sig" || key == "certificate" {
				weight = 2.0 // Certificate and signature are critical
			}

			avgNISTLevel += float64(nistLevel) * weight
			componentCount += weight
		}

		// Calculate weighted average
		avgNISTLevel = avgNISTLevel / componentCount
	} else {
		// Fallback: check cipher suites if no detailed levels
		for _, cs := range cipherSuites {
			if cs.NISTLevel < worstNISTLevel {
				worstNISTLevel = cs.NISTLevel
			}
			avgNISTLevel += float64(cs.NISTLevel)
			componentCount += 1.0
		}
		avgNISTLevel = avgNISTLevel / componentCount
	}

	// Use weighted average for risk calculation, but cap at worst level
	// This reflects that one weak component doesn't make everything weak,
	// but the worst component still matters significantly
	effectiveLevel := avgNISTLevel
	if float64(worstNISTLevel) < effectiveLevel {
		// If worst level is significantly lower, blend it in (30% worst, 70% average)
		effectiveLevel = 0.3*float64(worstNISTLevel) + 0.7*effectiveLevel
	}

	// NIST level to risk: Level 1 = 1.0, Level 5 = 0.0
	// Linear mapping: risk = 1.0 - ((level - 1) / 4)
	baseRisk := 1.0 - ((effectiveLevel - 1.0) / 4.0)
	if baseRisk < 0.0 {
		baseRisk = 0.0
	}
	if baseRisk > 1.0 {
		baseRisk = 1.0
	}

	// 2. Cipher suite risk (25% weight)
	cipherRisk := 0.0
	if len(cipherSuites) == 0 {
		cipherRisk = 1.0 // High risk if no cipher suites
	} else {
		// Find worst cipher suite level
		worstCipherLevel := worstNISTLevel
		for _, cs := range cipherSuites {
			if cs.NISTLevel < worstCipherLevel {
				worstCipherLevel = cs.NISTLevel
			}
		}
		cipherRisk = 1.0 - ((float64(worstCipherLevel) - 1.0) / 4.0)
		if cipherRisk < 0.0 {
			cipherRisk = 0.0
		}
		if cipherRisk > 1.0 {
			cipherRisk = 1.0
		}
	}

	// 3. Protocol version risk (15% weight)
	// TLS 1.3 = 0.0 risk, TLS 1.2 = 0.3 risk, TLS 1.1 or older = 0.8 risk
	protocolRisk := 0.0
	protocolUpper := strings.ToUpper(protocolVersion)
	if strings.Contains(protocolUpper, "1.3") {
		protocolRisk = 0.0
	} else if strings.Contains(protocolUpper, "1.2") {
		protocolRisk = 0.3
	} else if strings.Contains(protocolUpper, "1.1") || strings.Contains(protocolUpper, "1.0") {
		protocolRisk = 0.8
	} else {
		protocolRisk = 0.5 // Unknown protocol version
	}

	// 4. Security features risk reduction (10% weight)
	// PFS and OCSP stapling reduce risk
	securityFeaturesRisk := 0.5 // Default: moderate risk
	if hasPFS && hasOCSPStapling {
		securityFeaturesRisk = 0.0 // Both features present: no additional risk
	} else if hasPFS {
		securityFeaturesRisk = 0.2 // PFS only: low additional risk
	} else if hasOCSPStapling {
		securityFeaturesRisk = 0.3 // OCSP only: moderate additional risk
	}

	// 5. PQC readiness risk reduction (10% weight)
	// PQC support significantly reduces quantum risk
	// For TLS 1.3, this is critical for quantum attack surface assessment
	var pqcRisk float64
	if isPQCMode {
		// Pure or hybrid PQC mode: minimal quantum risk
		// Hybrid provides protection against harvest-now-decrypt-later attacks
		pqcRisk = 0.0
	} else if kexPQCReady {
		// PQC KEX ready but not in PQC mode: low quantum risk
		// This shouldn't happen in practice (PQC KEX implies PQC mode)
		// but handle gracefully
		pqcRisk = 0.1
	} else {
		// No PQC KEM: high quantum risk for TLS 1.3
		// This is the "harvest now, decrypt later" vulnerability
		// Even with high NIST level certs, lack of PQC KEM is critical
		pqcRisk = 0.8
	}

	// Weighted combination
	riskScore := (baseRisk * 0.40) +
		(cipherRisk * 0.25) +
		(protocolRisk * 0.15) +
		(securityFeaturesRisk * 0.10) +
		(pqcRisk * 0.10)

	// Clamp between 0.0 and 1.0
	if riskScore > 1.0 {
		riskScore = 1.0
	}
	if riskScore < 0.0 {
		riskScore = 0.0
	}

	return riskScore
}

// generateRecommendations generates security findings based on scan results
// These are observations about security issues, not remediation recommendations
// Takes into account NIST levels, protocol version, PFS, OCSP, and PQC readiness
func (s *TLSService) generateRecommendations(
	cert domain.CertificateInfo,
	suites []domain.CipherSuiteInfo,
	level domain.NISTLevel,
	protocolVersion string,
	hasPFS bool,
	hasOCSPStapling bool,
	kexPQCReady bool,
	isPQCMode string, // Changed from bool to string: "classical", "hybrid", or "pure"
) []string {
	var recommendations []string
	protocolUpper := strings.ToUpper(protocolVersion)

	recommendations = append(recommendations, s.generateNISTLevelRecommendations(level)...)
	recommendations = append(recommendations, s.generateCertPQCRecommendations(cert)...)
	recommendations = append(recommendations, s.generateProtocolVersionRecommendations(protocolUpper)...)
	recommendations = append(recommendations, s.generatePFSRecommendations(hasPFS)...)
	recommendations = append(recommendations, s.generateOCSPRecommendations(hasOCSPStapling)...)
	recommendations = append(recommendations, s.generateCipherSuiteRecommendations(suites, level)...)
	recommendations = append(recommendations, s.generatePQCRecommendations(protocolUpper, isPQCMode, kexPQCReady)...)
	recommendations = append(recommendations, s.generatePositiveFeedback(level, protocolUpper, isPQCMode, hasPFS, hasOCSPStapling, recommendations)...)

	return recommendations
}

// generateNISTLevelRecommendations generates recommendations based on NIST security level
func (s *TLSService) generateNISTLevelRecommendations(level domain.NISTLevel) []string {
	var recommendations []string
	if level <= domain.NISTLevel1 {
		recommendations = append(recommendations, "CRITICAL: Certificate uses quantum-vulnerable algorithms (NIST Level 1).")
	} else if level <= domain.NISTLevel2 {
		recommendations = append(recommendations, "WARNING: Certificate may be vulnerable to quantum attacks (NIST Level 2). This endpoint has limited protection against quantum computing threats.")
	} else if level == domain.NISTLevel3 {
		recommendations = append(recommendations, "INFO: Certificate provides moderate quantum resistance (NIST Level 3). Higher NIST levels (4 or 5) would provide better protection against quantum attacks.")
	}
	return recommendations
}

// generateCertPQCRecommendations generates recommendations about certificate PQC readiness
func (s *TLSService) generateCertPQCRecommendations(cert domain.CertificateInfo) []string {
	var recommendations []string
	if !cert.IsPQCReady {
		recommendations = append(recommendations, "Certificate does not use post-quantum signature algorithms.")
	}
	return recommendations
}

// generateProtocolVersionRecommendations generates recommendations about TLS protocol version
func (s *TLSService) generateProtocolVersionRecommendations(protocolUpper string) []string {
	var recommendations []string
	if !strings.Contains(protocolUpper, "1.3") {
		if strings.Contains(protocolUpper, "1.2") {
			recommendations = append(recommendations, "TLS protocol version is 1.2 or older. TLS 1.3 provides improved security, better performance, and mandatory Perfect Forward Secrecy.")
		} else {
			recommendations = append(recommendations, "CRITICAL: TLS protocol version is outdated and insecure. This endpoint uses an obsolete TLS version that lacks modern security features.")
		}
	}
	return recommendations
}

// generatePFSRecommendations generates recommendations about Perfect Forward Secrecy
func (s *TLSService) generatePFSRecommendations(hasPFS bool) []string {
	var recommendations []string
	if !hasPFS {
		recommendations = append(recommendations, "Perfect Forward Secrecy (PFS) is not enabled. Past communications could be decrypted if the private key is compromised in the future.")
	}
	return recommendations
}

// generateOCSPRecommendations generates recommendations about OCSP stapling
func (s *TLSService) generateOCSPRecommendations(hasOCSPStapling bool) []string {
	var recommendations []string
	if !hasOCSPStapling {
		recommendations = append(recommendations, "OCSP stapling is not enabled. This may result in slower certificate validation and increased latency.")
	}
	return recommendations
}

// generateCipherSuiteRecommendations generates recommendations about cipher suites
func (s *TLSService) generateCipherSuiteRecommendations(suites []domain.CipherSuiteInfo, level domain.NISTLevel) []string {
	var recommendations []string
	hasWeakCipher := false
	worstCipherLevel := level
	for _, cs := range suites {
		if cs.NISTLevel < worstCipherLevel {
			worstCipherLevel = cs.NISTLevel
		}
		if cs.NISTLevel <= domain.NISTLevel1 && !cs.IsPQCReady {
			hasWeakCipher = true
		}
	}

	if hasWeakCipher {
		recommendations = append(recommendations, "Weak cipher suites (NIST Level 1) are enabled. These cipher suites are vulnerable to quantum attacks.")
	} else if worstCipherLevel <= domain.NISTLevel2 {
		recommendations = append(recommendations, "Cipher suites use NIST Level 2 or lower. Higher NIST levels (3 or higher) would provide better quantum resistance.")
	}
	return recommendations
}

// generatePQCRecommendations generates recommendations about post-quantum cryptography
func (s *TLSService) generatePQCRecommendations(protocolUpper string, isPQCMode string, kexPQCReady bool) []string {
	var recommendations []string
	if strings.Contains(protocolUpper, "1.3") {
		// TLS 1.3 specific PQC findings
		// Only recommend enabling PQC if it's truly not present
		hasPQCKEM := (isPQCMode == "hybrid" || isPQCMode == "pure") || kexPQCReady

		if !hasPQCKEM {
			recommendations = append(recommendations, "CRITICAL: Post-quantum cryptography (PQC) is not used for key exchange in TLS 1.3. This endpoint is vulnerable to 'harvest now, decrypt later' (HN-DL) attacks. Even if traffic is encrypted today, it can be decrypted in the future when quantum computers become available. Enable hybrid PQC KEMs (e.g., X25519MLKEM768) to protect against this threat.")
		} else {
			// PQC KEM is present - provide positive feedback
			if isPQCMode == "hybrid" {
				recommendations = append(recommendations, "✅ Hybrid post-quantum key exchange is enabled. This provides protection against quantum attacks (harvest-now-decrypt-later mitigated) while maintaining compatibility with classical systems.")
			} else if isPQCMode == "pure" {
				recommendations = append(recommendations, "✅ Pure post-quantum key exchange is enabled. This provides maximum quantum protection against harvest-now-decrypt-later attacks.")
			} else if kexPQCReady {
				// PQC detected but mode unclear - still positive
				recommendations = append(recommendations, "✅ Post-quantum key exchange is enabled. This provides protection against harvest-now-decrypt-later attacks.")
			}
		}
	} else {
		// TLS < 1.3: PQC is not applicable (should be filtered earlier)
		recommendations = append(recommendations, "TLS protocol version is below 1.3. PQC key exchange is only available in TLS 1.3. Upgrade to TLS 1.3 to enable PQC protection.")
	}
	return recommendations
}

// generatePositiveFeedback generates positive feedback for well-configured endpoints
func (s *TLSService) generatePositiveFeedback(level domain.NISTLevel, protocolUpper string, isPQCMode string, hasPFS bool, hasOCSPStapling bool, existingRecommendations []string) []string {
	var recommendations []string
	isWellConfigured := level >= domain.NISTLevel4 &&
		(isPQCMode == "hybrid" || isPQCMode == "pure") &&
		hasPFS &&
		hasOCSPStapling &&
		strings.Contains(protocolUpper, "1.3")

	if len(existingRecommendations) == 0 || isWellConfigured {
		recommendations = append(recommendations, "TLS configuration appears quantum-resistant and well-configured. Continue monitoring PQC standards updates and maintain current security practices.")
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
