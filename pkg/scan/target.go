package scan

// ScanTarget is a marker interface for typed scan inputs.
// Implementations: *TLSTarget, *WalletTarget.
type ScanTarget interface {
	ScanKind() string
}

// TLSTarget carries TLS scan input (endpoint URL).
type TLSTarget struct {
	Endpoint string
}

// ScanKind implements ScanTarget.
func (*TLSTarget) ScanKind() string { return KindTLS }

// WalletTarget carries wallet scan input (normalized address).
type WalletTarget struct {
	Address string
}

// ScanKind implements ScanTarget.
func (*WalletTarget) ScanKind() string { return KindWallet }
