package nats

import "github.com/google/uuid"

// WalletScanMessage represents a wallet scan request message
type WalletScanMessage struct {
	UserID  uuid.UUID `json:"user_id"`
	Address string    `json:"address"`
}

// TLSScanMessage represents a TLS scan request message
type TLSScanMessage struct {
	UserID   uuid.UUID `json:"user_id"`
	Endpoint string    `json:"endpoint"`
}
