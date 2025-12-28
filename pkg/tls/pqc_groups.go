package tls

// DefaultPQCGroups is the list of PQC groups to try when scanning TLS endpoints
// if no PQC is detected in the initial scan. These groups are tried in order
// until one succeeds or all fail.
var DefaultPQCGroups = []string{
	"X25519MLKEM768",
	"SecP256r1MLKEM768",
	"SecP384r1MLKEM1024",
	"X25519MLKEM1024",
	"mlkem768",
	"mlkem1024",
	"mlkem512",
}
