package scanread

import (
	"strings"
	"time"

	"cafe-discovery/internal/domain"

	"github.com/google/uuid"
)

// WalletScanRowWire mirrors cafe-persistence internal scan v1 wallet row JSON.
type WalletScanRowWire struct {
	ID              string  `json:"id"`
	UserID          string  `json:"user_id"`
	Address         string  `json:"address"`
	Type            string  `json:"type"`
	Algorithm       string  `json:"algorithm"`
	NISTLevel       int     `json:"nist_level"`
	KeyExposed      bool    `json:"key_exposed"`
	PublicKey       string  `json:"public_key"`
	TransactionHash string  `json:"transaction_hash"`
	ExposedNetwork  string  `json:"exposed_network"`
	IsEOA           bool    `json:"is_eoa"`
	IsERC4337       bool    `json:"is_erc4337"`
	RiskScore       float64 `json:"risk_score"`
	Networks        string  `json:"networks"`
	Connections     string  `json:"connections"`
	Status          string  `json:"status"`
	Error           string  `json:"error"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

// TLSScanRowWire mirrors cafe-persistence internal scan v1 TLS row JSON.
type TLSScanRowWire struct {
	ID              string  `json:"id"`
	UserID          string  `json:"user_id"`
	URL             string  `json:"url"`
	Host            string  `json:"host"`
	Port            int     `json:"port"`
	ProtocolVersion string  `json:"protocol_version"`
	NISTLevel       int     `json:"nist_level"`
	RiskScore       float64 `json:"risk_score"`
	PQCRisk         string  `json:"pqc_risk"`
	KexAlgorithm    string  `json:"kex_algorithm"`
	KexPQCReady     bool    `json:"kex_pqc_ready"`
	PQCMode         string  `json:"pqc_mode"`
	PFS             bool    `json:"pfs"`
	ALPN            string  `json:"alpn"`
	OCSPStapled     bool    `json:"ocsp_stapled"`
	Curve           string  `json:"curve"`
	Certificate     string  `json:"certificate"`
	CipherSuites    string  `json:"cipher_suites"`
	SupportedPQCs   string  `json:"supported_pqcs"`
	Recommendations string  `json:"recommendations"`
	NISTLevels      string  `json:"nist_levels"`
	Default         bool    `json:"default"`
	Status          string  `json:"status"`
	Error           string  `json:"error"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

// ListWalletScansWire is the list envelope from GET /internal/scan/v1/wallets/scans.
type ListWalletScansWire struct {
	Items  []WalletScanRowWire `json:"items"`
	Total  int64               `json:"total"`
	Limit  int                 `json:"limit"`
	Offset int                 `json:"offset"`
}

// ListTLSScansWire is the list envelope from GET /internal/scan/v1/tls/scans.
type ListTLSScansWire struct {
	Items  []TLSScanRowWire `json:"items"`
	Total  int64            `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

// ListTLSDefaultsWire is the list envelope from GET /internal/scan/v1/tls/scans/defaults.
type ListTLSDefaultsWire struct {
	Items  []TLSScanRowWire `json:"items"`
	Total  int64            `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

// WalletRowToEntity maps a persistence wallet row to a Discovery domain entity.
func WalletRowToEntity(w WalletScanRowWire) (*domain.ScanResultEntity, error) {
	id, err := uuid.Parse(strings.TrimSpace(w.ID))
	if err != nil {
		return nil, err
	}
	userID, err := uuid.Parse(strings.TrimSpace(w.UserID))
	if err != nil {
		return nil, err
	}
	createdAt, err := parseTime(w.CreatedAt)
	if err != nil {
		return nil, err
	}
	updatedAt, err := parseTime(w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &domain.ScanResultEntity{
		ID:              id,
		UserID:          userID,
		Address:         w.Address,
		Type:            domain.AccountType(w.Type),
		Algorithm:       domain.Algorithm(w.Algorithm),
		NISTLevel:       domain.NISTLevel(w.NISTLevel),
		KeyExposed:      w.KeyExposed,
		PublicKey:       w.PublicKey,
		TransactionHash: w.TransactionHash,
		ExposedNetwork:  w.ExposedNetwork,
		IsEOA:           w.IsEOA,
		IsERC4337:       w.IsERC4337,
		RiskScore:       w.RiskScore,
		Networks:        w.Networks,
		Connections:     w.Connections,
		Status:          w.Status,
		Error:           w.Error,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
	}, nil
}

// TLSRowToEntity maps a persistence TLS row to a Discovery domain entity.
func TLSRowToEntity(w TLSScanRowWire) (*domain.TLSScanResultEntity, error) {
	id, err := uuid.Parse(strings.TrimSpace(w.ID))
	if err != nil {
		return nil, err
	}
	createdAt, err := parseTime(w.CreatedAt)
	if err != nil {
		return nil, err
	}
	updatedAt, err := parseTime(w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	ent := &domain.TLSScanResultEntity{
		ID:              id,
		URL:             w.URL,
		Host:            w.Host,
		Port:            w.Port,
		ProtocolVersion: w.ProtocolVersion,
		NISTLevel:       domain.NISTLevel(w.NISTLevel),
		RiskScore:       w.RiskScore,
		PQCRisk:         w.PQCRisk,
		KexAlgorithm:    w.KexAlgorithm,
		KexPQCReady:     w.KexPQCReady,
		PQCMode:         w.PQCMode,
		PFS:             w.PFS,
		ALPN:            w.ALPN,
		OCSPStapled:     w.OCSPStapled,
		Curve:           w.Curve,
		Certificate:     w.Certificate,
		CipherSuites:    w.CipherSuites,
		SupportedPQCs:   w.SupportedPQCs,
		Recommendations: w.Recommendations,
		NISTLevels:      w.NISTLevels,
		Default:         w.Default,
		Status:          w.Status,
		Error:           w.Error,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
	}
	if uid := strings.TrimSpace(w.UserID); uid != "" {
		parsed, perr := uuid.Parse(uid)
		if perr != nil {
			return nil, perr
		}
		ent.UserID = &parsed
	}
	return ent, nil
}

func parseTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t.UTC(), nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}
