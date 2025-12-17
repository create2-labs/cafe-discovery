package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	// Récupérer les chemins des certificats depuis les arguments
	certFile := "/certs/server.crt"
	keyFile := "/certs/server.key"
	port := "8443"

	if len(os.Args) > 1 {
		certFile = os.Args[1]
	}
	if len(os.Args) > 2 {
		keyFile = os.Args[2]
	}
	if len(os.Args) > 3 {
		port = os.Args[3]
	}

	// Si les fichiers sont relatifs, chercher dans /certs
	if !filepath.IsAbs(certFile) {
		certFile = filepath.Join("/certs", certFile)
	}
	if !filepath.IsAbs(keyFile) {
		keyFile = filepath.Join("/certs", keyFile)
	}

	// Vérifier que les fichiers existent
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		log.Fatalf("Certificat introuvable: %s", certFile)
	}
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		log.Fatalf("Clé privée introuvable: %s", keyFile)
	}

	// Handler simple
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		if _, err := fmt.Fprint(w, "ok\n"); err != nil {
			log.Printf("Error writing response: %v", err)
		}
	})

	// Configuration TLS
	server := &http.Server{
		Addr: ":" + port,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	fmt.Printf("🚀 Serveur HTTPS démarré sur le port %s\n", port)
	fmt.Printf("📜 Certificat: %s\n", certFile)
	fmt.Printf("🔑 Clé privée: %s\n", keyFile)
	fmt.Printf("🌐 Test: curl -k https://localhost:%s/\n", port)
	fmt.Println()
	fmt.Println("⚠️  Note: Go ne supporte pas nativement les certificats PQC.")
	fmt.Println("   Ce serveur utilise un certificat RSA/ECDSA standard.")
	fmt.Println("   Pour tester les certificats PQC, utilisez un serveur OpenSSL.")
	fmt.Println()

	// Essayer de charger le certificat
	_, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Fatalf("Erreur lors du chargement du certificat/clé: %v\n"+
			"Le certificat doit être RSA ou ECDSA (pas PQC).\n"+
			"Générez un certificat standard avec: openssl req -new -x509 -nodes -days 365 -keyout server.key -out server.crt",
			err)
	}

	log.Fatal(server.ListenAndServeTLS(certFile, keyFile))
}
