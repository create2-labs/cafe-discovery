#!/bin/bash
# Script pour générer un certificat RSA standard pour le serveur Go
# (car Go ne supporte pas les certificats PQC nativement)

OUTPUT_DIR=${1:-/certs}
CN=${2:-localhost}

echo "Génération d'un certificat RSA standard pour le serveur Go..."
echo "CN: $CN"
echo ""

cd "$OUTPUT_DIR"

# Générer une clé RSA
echo "Génération de la clé privée RSA..."
openssl genrsa -out "${CN}-rsa.key" 2048

# Générer le certificat
echo "Génération du certificat..."
openssl req -new -x509 -key "${CN}-rsa.key" \
    -out "${CN}-rsa.crt" \
    -days 365 \
    -subj "/CN=${CN}/O=Test Certificate/C=FR/ST=Paris/L=Paris"

echo ""
echo "✓ Certificat RSA généré avec succès!"
echo "  Clé privée: ${OUTPUT_DIR}/${CN}-rsa.key"
echo "  Certificat: ${OUTPUT_DIR}/${CN}-rsa.crt"
echo ""
echo "Utilisez ce certificat pour le serveur Go:"
echo "  start-test-server.sh /certs/${CN}-rsa.crt /certs/${CN}-rsa.key 8443"



