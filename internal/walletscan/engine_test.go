package walletscan

import "testing"

func TestValidateAndNormalizeAddress(t *testing.T) {
	t.Parallel()

	engine := NewWalletScanEngine(nil, nil)

	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{
			name: "checksum input",
			in:   "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
			want: "0x742d35cc6634c0532925a3b844bc454e4438f44e",
		},
		{
			name: "uppercase prefix input",
			in:   "0X742D35CC6634C0532925A3B844BC454E4438F44E",
			want: "0x742d35cc6634c0532925a3b844bc454e4438f44e",
		},
		{
			name: "missing prefix",
			in:   "742d35cc6634c0532925a3b844bc454e4438f44e",
			want: "0x742d35cc6634c0532925a3b844bc454e4438f44e",
		},
		{
			name:    "invalid address",
			in:      "0x1234",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := engine.ValidateAndNormalizeAddress(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ValidateAndNormalizeAddress(%q) expected error", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateAndNormalizeAddress(%q) error = %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("ValidateAndNormalizeAddress(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
