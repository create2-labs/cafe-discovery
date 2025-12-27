# Post-Quantum Cryptography (PQC) Certificate Generation

This guide explains how to generate TLS certificates using post-quantum algorithms for testing the TLS scanner.

## Prerequisites

### Option 1: oqs-provider with OpenSSL 3.x (Recommended - Simplest)

`oqs-provider` is an OpenSSL 3 provider that adds support for post-quantum algorithms. This is the simplest and most modern method.

**Reference**: [https://github.com/open-quantum-safe/oqs-provider](https://github.com/open-quantum-safe/oqs-provider)

#### Installing OpenSSL 3.x

```bash
# On Ubuntu/Debian
sudo apt-get update
sudo apt-get install openssl libssl-dev

# Verify version (must be 3.0+)
openssl version
```

#### Installing oqs-provider

```bash
# Clone the repository
git clone https://github.com/open-quantum-safe/oqs-provider.git
cd oqs-provider

# Install dependencies (liboqs will be compiled automatically)
mkdir _build && cd _build
cmake ..
make -j$(nproc)
sudo make install

# Configure OpenSSL to use the provider
# Add to ~/.bashrc or ~/.zshrc
export OPENSSL_CONF=/usr/local/ssl/openssl.cnf
# Or create a local openssl.cnf (see below)
```

#### Configuring OpenSSL to use oqs-provider

Create an `openssl-pqc.cnf` file:

```ini
openssl_conf = openssl_init

[openssl_init]
providers = provider_sect

[provider_sect]
default = default_sect
oqsprovider = oqsprovider_sect

[default_sect]
activate = 1

[oqsprovider_sect]
activate = 1
```

Use this file:
```bash
export OPENSSL_CONF=/path/to/openssl-pqc.cnf
# Or use -config in commands
openssl req -config openssl-pqc.cnf ...
```

#### Verifying installation

```bash
# List available PQC algorithms
openssl list -provider oqsprovider -public-key-algorithms | grep -E "(dilithium|falcon)"

# Should display: dilithium2, dilithium3, dilithium5, falcon512, falcon1024, etc.
```

### Option 2: Standard OpenSSL (for Ed25519)

Ed25519 is quantum-resistant and supported by standard OpenSSL:

```bash
openssl version
# Version 1.1.1 or higher recommended
```

### Option 3: OpenSSL with Full Open Quantum Safe (Legacy method)

If you prefer to compile full OpenSSL-OQS (more complex method):

```bash
# Install OpenSSL-OQS
git clone https://github.com/open-quantum-safe/openssl.git
cd openssl
git submodule update --init
./Configure linux-x86_64 --with-oqs
make -j$(nproc)
sudo make install
```

## Certificate Generation

### Quick method with script

```bash
# Use the provided script (ensure oqs-provider is configured)
./scripts/generate-pqc-cert.sh dilithium3 365 localhost

# Or with custom options
./scripts/generate-pqc-cert.sh falcon512 365 test.example.com
```

### Manual method with oqs-provider

**Important**: Ensure `oqs-provider` is installed and configured (see prerequisites above).

#### 1. Certificate with Dilithium3 (NIST Standard)

```bash
# With provider configured globally
openssl genpkey -algorithm dilithium3 -out dilithium3.key
openssl req -new -x509 -key dilithium3.key \
  -out dilithium3.crt -days 365 \
  -subj "/CN=localhost/O=Test PQC/C=US"

# OR with -config if using a local config file
openssl genpkey -config openssl-pqc.cnf \
  -algorithm dilithium3 -out dilithium3.key
openssl req -config openssl-pqc.cnf -new -x509 \
  -key dilithium3.key -out dilithium3.crt -days 365 \
  -subj "/CN=localhost/O=Test PQC/C=US"
```

#### 2. Certificate with Falcon1024 (NIST Standard)

```bash
openssl genpkey -algorithm falcon1024 -out falcon1024.key
openssl req -new -x509 -key falcon1024.key \
  -out falcon1024.crt -days 365 \
  -subj "/CN=localhost/O=Test PQC/C=US"
```

#### 3. Certificate with Ed25519 (Quantum-Resistant, widely supported)

```bash
openssl genpkey -algorithm ED25519 -out ed25519.key
openssl req -new -x509 -key ed25519.key \
  -out ed25519.crt -days 365 \
  -subj "/CN=localhost/O=Test Quantum-Resistant/C=US"
```

## Available PQC Algorithms

### Digital signatures (NIST standardized)

| Algorithm    | NIST Level | Usage                        |
| ------------ | ---------- | ---------------------------- |
| `dilithium2` | 2          | Signatures, medium size      |
| `dilithium3` | 3          | Signatures, recommended      |
| `dilithium5` | 5          | Signatures, maximum security |
| `falcon512`  | 1          | Signatures, compact          |
| `falcon1024` | 5          | Signatures, high security    |

### Quantum-resistant algorithms (non-PQC standard)

| Algorithm | Support | Usage                         |
| --------- | ------- | ----------------------------- |
| `ED25519` | Wide    | Compatible, quantum-resistant |
| `ED448`   | Medium  | Alternative to Ed25519        |

## Testing with a local HTTPS server

### 1. Generate the certificate

```bash
./scripts/generate-pqc-cert.sh dilithium3 365 localhost
```

### 2. Run an HTTPS server with Go

Create a `test-server.go` file:

```go
package main

import (
    "crypto/tls"
    "fmt"
    "net/http"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Hello from PQC server!")
    })

    server := &http.Server{
        Addr:    ":8443",
        Handler: mux,
        TLSConfig: &tls.Config{
            MinVersion: tls.VersionTLS12,
        },
    }

    fmt.Println("Starting PQC server on :8443")
    err := server.ListenAndServeTLS("localhost-dilithium3.crt", "localhost-dilithium3.key")
    if err != nil {
        panic(err)
    }
}
```

### 3. Run the server

```bash
go run test-server.go
```

### 4. Scan with the API

```bash
curl -X POST http://localhost:8080/discovery/scan/endpoints \
  -H "Content-Type: application/json" \
  -d '{"url": "https://localhost:8443"}' | jq
```

## Certificate Verification

```bash
# View certificate details
openssl x509 -in dilithium3.crt -text -noout

# Verify signature algorithm
openssl x509 -in dilithium3.crt -text -noout | grep "Signature Algorithm"

# Verify public key algorithm
openssl x509 -in dilithium3.crt -text -noout | grep "Public Key Algorithm"
```

## Current Limitations

⚠️ **Important**: PQC certificates have limitations:

1. **Browser support**: Browsers do not yet natively support PQC certificates
2. **TLS 1.3**: PQC support in TLS 1.3 is still experimental
3. **Certificate authorities**: No public CA currently issues PQC certificates
4. **Interoperability**: Few servers/clients currently support PQC certificates

## Resources

- **[oqs-provider](https://github.com/open-quantum-safe/oqs-provider)** - OpenSSL 3 provider for PQC algorithms (Recommended)
- [Open Quantum Safe](https://openquantumsafe.org/)
- [NIST PQC Standards](https://csrc.nist.gov/projects/post-quantum-cryptography)
- [OQS-OpenSSL Repository](https://github.com/open-quantum-safe/openssl) (alternative method)
- [oqs-provider Documentation](https://github.com/open-quantum-safe/oqs-provider/blob/main/USAGE.md)
