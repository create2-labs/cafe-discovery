package main

import (
	"encoding/json"
	"fmt"
	"os"

	"cafe/backends/discovery/server"
)

// --- main: enhanced CLI with PQC group testing ---
func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: pq-scan <host|host:port|https://host[:port]> [group]")
		os.Exit(1)
	}
	target := os.Args[1]
	var customGroup string
	if len(os.Args) >= 3 {
		customGroup = os.Args[2]
	}

	// Liste des groupes PQC à tester (ordre de priorité)
	groups := []string{
		"X25519MLKEM768",
		"SecP256r1MLKEM768",
		"SecP384r1MLKEM1024",
		"X25519MLKEM1024",
		"mlkem768",
		"mlkem1024",
		"mlkem512",
	}
	if customGroup != "" {
		groups = append([]string{customGroup}, groups...)
	}

	fmt.Printf("🔍 Testing endpoint: %s\n", target)

	// Essai initial avec la version classique (sans groupe forcé)
	res, err := server.ScanTLS(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Initial handshake failed: %v\n", err)
	}

	// Dump result as formatted JSON
	out, _ := json.MarshalIndent(res, "", "  ")
	fmt.Println("📋 Scan result:")
	fmt.Println(string(out))

	// Si on a déjà une info sur le groupe, inutile d’aller plus loin

	if grp, ok := res["kex_group"].(string); ok && grp != "" && grp != "unknown" {
		fmt.Println("✅ TLS handshake successful with standard configuration")
		out, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(out))
		return
	}

	fmt.Println("ℹ️  Trying PQC/hybrid groups...")

	found := false

	for _, g := range groups {
		fmt.Printf(" → Trying group: %s ... ", g)
		out, err := server.TryGroup(target, g)
		if err == nil && out != nil {
			kex, _ := out["kex_group"].(string)
			if kex != "" && kex != "unknown" {
				fmt.Println("✅ success")
				found = true
				outJSON, _ := json.MarshalIndent(out, "", "  ")
				fmt.Println(string(outJSON))
				break
			} else {
				fmt.Println("✅ handshake succeeded for group:", g)
				out["kex_group"] = g
				out["kex_pqc_ready"] = true
				return
			}
		} else {
			fmt.Printf("❌ %v\n", err)
		}
	}

	if !found {
		fmt.Println("❌ No PQC or hybrid KEX succeeded.")
	}

}
