package address

import (
	"fmt"
	"strings"
)

// ValidateAndNormalizeAddress validates and normalizes an Ethereum address.
func ValidateAndNormalizeAddress(address string) (string, error) {
	normalized := normalizeAddress(address)
	if !isValidAddress(normalized) {
		return "", fmt.Errorf("invalid Ethereum address: %s", address)
	}
	return normalized, nil
}

func normalizeAddress(address string) string {
	a := strings.TrimSpace(address)
	if strings.HasPrefix(a, "0X") {
		a = "0x" + a[2:]
	}
	if !strings.HasPrefix(a, "0x") {
		a = "0x" + a
	}
	return strings.ToLower(a)
}

func isValidAddress(address string) bool {
	if len(address) != 42 || !strings.HasPrefix(address, "0x") {
		return false
	}
	for _, c := range address[2:] {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}
