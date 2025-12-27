# Post-Quantum Cryptography JWT Implementation

The service supports hybrid PQC JWT tokens that combine:
- EdDSA (Ed25519): Classical signature algorithm for current security
- ML-DSA-65: Post-quantum signature algorithm for future quantum resistance

This hybrid approach provides security against both classical and quantum attacks. Classic HMAC tokens are not supported.

## Prerequisites

### Installing Open Quantum Safe (OQS) Library

The PQC JWT implementation requires the Open Quantum Safe (OQS) library with ML-DSA-65 support.

#### macOS (Homebrew)

```bash
# Install liboqs
brew install liboqs

# Verify installation
pkg-config --modversion liboqs
```

The library will be installed at:
- Headers: `/opt/homebrew/include` (or `/usr/local/include`)
- Libraries: `/opt/homebrew/lib` (or `/usr/local/lib`)

#### Linux (Debian/Ubuntu)

```bash
# Install dependencies
sudo apt-get update
sudo apt-get install -y build-essential cmake git libssl-dev

# Clone and build liboqs
git clone https://github.com/open-quantum-safe/liboqs.git
cd liboqs
mkdir build && cd build
cmake -DCMAKE_INSTALL_PREFIX=/usr/local ..
make -j$(nproc)
sudo make install

# Update library cache
sudo ldconfig
```

#### Docker

If you're using Docker, you can use a base image with OQS pre-installed. See the `docker/` directory for examples.

## Configuration

### Environment Variables

The application **only supports hybrid PQC tokens** (EdDSA + ML-DSA-65). No policy configuration is needed - hybrid mode is always enabled.

```bash
# JWT_SECRET is still required but not used for signing (kept for backward compatibility)
export JWT_SECRET=your-secret-key-here
```

### CGO Configuration

The PQC package uses CGO to interface with liboqs. The CGO flags are configured in `pkg/pqc/oqs.go` and `pkg/pqc/kem.go`.

For macOS with Homebrew, the default paths are:
- Headers: `/opt/homebrew/opt/openssl@3/include` and `/opt/liboqs/include`
- Libraries: `/opt/homebrew/opt/openssl@3/lib` and `/opt/liboqs/lib`

If your installation is in a different location, you may need to adjust the CGO flags:

```go
/*
#cgo CFLAGS: -I${SRCDIR}/../native -I/path/to/openssl/include -I/path/to/liboqs/include
#cgo LDFLAGS: -L/path/to/openssl/lib -L/path/to/liboqs/lib -lssl -lcrypto -loqs -Wl,-rpath,/path/to/liboqs/lib
*/
```

## JWT Token Format

The application **only supports hybrid PQC tokens** (EdDSA + ML-DSA-65). Classic HMAC tokens are not supported.

### Hybrid Mode (EdDSA + ML-DSA-65)

Uses JWS JSON General Serialization, then base64url-encodes the entire JSON object:

```json
{
  "payload": "<base64url-encoded-claims>",
  "signatures": [
    {
      "protected": "<base64url-encoded-ed25519-header>",
      "signature": "<base64url-encoded-ed25519-signature>"
    },
    {
      "protected": "<base64url-encoded-mldsa65-header>",
      "signature": "<base64url-encoded-mldsa65-signature>"
    }
  ]
}
```

Both signatures must be valid for the token to be accepted.

## Architecture

### Key Management

At server startup:
1. **EdDSA Key Pair** (Ed25519): Generated using Go's standard `crypto/ed25519` package
2. **ML-DSA Key Pair** (ML-DSA-65): Generated using liboqs with context string "JWT"

The keys are stored in memory and used for signing and verification.

### Token Generation

When a user authenticates (signup or signin):
1. Server creates JWT claims (user ID, email, expiration, etc.)
2. Server signs the JWT using both algorithms:
   - EdDSA signature over the header and payload
   - ML-DSA-65 signature over the header and payload
3. Server returns the signed token to the client

### Token Verification

When a protected endpoint is accessed:
1. Server extracts the token from the `Authorization: Bearer <token>` header
2. Server verifies both signatures:
   - EdDSA signature must be valid
   - ML-DSA-65 signature must be valid
3. Server validates claims (expiration, etc.)
4. If all checks pass, the request is authorized

## Usage

### Starting the Server

The server always uses hybrid PQC tokens:

```bash
export JWT_SECRET=your-secret-key-here
go run cmd/server/main.go
```

### Testing

The frontend and API clients don't need any changes - they simply send the token in the `Authorization` header. The backend only accepts hybrid PQC tokens (EdDSA + ML-DSA-65).

### Example: Sign In

```bash
# Sign in (always returns hybrid PQC token)
curl -X POST http://localhost:8080/auth/signin \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "password123"}'

# Response includes the hybrid JWT token
{
  "token": "eyJwYXlsb2FkIjoi...",  # base64url-encoded hybrid JWS JSON
  "user": { ... }
}

# Use the token for authenticated requests
curl -X GET http://localhost:8080/discovery/scans \
  -H "Authorization: Bearer eyJwYXlsb2FkIjoi..."
```

## Troubleshooting

### OQS library not found

If you get linker errors about `-loqs`:

1. Verify liboqs is installed:
   ```bash
   pkg-config --modversion liboqs
   ```

2. Check library paths:
   ```bash
   # macOS
   ls -la /opt/homebrew/lib/liboqs.dylib
   
   # Linux
   ls -la /usr/local/lib/liboqs.so
   ```

3. Update CGO flags if needed (see Configuration section)

### ML-DSA-65 not available

If you get an error that ML-DSA-65 is not available:

1. Verify liboqs was compiled with ML-DSA-65 support:
   ```bash
   # Check available algorithms
   # The error message will list all enabled algorithms
   ```

2. Rebuild liboqs with ML-DSA-65 enabled (it should be enabled by default)

### Build Errors

If you encounter build errors:

1. Ensure CGO is enabled (it's enabled by default in Go)
2. Check that C compiler is available:
   ```bash
   gcc --version
   ```
3. Verify all dependencies are installed (OpenSSL, liboqs)

## Security Considerations

⚠️ **Important Security Notes**:

1. **Key Storage**: Server private keys are stored in memory. In production:
   - Consider using a Hardware Security Module (HSM)
   - Implement key rotation policies
   - Use secure key management services

2. **Token Size**: Hybrid tokens are larger than classic tokens (due to ML-DSA-65 signatures). Ensure your HTTP infrastructure can handle larger headers.

3. **Performance**: ML-DSA-65 signatures are slower than EdDSA. Consider:
   - Token caching strategies
   - Signature verification optimization
   - Load testing with hybrid tokens

4. **Token Format**: Only hybrid PQC tokens are supported. Classic HMAC tokens are not accepted.

## References

- [Open Quantum Safe](https://openquantumsafe.org/)
- [liboqs](https://github.com/open-quantum-safe/liboqs)
- [RFC 7519 - JSON Web Token (JWT)](https://tools.ietf.org/html/rfc7519)
- [RFC 7515 - JSON Web Signature (JWS)](https://tools.ietf.org/html/rfc7515)
- [lodygens/pqc-jwt](https://github.com/lodygens/pqc-jwt) - Original implementation

