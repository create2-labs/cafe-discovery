package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CafeWallet represents a user's saved wallet
type CafeWallet struct {
	PubKeyHash string    `gorm:"type:varchar(255);primaryKey" json:"pub_key_hash"`
	UserID     uuid.UUID `gorm:"type:char(36);not null;index" json:"user_id"`
	UserPubKey string    `gorm:"type:varchar(255);not null" json:"user_pub_key"`
	Label      string    `gorm:"type:varchar(255)" json:"label"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for GORM
func (CafeWallet) TableName() string {
	return "cafe_wallets"
}

