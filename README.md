# Cafe Discovery Service

A Discovery service for identifying cryptographic exposures and quantum vulnerabilities on the Ethereum network and related infrastructure.

## Features

- **Wallet Scanning**: Scan wallets across multiple EVM-compatible networks
- **Key Exposure Detection**: Detect whether a wallet's public key has been revealed on-chain
- **Account Type Detection**: Determine if an address is an EOA (Externally Owned Account) or AA (Abstract Account/ERC-4337)
- **Risk Assessment**: Calculate risk scores based on exposure across networks
- **Quantum Security Level**: Assess NIST quantum-security levels
- **TLS Scanning**: Scan TLS endpoints for post-quantum cryptography (PQC) certificate support
- **Post-Quantum JWT**: Hybrid PQC JWT tokens (EdDSA + ML-DSA-65) for quantum-resistant authentication

## Architecture

The application uses an asynchronous message-based architecture with NATS and PostgreSQL to improve scalability and performance.

### System Components

#### 1. API Server (`cmd/server`)

- **Role**: HTTP server (Fiber) that exposes REST endpoints
- **Responsibilities**:
  - User authentication with hybrid PQC JWT tokens
  - Receiving scan requests (wallet and TLS)
  - Publishing NATS messages for asynchronous processing
  - Reading results from PostgreSQL

#### 2. Worker (`cmd/worker`)

- **Role**: NATS consumer that processes scans
- **Responsibilities**:
  - Consuming NATS messages (wallet scan, TLS scan)
  - Processing scans (calling services)
  - Saving results to PostgreSQL

#### 3. NATS

- **Role**: Messaging system for asynchronous communication
- **Subjects**:
  - `cafe.discovery.wallet.scan`: Wallet scan requests
  - `cafe.discovery.tls.scan`: TLS scan requests
- **Queue**: `cafe.workers` (enables load distribution between multiple workers)

#### 4. PostgreSQL

- **Role**: Primary database
- **Advantages**:
  - Better performance for complex queries
  - Native JSON support
  - ACID transactions
  - Horizontal scalability with read replicas

### Project Structure

```
cafe-discovery/
├── cmd/   
│   ├── server/            # API server entrypoint
│   ├── worker/            # Worker entrypoint for background processing
│   └── cli/               # Some command line tools
│      └── publickey/      # Utility for testing public key recovery
├── internal/
│   ├── app/               # Application container (orchestration)
│   ├── domain/            # Domain models and types
│   ├── handler/           # HTTP handlers (Fiber)
│   ├── service/           # Business logic
│   └── worker/            # NATS workers (wallet & TLS scanning)
├── pkg/
│   ├── evm/               # EVM client for blockchain interactions
│   ├── nats/              # NATS messaging client
│   ├── postgres/          # PostgreSQL database client
│   ├── pqc/               # Post-quantum cryptography (JWT, KEM)
│   └── tls/               # TLS scanner with PQC support
├── docs/                  # Documentation
│   ├── PQC_CERTIFICATES.md # PQC certificate generation guide
│   └── PQC_JWT.md         # PQC JWT implementation guide
└── config.yaml            # Configuration file
```

### Data Flow

#### Wallet Scan

```
Client HTTP → API Server → NATS (publish) → Worker → Service → PostgreSQL
                                    ↓
                              Immediate Response
```

1. Client sends a POST request to `/discovery/scan`
2. API Server validates the request and publishes a NATS message
3. Client receives an immediate response: `{"status": "processing"}`
4. A worker consumes the message and processes the scan
5. The result is saved to PostgreSQL

#### TLS Scan

Same flow as wallet scan, but using the `cafe.discovery.tls.scan` subject.

### Architecture Benefits

1. **Scalability**: Workers can be scaled independently
2. **Resilience**: NATS messages can be persisted (JetStream)
3. **Performance**: HTTP requests return immediately
4. **Decoupling**: API and processing are separated
5. **Load Distribution**: Multiple workers share the load via NATS queues

### Deployment Considerations

#### Development

- Use Docker Compose for local PostgreSQL and NATS instances
- Run API server and worker as separate processes

#### Production

- **API Server**: Deploy multiple instances behind a load balancer
- **Workers**: Deploy multiple instances for horizontal scalability
- **NATS**: Use NATS JetStream for persistence and high availability
- **PostgreSQL**: Use a PostgreSQL cluster with read replicas

### Migration Notes

The application was migrated from MySQL to PostgreSQL. Repositories use GORM which abstracts database differences, but some queries may require adjustments.

## Configuration

Edit `config.yaml` to configure server and blockchain RPC endpoints:

```yaml
server:
  host: "0.0.0.0"
  port: "8080"

blockchains:
  - name: ethereum-mainnet
    rpc: "https://ethereum-rpc.publicnode.com"
  - name: polygon
    rpc: "https://polygon.llamarpc.com"
  # ... more networks
```

## Prerequisites

- Go 1.24+ 
- Docker and Docker Compose
- PostgreSQL 16+ (via Docker)
- NATS (via Docker)
- **Required for JWT authentication**: Open Quantum Safe (OQS) library (liboqs) with ML-DSA-65 support
  - The service uses hybrid PQC JWT tokens (EdDSA + ML-DSA-65) for all authentication
  - See [Post-Quantum Cryptography](#post-quantum-cryptography-pqc) section for installation instructions

## Running the Service

The application consists of three components that need to be running:

### Step 1: Start Infrastructure Services

Start PostgreSQL and NATS using Docker Compose:

```bash
docker-compose up -d
```

This will start:
- PostgreSQL on port `5432`
- NATS on ports `4222` (client) and `8222` (monitoring)

Verify services are running:
```bash
docker-compose ps
```

### Step 2: Install Dependencies

```bash
go mod tidy
```

### Step 3: Start the API Server

In a separate terminal:

```bash
go run cmd/server/main.go
```

Or with a custom config path:
```bash
CONFIG_PATH=config.yaml go run cmd/server/main.go
```

The API server will start on `http://localhost:8080` by default.

### Step 4: Start the Worker

In another terminal:

```bash
go run cmd/worker/main.go
```

The worker processes background tasks:
- **Wallet Worker**: Processes wallet scan requests from NATS
- **TLS Worker**: Processes TLS scan requests from NATS

### Environment Variables

You can configure the application using environment variables:

```bash
# Configuration file path
export CONFIG_PATH=config.yaml

# Server configuration
export SERVER_HOST=0.0.0.0
export SERVER_PORT=8080

# Worker health check port
export WORKER_HEALTH_PORT=8081

# PostgreSQL configuration (defaults match docker-compose.yml)
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_DATABASE=cafe
export POSTGRES_USER=cafe
export POSTGRES_PASSWORD=cafe
export POSTGRES_SSLMODE=disable

# NATS configuration
export NATS_URL=nats://localhost:4222

# JWT configuration (required for authentication)
# Note: The service always uses hybrid PQC tokens (EdDSA + ML-DSA-65)
# JWT_SECRET is kept for API compatibility but not used for token signing
export JWT_SECRET=your-secret-key-here

# Moralis API (required for wallet scanning features)
# Get your API key from https://moralis.io
export MORALIS_API_KEY=your_api_key_here
export MORALIS_API_URL=https://deep-index.moralis.io

# Cloudflare Turnstile (required for signup/signin protection)
# Development keys are configured by default (always pass verification)
# Development keys (default):
#   Site Key: 1x00000000000000000000AA
#   Secret Key: 1x0000000000000000000000000000000AA
# For production, get your keys from https://developers.cloudflare.com/turnstile/
# Note: The service will log a warning when using development keys
export TURNSTILE_SECRET_KEY=1x0000000000000000000000000000000AA  # Dev key (default)
export TURNSTILE_SITE_KEY=1x00000000000000000000AA  # Dev key (default)

# Logging
export LOG_LEVEL=info  # Options: trace, debug, info, warn, error, fatal, panic
```

### Démarrer en mode debug

Pour activer les logs de debug, définissez la variable d'environnement `LOG_LEVEL` à `debug` :

```bash
# Mode debug
export LOG_LEVEL=debug
go run cmd/server/main.go

# Ou pour le worker
export LOG_LEVEL=debug
go run cmd/worker/main.go
```

**Niveaux de log disponibles :**
- `trace` : Niveau le plus détaillé (tous les logs)
- `debug` : Logs de débogage détaillés
- `info` : Informations générales (par défaut)
- `warn` : Avertissements
- `error` : Erreurs uniquement
- `fatal` : Erreurs fatales uniquement
- `panic` : Panics uniquement

**Exemple avec les deux services en mode debug :**

```bash
# Terminal 1 - Serveur en mode debug
export LOG_LEVEL=debug
go run cmd/server/main.go

# Terminal 2 - Worker en mode debug
export LOG_LEVEL=debug
go run cmd/worker/main.go
```

### Quick Start Script

You can create a simple startup script:

```bash
#!/bin/bash
# Start all services

echo "Starting infrastructure services..."
docker-compose up -d

echo "Waiting for services to be ready..."
sleep 5

echo "Starting API server..."
go run cmd/server/main.go &
SERVER_PID=$!

echo "Starting worker..."
go run cmd/worker/main.go &
WORKER_PID=$!

echo "Services started!"
echo "API Server PID: $SERVER_PID"
echo "Worker PID: $WORKER_PID"
echo ""
echo "To stop services:"
echo "  kill $SERVER_PID $WORKER_PID"
echo "  docker-compose down"
```

## Post-Quantum Cryptography (PQC)

The service implements post-quantum cryptography for both authentication (JWT) and TLS scanning capabilities.

### PQC JWT Authentication

The service uses **hybrid PQC JWT tokens** that combine:
- **EdDSA (Ed25519)**: Classical signature algorithm for current security
- **ML-DSA-65**: Post-quantum signature algorithm for future quantum resistance

This hybrid approach provides security against both classical and quantum attacks. Classic HMAC tokens are not supported.

#### Prerequisites for PQC JWT

The PQC JWT implementation requires the Open Quantum Safe (OQS) library with ML-DSA-65 support.

**macOS (Homebrew):**
```bash
brew install liboqs
pkg-config --modversion liboqs
```

**Linux (Debian/Ubuntu):**
```bash
sudo apt-get update
sudo apt-get install -y build-essential cmake git libssl-dev
git clone https://github.com/open-quantum-safe/liboqs.git
cd liboqs
mkdir build && cd build
cmake -DCMAKE_INSTALL_PREFIX=/usr/local ..
make -j$(nproc)
sudo make install
sudo ldconfig
```

#### JWT Token Format

The application only supports hybrid PQC tokens (EdDSA + ML-DSA-65). Uses JWS JSON General Serialization:

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

#### Configuration

The application **always uses hybrid PQC tokens** (EdDSA + ML-DSA-65). No policy configuration is needed - hybrid mode is always enabled. Classic HMAC tokens are not supported.

```bash
# JWT_SECRET is required but not used for signing (kept for API compatibility)
export JWT_SECRET=your-secret-key-here
```

**Important**: 
- The OQS library must be installed and available for the service to start
- If OQS is not found or ML-DSA-65 is not available, the service will fail to initialize the authentication service
- The service will log an error message listing available algorithms if ML-DSA-65 is not found
- See [docs/PQC_JWT.md](docs/PQC_JWT.md) for detailed installation instructions

#### Security Considerations

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

For more details, see [docs/PQC_JWT.md](docs/PQC_JWT.md).

### PQC TLS Certificate Scanning

The service can scan TLS endpoints to detect post-quantum certificate support. You can generate PQC certificates for testing using the provided tools.

#### Generating PQC Certificates

**Quick method with script:**
```bash
./scripts/generate-pqc-cert.sh dilithium3 365 localhost
```

**Available PQC Algorithms:**

| Algorithm    | NIST Level | Usage                               |
| ------------ | ---------- | ----------------------------------- |
| `dilithium2` | 2          | Signatures, medium size             |
| `dilithium3` | 3          | Signatures, recommended             |
| `dilithium5` | 5          | Signatures, maximum security        |
| `falcon512`  | 1          | Signatures, compact                 |
| `falcon1024` | 5          | Signatures, high security           |
| `ED25519`    | -          | Quantum-resistant, widely supported |

#### Prerequisites for PQC Certificates

**Option 1: oqs-provider with OpenSSL 3.x (Recommended)**

```bash
# Install OpenSSL 3.x
sudo apt-get update
sudo apt-get install openssl libssl-dev

# Install oqs-provider
git clone https://github.com/open-quantum-safe/oqs-provider.git
cd oqs-provider
mkdir _build && cd _build
cmake ..
make -j$(nproc)
sudo make install
```

**Option 2: Standard OpenSSL (for Ed25519)**

Ed25519 is quantum-resistant and supported by standard OpenSSL 1.1.1+.

#### Testing with PQC Certificates

1. Generate a certificate:
```bash
./scripts/generate-pqc-cert.sh dilithium3 365 localhost
```

2. Run a test HTTPS server (see `docker/test-server.go` for example)

3. Scan with the API:
```bash
curl -X POST http://localhost:8080/discovery/tls/scan \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"url": "https://localhost:8443"}'
```

#### Current Limitations

⚠️ **Important**: PQC certificates have limitations:

1. **Browser support**: Browsers do not yet natively support PQC certificates
2. **TLS 1.3**: PQC support in TLS 1.3 is still experimental
3. **Certificate authorities**: No public CA currently issues PQC certificates
4. **Interoperability**: Few servers/clients currently support PQC certificates

For detailed instructions, see [docs/PQC_CERTIFICATES.md](docs/PQC_CERTIFICATES.md).

## API Endpoints

### Authentication

Most endpoints require JWT authentication. The service uses hybrid PQC JWT tokens (EdDSA + ML-DSA-65).

### POST /auth/signup

Register a new user account. Requires Cloudflare Turnstile verification.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "securepassword",
  "confirm_password": "securepassword",
  "turnstile_token": "0.abcdefghijklmnopqrstuvwxyz..."
}
```

**Note**: The `turnstile_token` is generated by the Cloudflare Turnstile widget on the frontend. By default, the service uses Cloudflare's free development keys which always pass verification. The service will log a warning when using development keys. For production, configure production keys from your Cloudflare dashboard.

### POST /auth/signin

Sign in and receive a hybrid PQC JWT token. Requires Cloudflare Turnstile verification.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "securepassword",
  "turnstile_token": "0.abcdefghijklmnopqrstuvwxyz..."
}
```

**Note**: The `turnstile_token` is generated by the Cloudflare Turnstile widget on the frontend. By default, the service uses Cloudflare's free development keys which always pass verification. The service will log a warning when using development keys. For production, configure production keys from your Cloudflare dashboard.

**Response:**
```json
{
  "token": "eyJwYXlsb2FkIjoi...",
  "user": {
    "id": "uuid",
    "email": "user@example.com"
  }
}
```

The token is a hybrid PQC JWT (base64url-encoded JWS JSON General Serialization format).

### POST /discovery/scan

Scans a wallet address across all configured networks. Requires authentication. The scan is processed asynchronously via NATS.

**Request:**
```json
{
  "address": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
}
```

**Response:**
```json
{
  "message": "scan queued successfully",
  "address": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
  "status": "processing"
}
```

### GET /discovery/scans

Returns paginated list of wallet scan results for the authenticated user.

**Query Parameters:**
- `limit` (optional): Number of results per page (default: 20)
- `offset` (optional): Number of results to skip (default: 0)

**Response:**
```json
{
  "results": [
    {
      "address": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
      "type": "EOA",
      "algorithm": "ECDSA-secp256k1",
      "nist_level": 1,
      "key_exposed": true,
      "risk_score": 0.85,
      "first_seen": "2025-03-10T15:22:00Z",
      "last_seen": "2025-10-16T08:10:00Z",
      "networks": ["ethereum-mainnet", "polygon"]
    }
  ],
  "total": 1,
  "limit": 20,
  "offset": 0,
  "count": 1
}
```

### POST /discovery/tls/scan

Scans a TLS endpoint for quantum-safe certificate support. Requires authentication. The scan is processed asynchronously via NATS.

**Request:**
```json
{
  "url": "https://example.com"
}
```

**Note**: You can specify a custom port in the URL (e.g., `https://example.com:8443`). If no port is specified, port 443 is used by default for HTTPS URLs.

**Response:**
```json
{
  "message": "scan queued successfully",
  "endpoint": "https://example.com",
  "status": "processing"
}
```

### GET /discovery/tls/scans

Returns paginated list of TLS scan results for the authenticated user.

**Query Parameters:**
- `limit` (optional): Number of results per page (default: 20)
- `offset` (optional): Number of results to skip (default: 0)

### GET /discovery/rpcs

Returns the list of configured RPC endpoints. No authentication required.

**Response:**
```json
{
  "blockchains": [
    {
      "name": "ethereum-mainnet",
      "rpc": "https://ethereum-rpc.publicnode.com"
    },
    {
      "name": "polygon",
      "rpc": "https://polygon.llamarpc.com"
    }
  ],
  "count": 6
}
```

### GET /health

Health check endpoint. No authentication required.

**Response:**
```json
{
  "status": "ok",
  "app_name": "Cafe Discovery Service",
  "version": "1.0.0",
  "timestamp": "2025-01-15T10:30:00Z"
}
```

### Worker Health Check

The worker exposes a health check endpoint on port `8081` (configurable via `WORKER_HEALTH_PORT` environment variable).

**Endpoint:** `GET http://localhost:8081/health`

**Response (healthy):**
```json
{
  "status": "ok",
  "app_name": "Cafe Discovery Worker",
  "timestamp": "2025-01-15T10:30:00Z",
  "checks": {
    "nats": {
      "connected": true
    },
    "workers": {
      "wallet": {
        "running": true
      },
      "tls": {
        "running": true
      }
    }
  }
}
```

**Response (degraded):**
Returns HTTP 503 status code when NATS is disconnected or workers are not running.

## Testing

### 1. Register and Authenticate

**Note**: The signup and signin endpoints require a Cloudflare Turnstile token. By default, the service uses Cloudflare's free development keys which always pass verification. The service will log a warning when using development keys. For production, configure production keys from your Cloudflare dashboard.

```bash
# Register a new user (requires turnstile_token from frontend widget)
curl -X POST http://localhost:8080/auth/signup \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "testpassword123",
    "confirm_password": "testpassword123",
    "turnstile_token": "your_turnstile_token_here"
  }'

# Sign in and get JWT token (hybrid PQC token, requires turnstile_token)
TOKEN=$(curl -s -X POST http://localhost:8080/auth/signin \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "testpassword123",
    "turnstile_token": "your_turnstile_token_here"
  }' \
  | jq -r '.token')

echo "Token: $TOKEN"
```

**Getting Turnstile Tokens**: In a real application, the Turnstile token is generated by the Cloudflare Turnstile widget embedded in the frontend. For API testing, you can:
1. Use the frontend to get a valid token
2. Or temporarily disable Turnstile verification by not setting `TURNSTILE_SECRET_KEY` (development only)

### 2. Test Wallet Scanning

```bash
# Queue a wallet scan (requires authentication)
curl -X POST http://localhost:8080/discovery/scan \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"address": "0x13f735c915bba9136Db794F6b1f42566B24861B8"}'

# List scan results
curl -X GET "http://localhost:8080/discovery/scans?limit=10&offset=0" \
  -H "Authorization: Bearer $TOKEN"
```

### 3. Test TLS Scanning

```bash
# Queue a TLS scan (default port 443)
curl -X POST http://localhost:8080/discovery/tls/scan \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"url": "https://example.com"}'

# Queue a TLS scan with custom port (e.g., 8443)
curl -X POST http://localhost:8080/discovery/tls/scan \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"url": "https://localhost:8443"}'

# List TLS scan results
curl -X GET "http://localhost:8080/discovery/tls/scans?limit=10&offset=0" \
  -H "Authorization: Bearer $TOKEN"
```

### 4. Public Endpoints

```bash
# List configured RPC endpoints (no auth required)
curl http://localhost:8080/discovery/rpcs

# Health check (no auth required)
curl http://localhost:8080/health

# Worker health check (no auth required)
curl http://localhost:8081/health
```

## Risk Scoring

### Wallet Risk Score

The wallet risk score (0.0 to 1.0, where higher = higher risk) is calculated based on:

1. **Base Risk**: NIST Level 1 (ECDSA-secp256k1) contributes 0.5 base risk (quantum-broken)
2. **Network Exposure**: Each network where the key is exposed adds up to 0.4 risk
3. **Transaction Count**: More transactions increase risk (logarithmic scale):
   - 1-10 transactions: +0.05
   - 10-100 transactions: +0.15
   - 100+ transactions: +0.25

**Key Exposure Detection**: A wallet's public key is considered exposed if it has sent at least one transaction (nonce > 0), making it vulnerable to quantum attacks once quantum computers are available.

**Account Type Detection**:
- **EOA**: Externally Owned Account using ECDSA-secp256k1 (quantum-breakable)
- **AA**: Abstract Account compliant with ERC-4337 (potentially more flexible for PQC migration)

### TLS Risk Score

The TLS risk score (0.0 to 1.0, where higher = higher risk) is a comprehensive assessment of TLS endpoint security, considering both classical and post-quantum cryptography factors.

#### Calculation Method

The risk score uses a weighted combination of multiple security factors:

**1. Base Risk (40% weight)**
- Based on the worst NIST security level across all TLS components
- Uses detailed NIST levels (kex, sig, cipher, hkdf, session) if available from PQC scan
- Falls back to certificate and cipher suite levels if detailed levels are not available
- Formula: `risk = 1.0 - ((level - 1) / 4)`
  - NIST Level 1 (quantum-broken): 1.0 risk
  - NIST Level 3 (moderate): 0.5 risk
  - NIST Level 5 (PQC-ready): 0.0 risk

**2. Cipher Suite Risk (25% weight)**
- Evaluates the weakest cipher suite supported
- No cipher suites available: 1.0 risk (critical)
- Uses the same NIST level mapping as base risk

**3. Protocol Version Risk (15% weight)**
- TLS 1.3: 0.0 risk (most secure)
- TLS 1.2: 0.3 risk (acceptable but older)
- TLS 1.1 or older: 0.8 risk (deprecated, insecure)
- Unknown protocol: 0.5 risk (moderate)

**4. Security Features (10% weight)**
- Perfect Forward Secrecy (PFS) and OCSP Stapling reduce risk:
  - Both PFS and OCSP: 0.0 additional risk
  - PFS only: 0.2 additional risk
  - OCSP only: 0.3 additional risk
  - Neither: 0.5 additional risk

**5. Post-Quantum Cryptography Readiness (10% weight)**
- Pure or hybrid PQC mode: 0.0 quantum risk (fully protected)
- PQC KEX ready (but not in PQC mode): 0.2 quantum risk
- High NIST level (≥4) but no PQC: 0.3 quantum risk
- Low NIST level or no PQC: 0.5 quantum risk

#### Final Score

The final risk score is calculated as:
```
risk_score = (base_risk × 0.40) +
             (cipher_risk × 0.25) +
             (protocol_risk × 0.15) +
             (security_features_risk × 0.10) +
             (pqc_risk × 0.10)
```

The score is clamped between 0.0 (lowest risk) and 1.0 (highest risk).

#### Risk Categories

- **0.0 - 0.1**: Very Low Risk - Excellent TLS configuration with PQC support
- **0.1 - 0.4**: Low Risk - Good TLS configuration, minor improvements possible
- **0.4 - 0.7**: Medium Risk - Acceptable but should be improved
- **0.7 - 1.0**: High Risk - Critical security issues, immediate action required

## Background Processing

The application uses NATS for asynchronous message processing:

- **Wallet Scans**: When a scan is requested via the API, it's queued in NATS and processed by the wallet worker
- **TLS Scans**: TLS endpoint scans are also queued and processed by the TLS worker
- **Scalability**: Multiple worker instances can be run to process messages in parallel

## Development Tools

### Public Key Recovery Utility (`cmd/cli/publickey`)

A development utility for testing public key recovery from blockchain transactions. This tool demonstrates how the service recovers public keys from transaction data.

**Usage:**

```bash
# Set required environment variable
export MORALIS_API_KEY=your_api_key_here

# Run the utility
go run cmd/cli/publickey/getpublickey.go
```

**Note:** This utility requires a valid Moralis API key to fetch transaction data. The API key must be provided via the `MORALIS_API_KEY` environment variable.

## Security Notes

⚠️ **Important**: Never commit API keys or sensitive credentials to version control. Always use environment variables or secure configuration management:

- Use environment variables for all API keys
- Never hardcode credentials in source code
- Use `.env` files (and add them to `.gitignore`) for local development
- Use secret management services in production

## Stopping the Application

To stop all services:

```bash
# Stop Go processes (Ctrl+C in each terminal)
# Or find and kill processes:
pkill -f "go run cmd/server/main.go"
pkill -f "go run cmd/worker/main.go"

# Stop Docker services
docker-compose down
```

To stop and remove volumes (clears database):

```bash
docker-compose down -v
```

## Additional Resources

- [Post-Quantum JWT Documentation](docs/PQC_JWT.md) - Detailed guide on PQC JWT implementation
- [PQC Certificate Generation Guide](docs/PQC_CERTIFICATES.md) - Guide for generating and testing PQC TLS certificates
- [Open Quantum Safe](https://openquantumsafe.org/) - Official OQS project
- [NIST PQC Standards](https://csrc.nist.gov/projects/post-quantum-cryptography) - NIST post-quantum cryptography standards
