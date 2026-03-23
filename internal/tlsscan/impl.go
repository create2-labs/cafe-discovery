package tlsscan

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"strconv"
	"strings"
	"time"

	"cafe-discovery/internal/domain"
	tlspkg "cafe-discovery/pkg/tls"
)

// buildResultForTLSBelow13 returns a quantum-unsafe result for TLS < 1.3.
func (e *TLSScanEngine) buildResultForTLSBelow13(info *tlspkg.TLSInfo, targetURL, protocolVersionStr string) *domain.TLSScanResult {
	certLevel, isPQCCert := e.pqcRules.ClassifyCertificate(info.Certificate)
	certInfo := e.extractCertificateInfo(info.Certificate, certLevel, isPQCCert)
	cipherSuites := e.extractCipherSuites(info)
	port, _ := strconv.Atoi(info.Port)
	return &domain.TLSScanResult{
		URL:             targetURL,
		Host:            info.Host,
		Port:            port,
		Certificate:     certInfo,
		CipherSuites:    cipherSuites,
		ProtocolVersion: protocolVersionStr,
		NISTLevel:       certLevel,
		RiskScore:       1.0,
		PQCRisk:         "critical",
		SupportedPQCs:   []string{},
		Recommendations: []string{
			"CRITICAL: TLS protocol version is below 1.3. TLS versions prior to 1.3 are fundamentally unsafe against quantum computing threats. Upgrade to TLS 1.3 immediately to enable quantum-resistant cryptography.",
		},
		ScannedAt:   time.Now(),
		KexPQCReady: false,
		PQCMode:     "classical",
		PFS:         false,
	}
}

// tls13BuildState holds working state while building a TLS 1.3 scan result.
type tls13BuildState struct {
	result        *domain.TLSScanResult
	certLevel     domain.NISTLevel
	certInfo      domain.CertificateInfo
	cipherSuites  []domain.CipherSuiteInfo
	overallLevel  domain.NISTLevel
	supportedPQCs []string
}

func (e *TLSScanEngine) buildTLS13BaseState(info *tlspkg.TLSInfo, targetURL, protocolVersionStr string) *tls13BuildState {
	certLevel, isPQCCert := e.pqcRules.ClassifyCertificate(info.Certificate)
	certInfo := e.extractCertificateInfo(info.Certificate, certLevel, isPQCCert)
	cipherSuites := e.extractCipherSuites(info)

	overallLevel := certLevel
	for _, cs := range cipherSuites {
		if cs.NISTLevel < overallLevel {
			overallLevel = cs.NISTLevel
		}
	}

	riskScore := e.calculateTLSRiskScore(tlsRiskParams{
		CertLevel: certLevel, CipherSuites: cipherSuites, ProtocolVersion: protocolVersionStr,
		HasPFS: false, HasOCSPStapling: false, KexPQCReady: false, IsPQCMode: false, DetailedNISTLevels: nil,
	})
	recommendations := e.generateRecommendations(tlsRecommendationParams{
		Cert: certInfo, Suites: cipherSuites, Level: overallLevel, ProtocolVersion: protocolVersionStr,
		HasPFS: false, HasOCSPStapling: false, KexPQCReady: false, IsPQCMode: "classical",
	})
	supportedPQCs := e.detectSupportedPQC(certInfo, cipherSuites)

	port, _ := strconv.Atoi(info.Port)
	result := &domain.TLSScanResult{
		URL:             targetURL,
		Host:            info.Host,
		Port:            port,
		Certificate:     certInfo,
		CipherSuites:    cipherSuites,
		ProtocolVersion: protocolVersionStr,
		NISTLevel:       overallLevel,
		RiskScore:       riskScore,
		PQCRisk:         "critical",
		SupportedPQCs:   supportedPQCs,
		Recommendations: recommendations,
		ScannedAt:       time.Now(),
	}

	return &tls13BuildState{
		result:        result,
		certLevel:     certLevel,
		certInfo:      certInfo,
		cipherSuites:  cipherSuites,
		overallLevel:  overallLevel,
		supportedPQCs: supportedPQCs,
	}
}

func (e *TLSScanEngine) applyPQCInfoToResult(state *tls13BuildState, info *tlspkg.TLSInfo, protocolVersionStr string) {
	if info.PQCInfo == nil {
		return
	}
	pqc := info.PQCInfo

	state.result.KexAlgorithm = pqc.KexAlg
	if state.result.KexAlgorithm == "" {
		state.result.KexAlgorithm = pqc.Group
	}
	state.result.KexPQCReady = pqc.KexPQCReady || pqc.PQC
	state.result.PQCMode = pqc.PQCMode

	e.inferPQCModeFromAlgorithm(state.result)
	if !state.result.KexPQCReady && (state.result.PQCMode == "hybrid" || state.result.PQCMode == "pure") {
		state.result.KexPQCReady = true
	}
	e.ensurePQCFlagsFromAlgorithmName(state.result)

	state.result.NISTLevels = pqc.NISTLevels
	state.result.Curve = pqc.CertPubkeyECGroup

	if strings.Contains(strings.ToUpper(protocolVersionStr), "1.3") {
		state.result.PFS = true
	} else if pqc.CipherSuite != "" {
		state.result.PFS = e.hasPFSFromCipherName(pqc.CipherSuite)
	}
}

func (e *TLSScanEngine) inferPQCModeFromAlgorithm(result *domain.TLSScanResult) {
	if result.PQCMode != "" || result.KexAlgorithm == "" {
		return
	}
	algUpper := strings.ToUpper(result.KexAlgorithm)
	hasClassical := strings.Contains(algUpper, "X25519") || strings.Contains(algUpper, "P256") ||
		strings.Contains(algUpper, "P384") || strings.Contains(algUpper, "SECP")
	hasPQC := strings.Contains(algUpper, "MLKEM") || strings.Contains(algUpper, "KYBER") ||
		strings.Contains(algUpper, "FRODO") || strings.Contains(algUpper, "BIKE")
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

func (e *TLSScanEngine) ensurePQCFlagsFromAlgorithmName(result *domain.TLSScanResult) {
	if result.KexAlgorithm == "" {
		return
	}
	algUpper := strings.ToUpper(result.KexAlgorithm)
	if !strings.Contains(algUpper, "MLKEM") && !strings.Contains(algUpper, "KYBER") &&
		!strings.Contains(algUpper, "FRODO") && !strings.Contains(algUpper, "BIKE") {
		return
	}
	result.KexPQCReady = true
	if result.PQCMode == "classical" || result.PQCMode == "" {
		if strings.Contains(algUpper, "X25519") || strings.Contains(algUpper, "P256") ||
			strings.Contains(algUpper, "P384") || strings.Contains(algUpper, "SECP") {
			result.PQCMode = "hybrid"
		} else {
			result.PQCMode = "pure"
		}
	}
}

func (e *TLSScanEngine) setPFSAndALPNOnResult(state *tls13BuildState, info *tlspkg.TLSInfo, protocolVersionStr string) {
	state.result.ALPN = info.ALPN
	state.result.OCSPStapled = info.OCSPStapled
	if state.result.PFS {
		return
	}
	if strings.Contains(strings.ToUpper(protocolVersionStr), "1.3") {
		state.result.PFS = true
	} else if len(state.cipherSuites) > 0 {
		cipherName := tlspkg.GetCipherSuiteName(state.cipherSuites[0].ID)
		state.result.PFS = e.hasPFSFromCipherName(cipherName)
	}
}

func (e *TLSScanEngine) updateRiskScoreIfNeeded(state *tls13BuildState, info *tlspkg.TLSInfo, protocolVersionStr string) {
	if info.PQCInfo != nil && info.PQCInfo.NISTLevels != nil {
		return
	}
	state.result.RiskScore = e.calculateTLSRiskScore(tlsRiskParams{
		CertLevel: state.certLevel, CipherSuites: state.cipherSuites, ProtocolVersion: protocolVersionStr,
		HasPFS: state.result.PFS, HasOCSPStapling: state.result.OCSPStapled,
		KexPQCReady: state.result.KexPQCReady, IsPQCMode: state.result.PQCMode == "hybrid" || state.result.PQCMode == "pure",
		DetailedNISTLevels: nil,
	})
}

func (e *TLSScanEngine) updateNISTLevelAndRiskFromPQC(state *tls13BuildState, info *tlspkg.TLSInfo, protocolVersionStr string) {
	if info.PQCInfo == nil || info.PQCInfo.NISTLevels == nil {
		return
	}
	for _, level := range info.PQCInfo.NISTLevels {
		if domain.NISTLevel(level) < state.overallLevel {
			state.overallLevel = domain.NISTLevel(level)
		}
	}
	state.result.NISTLevel = state.overallLevel
	state.result.RiskScore = e.calculateTLSRiskScore(tlsRiskParams{
		CertLevel: state.certLevel, CipherSuites: state.cipherSuites, ProtocolVersion: protocolVersionStr,
		HasPFS: state.result.PFS, HasOCSPStapling: state.result.OCSPStapled,
		KexPQCReady: state.result.KexPQCReady, IsPQCMode: state.result.PQCMode == "hybrid" || state.result.PQCMode == "pure",
		DetailedNISTLevels: info.PQCInfo.NISTLevels,
	})
	state.result.Recommendations = e.generateRecommendations(tlsRecommendationParams{
		Cert: state.certInfo, Suites: state.cipherSuites, Level: state.overallLevel, ProtocolVersion: protocolVersionStr,
		HasPFS: state.result.PFS, HasOCSPStapling: state.result.OCSPStapled,
		KexPQCReady: state.result.KexPQCReady, IsPQCMode: state.result.PQCMode,
	})
}

func (e *TLSScanEngine) updatePQCRiskAndFinalScores(state *tls13BuildState, info *tlspkg.TLSInfo, protocolVersionStr string) {
	e.applyPQCRiskToState(state, info, protocolVersionStr)
	state.result.SupportedPQCs = state.supportedPQCs

	state.result.RiskScore = e.calculateTLSRiskScore(tlsRiskParams{
		CertLevel: state.result.NISTLevel, CipherSuites: state.cipherSuites, ProtocolVersion: protocolVersionStr,
		HasPFS: state.result.PFS, HasOCSPStapling: state.result.OCSPStapled,
		KexPQCReady: state.result.KexPQCReady, IsPQCMode: state.result.PQCMode == "hybrid" || state.result.PQCMode == "pure",
		DetailedNISTLevels: state.result.NISTLevels,
	})
	state.result.Recommendations = e.generateRecommendations(tlsRecommendationParams{
		Cert: state.certInfo, Suites: state.cipherSuites, Level: state.result.NISTLevel, ProtocolVersion: protocolVersionStr,
		HasPFS: state.result.PFS, HasOCSPStapling: state.result.OCSPStapled,
		KexPQCReady: state.result.KexPQCReady, IsPQCMode: state.result.PQCMode,
	})
}

// applyPQCRiskToState sets state.result.PQCRisk and may append to state.supportedPQCs based on PQC detection.
func (e *TLSScanEngine) applyPQCRiskToState(state *tls13BuildState, info *tlspkg.TLSInfo, protocolVersionStr string) {
	protocolUpper := strings.ToUpper(protocolVersionStr)
	if !strings.Contains(protocolUpper, "1.3") {
		state.result.PQCRisk = "critical"
		return
	}
	hasPQCKEM := state.result.KexPQCReady || state.result.PQCMode == "hybrid" || state.result.PQCMode == "pure"
	if !hasPQCKEM && state.result.KexAlgorithm != "" {
		hasPQCKEM = e.applyPQCKEMFromAlgorithmName(state.result)
	}
	if !hasPQCKEM && info.PQCInfo != nil && (info.PQCInfo.PQC || info.PQCInfo.KexPQCReady) {
		hasPQCKEM = true
		state.result.KexPQCReady = true
	}
	if hasPQCKEM {
		state.result.PQCRisk = "safe"
		if state.result.KexAlgorithm != "" {
			state.supportedPQCs = append(state.supportedPQCs, state.result.KexAlgorithm)
		}
	} else {
		state.result.PQCRisk = "critical"
	}
}

// applyPQCKEMFromAlgorithmName updates result flags from algorithm name and returns true if PQC KEM was detected.
func (e *TLSScanEngine) applyPQCKEMFromAlgorithmName(result *domain.TLSScanResult) bool {
	algUpper := strings.ToUpper(result.KexAlgorithm)
	if !strings.Contains(algUpper, "MLKEM") && !strings.Contains(algUpper, "KYBER") &&
		!strings.Contains(algUpper, "FRODO") && !strings.Contains(algUpper, "BIKE") {
		return false
	}
	result.KexPQCReady = true
	if result.PQCMode == "" || result.PQCMode == "classical" {
		if strings.Contains(algUpper, "X25519") || strings.Contains(algUpper, "P256") ||
			strings.Contains(algUpper, "P384") || strings.Contains(algUpper, "SECP") {
			result.PQCMode = "hybrid"
		} else {
			result.PQCMode = "pure"
		}
	}
	return true
}

func (e *TLSScanEngine) extractCertificateInfo(cert *x509.Certificate, level domain.NISTLevel, isPQC bool) domain.CertificateInfo {
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
func (e *TLSScanEngine) extractCipherSuites(info *tlspkg.TLSInfo) []domain.CipherSuiteInfo {
	var suites []domain.CipherSuiteInfo

	for _, cipherID := range info.CipherSuites {
		cipherName := tlspkg.GetCipherSuiteName(cipherID)
		keyEx, enc, mac := tlspkg.ParseCipherSuite(cipherName)

		level, isPQC := e.pqcRules.ClassifyCipherSuite(cipherName)

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

// tlsRiskInputs holds NIST-level aggregates used for risk calculation.
type tlsRiskInputs struct {
	worstNISTLevel domain.NISTLevel
	avgNISTLevel   float64
}

// tlsRiskParams holds all inputs for TLS risk score calculation (keeps param count ≤7).
type tlsRiskParams struct {
	CertLevel          domain.NISTLevel
	CipherSuites       []domain.CipherSuiteInfo
	ProtocolVersion    string
	HasPFS             bool
	HasOCSPStapling    bool
	KexPQCReady        bool
	IsPQCMode          bool
	DetailedNISTLevels map[string]int
}

// calculateTLSRiskScore calculates a comprehensive risk score for TLS configuration
// (0.0 = lowest risk, 1.0 = highest). Weights: base 40%, cipher 25%, protocol 15%, security 10%, PQC 10%.
func (e *TLSScanEngine) calculateTLSRiskScore(p tlsRiskParams) float64 {
	nist := e.computeNISTLevelsForRisk(p.CertLevel, p.CipherSuites, p.DetailedNISTLevels)
	baseRisk := e.baseRiskFromNISTLevels(nist.avgNISTLevel, nist.worstNISTLevel)
	cipherRisk := e.cipherRiskFromSuites(p.CipherSuites, nist.worstNISTLevel)
	protocolRisk := e.protocolRiskFromVersion(p.ProtocolVersion)
	securityRisk := e.securityFeaturesRisk(p.HasPFS, p.HasOCSPStapling)
	pqcRisk := e.pqcRiskScore(p.IsPQCMode, p.KexPQCReady)

	score := (baseRisk * 0.40) + (cipherRisk * 0.25) + (protocolRisk * 0.15) + (securityRisk * 0.10) + (pqcRisk * 0.10)
	return clampRiskScore(score)
}

func (e *TLSScanEngine) computeNISTLevelsForRisk(certLevel domain.NISTLevel, cipherSuites []domain.CipherSuiteInfo, detailedNISTLevels map[string]int) tlsRiskInputs {
	if len(detailedNISTLevels) > 0 {
		return e.nistLevelsFromDetailed(certLevel, detailedNISTLevels)
	}
	return e.nistLevelsFromCipherSuites(certLevel, cipherSuites)
}

func (e *TLSScanEngine) nistLevelsFromDetailed(certLevel domain.NISTLevel, detailedNISTLevels map[string]int) tlsRiskInputs {
	worst := certLevel
	avg := float64(certLevel)
	count := 1.0
	for key, level := range detailedNISTLevels {
		nl := domain.NISTLevel(level)
		if nl < worst {
			worst = nl
		}
		weight := 1.0
		if key == "sig" || key == "certificate" {
			weight = 2.0
		}
		avg += float64(nl) * weight
		count += weight
	}
	return tlsRiskInputs{worstNISTLevel: worst, avgNISTLevel: avg / count}
}

func (e *TLSScanEngine) nistLevelsFromCipherSuites(certLevel domain.NISTLevel, cipherSuites []domain.CipherSuiteInfo) tlsRiskInputs {
	worst := certLevel
	avg := float64(certLevel)
	count := 1.0
	for _, cs := range cipherSuites {
		if cs.NISTLevel < worst {
			worst = cs.NISTLevel
		}
		avg += float64(cs.NISTLevel)
		count++
	}
	return tlsRiskInputs{worstNISTLevel: worst, avgNISTLevel: avg / count}
}

func (e *TLSScanEngine) baseRiskFromNISTLevels(avgNISTLevel float64, worstNISTLevel domain.NISTLevel) float64 {
	effectiveLevel := avgNISTLevel
	if float64(worstNISTLevel) < effectiveLevel {
		effectiveLevel = 0.3*float64(worstNISTLevel) + 0.7*effectiveLevel
	}
	baseRisk := 1.0 - ((effectiveLevel - 1.0) / 4.0)
	return clampRiskScore(baseRisk)
}

func (e *TLSScanEngine) cipherRiskFromSuites(cipherSuites []domain.CipherSuiteInfo, worstNISTLevel domain.NISTLevel) float64 {
	if len(cipherSuites) == 0 {
		return 1.0
	}
	worstCipher := worstNISTLevel
	for _, cs := range cipherSuites {
		if cs.NISTLevel < worstCipher {
			worstCipher = cs.NISTLevel
		}
	}
	return clampRiskScore(1.0 - ((float64(worstCipher) - 1.0) / 4.0))
}

func (e *TLSScanEngine) protocolRiskFromVersion(protocolVersion string) float64 {
	upper := strings.ToUpper(protocolVersion)
	if strings.Contains(upper, "1.3") {
		return 0.0
	}
	if strings.Contains(upper, "1.2") {
		return 0.3
	}
	if strings.Contains(upper, "1.1") || strings.Contains(upper, "1.0") {
		return 0.8
	}
	return 0.5
}

func (e *TLSScanEngine) securityFeaturesRisk(hasPFS, hasOCSPStapling bool) float64 {
	if hasPFS && hasOCSPStapling {
		return 0.0
	}
	if hasPFS {
		return 0.2
	}
	if hasOCSPStapling {
		return 0.3
	}
	return 0.5
}

func (e *TLSScanEngine) pqcRiskScore(isPQCMode, kexPQCReady bool) float64 {
	if isPQCMode {
		return 0.0
	}
	if kexPQCReady {
		return 0.1
	}
	return 0.8
}

func clampRiskScore(score float64) float64 {
	if score < 0.0 {
		return 0.0
	}
	if score > 1.0 {
		return 1.0
	}
	return score
}

// tlsRecommendationParams holds inputs for recommendation generation (keeps param count ≤7).
type tlsRecommendationParams struct {
	Cert            domain.CertificateInfo
	Suites          []domain.CipherSuiteInfo
	Level           domain.NISTLevel
	ProtocolVersion string
	HasPFS          bool
	HasOCSPStapling bool
	KexPQCReady     bool
	IsPQCMode       string // "classical", "hybrid", or "pure"
}

// generateRecommendations generates security findings based on scan results
// (NIST levels, protocol version, PFS, OCSP, PQC readiness).
func (e *TLSScanEngine) generateRecommendations(p tlsRecommendationParams) []string {
	protocolUpper := strings.ToUpper(p.ProtocolVersion)
	var recommendations []string
	recommendations = append(recommendations, e.generateNISTLevelRecommendations(p.Level)...)
	recommendations = append(recommendations, e.generateCertPQCRecommendations(p.Cert)...)
	recommendations = append(recommendations, e.generateProtocolVersionRecommendations(protocolUpper)...)
	recommendations = append(recommendations, e.generatePFSRecommendations(p.HasPFS)...)
	recommendations = append(recommendations, e.generateOCSPRecommendations(p.HasOCSPStapling)...)
	recommendations = append(recommendations, e.generateCipherSuiteRecommendations(p.Suites, p.Level)...)
	recommendations = append(recommendations, e.generatePQCRecommendations(protocolUpper, p.IsPQCMode, p.KexPQCReady)...)
	recommendations = append(recommendations, e.generatePositiveFeedback(p.Level, protocolUpper, p.IsPQCMode, p.HasPFS, p.HasOCSPStapling, recommendations)...)
	return recommendations
}

// generateNISTLevelRecommendations generates recommendations based on NIST security level
func (e *TLSScanEngine) generateNISTLevelRecommendations(level domain.NISTLevel) []string {
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
func (e *TLSScanEngine) generateCertPQCRecommendations(cert domain.CertificateInfo) []string {
	var recommendations []string
	if !cert.IsPQCReady {
		recommendations = append(recommendations, "Certificate does not use post-quantum signature algorithms.")
	}
	return recommendations
}

// generateProtocolVersionRecommendations generates recommendations about TLS protocol version
func (e *TLSScanEngine) generateProtocolVersionRecommendations(protocolUpper string) []string {
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
func (e *TLSScanEngine) generatePFSRecommendations(hasPFS bool) []string {
	var recommendations []string
	if !hasPFS {
		recommendations = append(recommendations, "Perfect Forward Secrecy (PFS) is not enabled. Past communications could be decrypted if the private key is compromised in the future.")
	}
	return recommendations
}

// generateOCSPRecommendations generates recommendations about OCSP stapling
func (e *TLSScanEngine) generateOCSPRecommendations(hasOCSPStapling bool) []string {
	var recommendations []string
	if !hasOCSPStapling {
		recommendations = append(recommendations, "OCSP stapling is not enabled. This may result in slower certificate validation and increased latency.")
	}
	return recommendations
}

// generateCipherSuiteRecommendations generates recommendations about cipher suites
func (e *TLSScanEngine) generateCipherSuiteRecommendations(suites []domain.CipherSuiteInfo, level domain.NISTLevel) []string {
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
func (e *TLSScanEngine) generatePQCRecommendations(protocolUpper string, isPQCMode string, kexPQCReady bool) []string {
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
func (e *TLSScanEngine) generatePositiveFeedback(level domain.NISTLevel, protocolUpper string, isPQCMode string, hasPFS bool, hasOCSPStapling bool, existingRecommendations []string) []string {
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
func (e *TLSScanEngine) detectSupportedPQC(cert domain.CertificateInfo, suites []domain.CipherSuiteInfo) []string {
	var supported []string

	if cert.IsPQCReady {
		if e.pqcRules.IsPQCAlgorithm(cert.PublicKeyAlgorithm) {
			supported = append(supported, cert.PublicKeyAlgorithm)
		}
		if e.pqcRules.IsPQCAlgorithm(cert.SignatureAlgorithm) {
			supported = append(supported, cert.SignatureAlgorithm)
		}
	}

	for _, cs := range suites {
		if cs.IsPQCReady {
			if e.pqcRules.IsPQCAlgorithm(cs.Name) {
				supported = append(supported, cs.Name)
			}
		}
	}

	return supported
}

// hasPFSFromCipherName checks if a cipher suite name indicates Perfect Forward Secrecy
func (e *TLSScanEngine) hasPFSFromCipherName(cipherName string) bool {
	cipherUpper := strings.ToUpper(cipherName)
	return strings.Contains(cipherUpper, "ECDHE") || strings.Contains(cipherUpper, "DHE")
}
