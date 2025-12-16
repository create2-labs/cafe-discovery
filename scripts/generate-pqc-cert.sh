#!/bin/bash

# Script to generate Post-Quantum Cryptography (PQC) certificates
# Requires OpenSSL 3.x with oqs-provider
# See: https://github.com/open-quantum-safe/oqs-provider

set -e

ALGORITHM=${1:-dilithium3}
DAYS=${2:-365}
CN=${3:-localhost}
CONFIG_FILE=${4:-}

echo "Generating PQC certificate with algorithm: $ALGORITHM"
echo "CN: $CN"
echo "Validity: $DAYS days"
echo ""

# Check OpenSSL version (must be 3.0+)
OPENSSL_VERSION=$(openssl version | awk '{print $2}' | cut -d. -f1)
if [ "$OPENSSL_VERSION" -lt 3 ]; then
    echo "ERROR: OpenSSL 3.0+ required (found: $(openssl version))"
    echo "Please install OpenSSL 3.x or use oqs-provider"
    exit 1
fi

# Check if oqs-provider is available
if ! openssl list -providers 2>/dev/null | grep -q oqsprovider; then
    echo "WARNING: oqs-provider not found. Trying default algorithms..."
    echo "To use PQC algorithms, install oqs-provider:"
    echo "  https://github.com/open-quantum-safe/oqs-provider"
    echo ""
fi

# Check if OpenSSL supports the algorithm
ALGORITHMS_CMD="openssl list -public-key-algorithms"
if [ -n "$CONFIG_FILE" ]; then
    ALGORITHMS_CMD="$ALGORITHMS_CMD -config $CONFIG_FILE"
fi

if ! $ALGORITHMS_CMD 2>/dev/null | grep -q "$ALGORITHM"; then
    echo "ERROR: Algorithm $ALGORITHM not supported by OpenSSL"
    echo ""
    echo "Available algorithms:"
    $ALGORITHMS_CMD | grep -E "(dilithium|falcon|ed25519|ED25519)" || echo "  (none found)"
    echo ""
    echo "If using oqs-provider, ensure it's properly configured:"
    echo "  1. Install oqs-provider"
    echo "  2. Set OPENSSL_CONF to point to openssl-pqc.cnf"
    echo "  3. Or use -config option with openssl-pqc.cnf"
    exit 1
fi

# Build openssl commands with optional config
GENPKEY_CMD="openssl genpkey"
REQ_CMD="openssl req"
if [ -n "$CONFIG_FILE" ]; then
    GENPKEY_CMD="$GENPKEY_CMD -config $CONFIG_FILE"
    REQ_CMD="$REQ_CMD -config $CONFIG_FILE"
fi

# Generate private key
echo "Generating private key..."
$GENPKEY_CMD -algorithm "$ALGORITHM" -out "${CN}-${ALGORITHM}.key"

# Generate self-signed certificate
echo "Generating certificate..."
$REQ_CMD -new -x509 -key "${CN}-${ALGORITHM}.key" \
  -out "${CN}-${ALGORITHM}.crt" \
  -days "$DAYS" \
  -subj "/CN=${CN}/O=Test PQC Certificate/C=FR/ST=Paris/L=Paris"

echo ""
echo "✓ Certificate generated successfully!"
echo "  Private key: ${CN}-${ALGORITHM}.key"
echo "  Certificate: ${CN}-${ALGORITHM}.crt"
echo ""
echo "To view certificate details:"
echo "  openssl x509 -in ${CN}-${ALGORITHM}.crt -text -noout"

