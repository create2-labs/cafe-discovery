package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TLSScanResultEntity represents a TLS scan result stored in the database
type TLSScanResultEntity struct {
	ID              uuid.UUID      `gorm:"type:char(36);primary_key" json:"id"`
	UserID          uuid.UUID      `gorm:"type:char(36);not null;index" json:"user_id"`
	URL             string         `gorm:"type:varchar(500);not null;index" json:"url"`
	Host            string         `gorm:"type:varchar(255);not null" json:"host"`
	Port            int            `gorm:"not null" json:"port"`
	ProtocolVersion string         `gorm:"type:varchar(20);not null" json:"protocol_version"`
	NISTLevel       NISTLevel      `gorm:"not null" json:"nist_level"`
	RiskScore       float64        `gorm:"not null" json:"risk_score"`
	PQCRisk         string         `gorm:"type:varchar(20);not null" json:"pqc_risk"`
	Certificate     string         `gorm:"type:text" json:"-"` // JSON stored as text
	CipherSuites    string         `gorm:"type:text" json:"-"` // JSON array stored as text
	SupportedPQCs   string         `gorm:"type:text" json:"-"` // JSON array stored as text
	Recommendations string         `gorm:"type:text" json:"-"` // JSON array stored as text
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for GORM
func (TLSScanResultEntity) TableName() string {
	return "tls_scan_results"
}

// BeforeCreate hook to generate UUID before creating a TLS scan result
func (t *TLSScanResultEntity) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

// ToTLSScanResult converts the entity to the domain TLSScanResult DTO
func (t *TLSScanResultEntity) ToTLSScanResult() *TLSScanResult {
	var certInfo CertificateInfo
	var cipherSuites []CipherSuiteInfo
	var supportedPQCs []string
	var recommendations []string

	// Parse JSON fields
	if t.Certificate != "" {
		_ = json.Unmarshal([]byte(t.Certificate), &certInfo)
	}
	if t.CipherSuites != "" {
		_ = json.Unmarshal([]byte(t.CipherSuites), &cipherSuites)
	}
	if t.SupportedPQCs != "" {
		_ = json.Unmarshal([]byte(t.SupportedPQCs), &supportedPQCs)
	}
	if t.Recommendations != "" {
		_ = json.Unmarshal([]byte(t.Recommendations), &recommendations)
	}

	return &TLSScanResult{
		URL:             t.URL,
		Host:            t.Host,
		Port:            t.Port,
		Certificate:     certInfo,
		CipherSuites:    cipherSuites,
		ProtocolVersion: t.ProtocolVersion,
		NISTLevel:       t.NISTLevel,
		RiskScore:       t.RiskScore,
		PQCRisk:         t.PQCRisk,
		SupportedPQCs:   supportedPQCs,
		Recommendations: recommendations,
		ScannedAt:       t.CreatedAt,
	}
}

// FromTLSScanResult creates an entity from a domain TLSScanResult DTO
func FromTLSScanResult(userID uuid.UUID, result *TLSScanResult) *TLSScanResultEntity {
	certJSON, _ := json.Marshal(result.Certificate)
	cipherSuitesJSON, _ := json.Marshal(result.CipherSuites)
	supportedPQCsJSON, _ := json.Marshal(result.SupportedPQCs)
	recommendationsJSON, _ := json.Marshal(result.Recommendations)

	return &TLSScanResultEntity{
		UserID:          userID,
		URL:             result.URL,
		Host:            result.Host,
		Port:            result.Port,
		ProtocolVersion: result.ProtocolVersion,
		NISTLevel:       result.NISTLevel,
		RiskScore:       result.RiskScore,
		PQCRisk:         result.PQCRisk,
		Certificate:     string(certJSON),
		CipherSuites:    string(cipherSuitesJSON),
		SupportedPQCs:   string(supportedPQCsJSON),
		Recommendations: string(recommendationsJSON),
	}
}

