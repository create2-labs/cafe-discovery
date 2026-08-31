package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ScanResultEntity represents a scan result stored in the database
type ScanResultEntity struct {
	ID              uuid.UUID      `gorm:"type:char(36);primary_key" json:"id"`
	UserID          uuid.UUID      `gorm:"type:char(36);not null;index" json:"user_id"`
	Address         string         `gorm:"not null;index" json:"address"`
	Type            AccountType    `gorm:"type:varchar(20);not null" json:"type"`
	Algorithm       Algorithm      `gorm:"type:varchar(50);not null" json:"algorithm"`
	NISTLevel       NISTLevel      `gorm:"not null" json:"nist_level"`
	KeyExposed      bool           `gorm:"not null" json:"key_exposed"`
	PublicKey       string         `gorm:"type:text" json:"public_key,omitempty"`              // Recovered public key
	TransactionHash string         `gorm:"type:varchar(66)" json:"transaction_hash,omitempty"` // Hash of transaction that exposed the key
	ExposedNetwork  string         `gorm:"type:varchar(50)" json:"exposed_network,omitempty"`  // Network where key was exposed
	IsEOA           bool           `gorm:"not null" json:"is_eoa"`
	IsERC4337       bool           `gorm:"not null" json:"is_erc4337"`
	RiskScore       float64        `gorm:"not null" json:"risk_score"`
	Networks        string         `gorm:"type:text" json:"-"` // JSON array stored as text
	Connections     string         `gorm:"type:text" json:"-"` // JSON array stored as text
	Status          string         `gorm:"type:varchar(20);index" json:"status"` // empty/PENDING until scan.started (RUNNING); then SUCCESS, FAILED, TIMEOUT, UNREACHABLE
	Error           string         `gorm:"type:text" json:"error,omitempty"`                    // Error message when status is FAILED
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for GORM
func (ScanResultEntity) TableName() string {
	return "scan_results"
}

// BeforeCreate hook to generate UUID before creating a scan result
func (s *ScanResultEntity) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// ToScanResult converts the entity to the domain ScanResult DTO
func (s *ScanResultEntity) ToScanResult() *ScanResult {
	networks := parseStringArray(s.Networks)
	connections := parseStringArray(s.Connections)

	return &ScanResult{
		Address:         s.Address,
		Type:            s.Type,
		Algorithm:       s.Algorithm,
		NISTLevel:       s.NISTLevel,
		KeyExposed:      s.KeyExposed,
		PublicKey:       s.PublicKey,
		TransactionHash: s.TransactionHash,
		ExposedNetwork:  s.ExposedNetwork,
		IsEOA:           s.IsEOA,
		IsERC4337:       s.IsERC4337,
		RiskScore:       s.RiskScore,
		Networks:        networks,
		Connections:     connections,
		FirstSeen:       &s.CreatedAt,
		LastSeen:        &s.UpdatedAt,
		ScannedAt:       s.UpdatedAt, // Use UpdatedAt as ScannedAt for existing records
	}
}

func parseStringArray(s string) []string {
	if s == "" || s == "[]" {
		return []string{}
	}
	var arr []string
	if err := json.Unmarshal([]byte(s), &arr); err != nil {
		return []string{}
	}
	return arr
}
