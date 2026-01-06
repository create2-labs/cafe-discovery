# Cafe Discovery Service

A Discovery service for identifying cryptographic exposures and quantum vulnerabilities on the Ethereum network and related infrastructure.

## Features

- Wallet Scanning: Scan wallets across multiple EVM-compatible networks
- Key Exposure Detection: Detect whether a wallet's public key has been revealed on-chain
- Account Type Detection: Determine if an address is an EOA (Externally Owned Account) or AA (Abstract Account/ERC-4337)
- Risk Assessment: Calculate risk scores based on exposure across networks
- Quantum Security Level: Assess NIST quantum-security levels
- TLS Scanning: Scan TLS endpoints for post-quantum cryptography (PQC) certificate support
- Post-Quantum JWT: Hybrid PQC JWT tokens (EdDSA + ML-DSA-65) for quantum-resistant authentication

## Architecture

The application is designed to be scalabled with a focus on performances.

### Goals

1. Scalability: threaded processes (Workers) to be able to scale
2. Resilience: NATS messages can be persisted with JetStream; this is not implemented yet
3. Performance: HTTP requests return immediately
4. Decoupling: API and processing are separated
5. Load Distribution: Multiple workers share the load via NATS queues

### System Components

#### 1. API Server (`cmd/server`)

- Role: HTTP server (Fiber) that exposes REST endpoints
- Responsibilities:
  - User authentication with hybrid PQC JWT tokens
  - Receiving scan requests (wallet and TLS)
  - Publishing NATS messages for asynchronous processing
  - Reading results from PostgreSQL

#### 2. Worker (`cmd/worker`)

- Role: NATS consumer that processes scans
- Responsibilities:
  - Consuming NATS messages (wallet scan, TLS scan)
  - Processing scans (calling services)
  - Saving authenticated user results to PostgreSQL
  - Saving anonymous TLS scan results to Redis (with TTL)

#### 3. NATS

- Role: Messaging system for asynchronous communication
- Note: NATS is managed in [cafe-infra](https://github.com/kantika-tech/cafe-infra)
- Subjects:
  - `cafe.discovery.wallet.scan`: Wallet scan requests
  - `cafe.discovery.tls.scan`: TLS scan requests
- Queue: `cafe.workers` (enables load distribution between multiple workers)

#### 4. PostgreSQL

- Role: Primary database for authenticated users
- Note: PostgreSQL is managed in [cafe-infra](https://github.com/kantika-tech/cafe-infra)
- Stores:
  - User accounts and authentication data
  - Wallet scan results (authenticated users)
  - TLS scan results (authenticated users)
  - Subscription plans and user plans
- Advantages:
  - Better performance for complex queries
  - Native JSON support
  - ACID transactions
  - Horizontal scalability with read replicas

#### 5. Redis

- Role: Temporary storage for anonymous TLS scan results
- Note: Redis is managed in [cafe-infra](https://github.com/kantika-tech/cafe-infra)
- Stores:
  - Anonymous TLS scan results (with TTL expiration)
  - Results are isolated per anonymous session using token hash
- Use Case:
  - Allows unauthenticated users to scan TLS endpoints
  - Results are automatically expired after a configurable TTL
  - Provides fast key-value storage for temporary data
- Advantages:
  - Fast in-memory storage
  - Automatic expiration (TTL)
  - Low latency for read/write operations
  - Reduces load on PostgreSQL for temporary data

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
│   ├── metrics/           # Prometheus metrics registration
│   ├── service/           # Business logic
│   └── worker/            # NATS workers (wallet & TLS scanning)
├── pkg/
│   ├── evm/               # EVM client for blockchain interactions
│   ├── nats/              # NATS messaging client
│   ├── postgres/          # PostgreSQL database client
│   ├── pqc/               # Post-quantum cryptography (JWT, KEM)
│   ├── redis/             # Redis database client
│   └── tls/               # TLS scanner with PQC support
├── docs/                  # Documentation
│   ├── PQC_CERTIFICATES.md # PQC certificate generation guide
│   └── PQC_JWT.md         # PQC JWT implementation guide
├── scripts/               # Build and utility scripts
│   └── install_oqs_openssl_debian.sh  # OQS installation script
├── Dockerfile-oqs         # Base image with PQC facilities (build: oqs:dev)
├── Dockerfile-discovery-backend  # API server image (uses oqs:dev)
├── Dockerfile-discovery-worker   # Worker image (uses oqs:dev)
├── docker-compose.yml     # Docker Compose configuration to manage backend and nats worker
└── config.yaml            # Configuration file
```

### Dockerfile Structure

The project uses a multi-stage Docker build approach with a shared base image:

1. `Dockerfile-oqs`: 
   - Base image containing Open Quantum Safe (OQS) libraries
   - Includes liboqs, OpenSSL with oqs-provider, and Go runtime
   - Build command: `docker build -f Dockerfile-oqs -t oqs:dev .`
   - This image must be built before building the application images

2. `Dockerfile-discovery-backend`:
   - Builds the API server binary
   - Uses `oqs:dev` as base image
   - Creates a slim runtime image with only necessary dependencies
   - Output: `cafe-discovery-backend` service

3. `Dockerfile-discovery-worker`:
   - Builds the worker binary
   - Uses `oqs:dev` as base image
   - Creates a slim runtime image with only necessary dependencies
   - Output: `cafe-discovery-worker` service

Build Order:
1. First, build the base image: `docker build -f Dockerfile-oqs -t oqs:dev .`
2. Then, build the services: `docker-compose build` (or `docker-compose up --build`)

### Data Flow

#### Wallet Scan

```
Client HTTP → Discovery → NATS (publish) → Worker → Service → PostgreSQL
               backend           ↓
                         Immediate Response
```

1. Client sends a POST request to `/discovery/scan`
2. API Server validates the request and publishes a NATS message
3. Client receives an immediate response: `{"status": "processing"}`
4. A worker consumes the message and processes the scan
5. The result is saved to PostgreSQL

#### TLS Scan

Authenticated Users:
```
Client HTTP → Discovery  → NATS (publish) → Worker → Service → PostgreSQL
               backend            ↓
                         Immediate Response
```

1. Client sends a POST request to `/discovery/tls/scan`
2. API Server validates the request and publishes a NATS message to `cafe.discovery.tls.scan`
3. Client receives an immediate response: `{"message": "scan queued successfully", "status": "processing"}`
4. A worker consumes the message and processes the TLS scan (checks for PQC certificate support)
5. The result is saved to PostgreSQL for permanent access

Anonymous Users:
```
Client HTTP → Discovery → NATS (publish) → Worker → Service → Redis (with TTL)
               backend           ↓
                         Immediate Response
```

1. Client sends a POST request to `/discovery/tls/scan` (without authentication)
2. API Server validates the request, extracts token from Authorization header, and publishes a NATS message to `cafe.discovery.tls.scan.anonymous`
3. Client receives an immediate response: `{"message": "scan queued successfully", "status": "processing"}`
4. A worker consumes the message and processes the TLS scan (checks for PQC certificate support)
5. The result is saved to Redis with automatic expiration (TTL), isolated per anonymous session using token hash

Notes:
- Anonymous TLS scans are stored in Redis with automatic expiration (TTL)
- Results are isolated per anonymous session using token hash
- Authenticated TLS scans are stored in PostgreSQL for permanent access


### Deployment Considerations

#### Development

- Infrastructure services (PostgreSQL, NATS, Redis) are managed in [cafe-infra](https://github.com/kantika-tech/cafe-infra)
- Run API server and worker as separate processes

#### Production

This will be implemented later 

- Backend: Deploy multiple instances behind a load balancer
- Workers: Deploy multiple instances for horizontal scalability
- NATS: Use NATS JetStream for persistence and high availability
- PostgreSQL: Use a PostgreSQL cluster with read replicas

## Configuration

The application can be configured using either:
1. `config.yaml` file (recommended for Docker deployments)
2. Environment variables (override config.yaml values). This will ease the usage of k8s, later.

### Configuration File (`config.yaml`)

The `config.yaml` file contains all configuration settings. Here's the complete structure:

```yaml
server:
  host: "0.0.0.0"
  port: "8080"

# PostgreSQL configuration (for Docker, use service name 'postgres' or 'cafe-postgres')
POSTGRES_HOST: "cafe-postgres"
POSTGRES_PORT: "5432"
POSTGRES_DATABASE: "cafe"
POSTGRES_USER: "cafe"
POSTGRES_PASSWORD: "cafe"
POSTGRES_SSLMODE: "disable"

# NATS configuration (for Docker, use service name 'nats')
NATS_URL: "nats://nats:4222"

# Redis configuration (for Docker, use service name 'redis')
REDIS_URL: "redis://redis:6379"

# JWT configuration (required for authentication)
JWT_SECRET: "change-me-in-production"

# Moralis API configuration
MORALIS_API_KEY: ""
MORALIS_API_URL: "https://deep-index.moralis.io"

# Cloudflare Turnstile configuration (optional, uses dev keys by default)
TURNSTILE_SECRET_KEY: "1x0000000000000000000000000000000AA"
TURNSTILE_SITE_KEY: "1x00000000000000000AA"

# Logging
LOG_LEVEL: "info"

# CORS configuration
CORS_ALLOW_ORIGINS: "http://localhost:3000,http://localhost:3001,http://localhost:5173"
CORS_ALLOW_METHODS: "GET,POST,PUT,DELETE,OPTIONS"

blockchains:
  - name: ethereum-mainnet
    rpc: "https://ethereum-rpc.publicnode.com"
    moralis_chain_name: "eth"
  - name: polygon
    rpc: "https://polygon-bor-rpc.publicnode.com"
    moralis_chain_name: "polygon amoy"
  # ... more networks
```

Note: 
- Environment variables always override values from `config.yaml` 
- For Docker deployments, use service names (e.g., `postgres`, `nats`, `redis`) as hostnames
- The `CONFIG_PATH` environment variable can be used to specify a custom config file path (default: `config.yaml`)

## Prerequisites

- Go 1.24+ 
- Docker and Docker Compose
- Infrastructure services (PostgreSQL, NATS, Redis) - see [cafe-infra](../cafe-infra/README.md)
- Required for JWT authentication: Open Quantum Safe (OQS) library (liboqs) with ML-DSA-65 support
  - The service uses hybrid PQC JWT tokens (EdDSA + ML-DSA-65) for all authentication
  - See [Post-Quantum Cryptography](#post-quantum-cryptography-pqc) section for installation instructions

## Running the Service

Backend and worker are managed by Docker Compose

### Step 1: Build OQS Base Image

Before building the discovery services, you must first build the OQS base image:

```bash
cd cafe-discovery
docker build -f Dockerfile-oqs -t oqs:dev .
```

This creates the base image `oqs:dev` containing:
- Open Quantum Safe (OQS) library (liboqs)
- OpenSSL with oqs-provider
- Go runtime environment
- All necessary build tools and dependencies

Note: This step only needs to be done once, or when you need to update the OQS libraries.

### Step 2: Start Infrastructure Services

The infrastructure is managed in the `cafe-infra` [cafe-infra](https://github.com/kantika-tech/cafe-infra) repository.
Please, refer to it.

For information, the infrastructure is as follow:
- PostgreSQL on port `5432`
- NATS on ports `4222` (client) and `8222` (monitoring)
- Redis on port `6379`
- Observability stack:
  - Prometheus on port `9090` (metrics collection)
  - Grafana on port `3000` (dashboards and visualization)
  - Loki on port `3100` (log aggregation)
  - Tempo on port `3200` (distributed tracing)
  - OpenTelemetry Collector on ports `4317` (gRPC) and `4318` (HTTP)

#### Step 3: Start Cafe Discovery Services

From the `cafe-discovery` directory:

```bash
# Set required environment variables (optional - can also be set in config.yaml)
export JWT_SECRET=your-secret-key-here
export MORALIS_API_KEY=your_api_key_here

# Start both server and worker
docker-compose up -d

# Or start individually
docker-compose up -d cafe-discovery-backend
docker-compose up -d cafe-discovery-worker
```

The services will:
- Build the Docker images using `Dockerfile-discovery-backend` and `Dockerfile-discovery-worker`
- These Dockerfiles use `oqs:dev` as the base image
- Connect to the `cafe-infra_observability` network
- Use service names for connections (postgres, nats, redis) as configured in `config.yaml`
- Expose the API server on `http://localhost:8080`
- Load configuration from `/app/config.yaml` (mounted from `./config.yaml`)

Dockerfile Structure:
- `Dockerfile-oqs`: Base image with OQS libraries and build tools
- `Dockerfile-discovery-backend`: Builds the API server using `oqs:dev` as base
- `Dockerfile-discovery-worker`: Builds the worker using `oqs:dev` as base

Verify services are running:
```bash
# Check container status
docker-compose ps

# Health check
curl http://localhost:8080/health

# Metrics endpoint (Prometheus format)
curl http://localhost:8080/metrics

# View logs
docker-compose logs -f cafe-discovery-backend
docker-compose logs -f cafe-discovery-worker
```

Stop services:
```bash
docker-compose down
```

### Environment Variables

You can configure the application using environment variables. Environment variables always override values from `config.yaml`.

Configuration Priority:
1. Environment variables (highest priority)
2. `config.yaml` file values
3. Default values (lowest priority)

```bash
# Configuration file path (default: config.yaml)
# This tells Viper where to find the config file
export CONFIG_PATH=config.yaml

# Server configuration
export SERVER_HOST=0.0.0.0
export SERVER_PORT=8080

# Worker health check port
export WORKER_HEALTH_PORT=8081

# PostgreSQL configuration
# Use Docker service names
export POSTGRES_HOST=postgres
export POSTGRES_PORT=5432
export POSTGRES_DATABASE=cafe
export POSTGRES_USER=cafe
export POSTGRES_PASSWORD=cafe
export POSTGRES_SSLMODE=disable

# NATS configuration
export NATS_URL="nats://localhost:4222"  

# Redis configuration
exort REDIS_URL="redis://redis:6379"


# JWT configuration (required for authentication)
# Note: The service always uses hybrid PQC tokens (EdDSA + ML-DSA-65)
# To enforce security, there is no default value for JWT_SECRET: 
# It is not set here, so that it can not be copied/pasted
export JWT_SECRET=

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

# CORS configuration
export CORS_ALLOW_ORIGINS="http://localhost:3000,http://localhost:3001,http://localhost:5173"
export CORS_ALLOW_METHODS="GET,POST,PUT,DELETE,OPTIONS"
```

Using config.yaml vs Environment Variables:

- For Docker deployments: Use `config.yaml` with Docker service names (postgres, nats, redis)
- For production: Use environment variables or a secrets management system

### Démarrer en mode debug

Log levels permit to define how detailed should the log be.

Availabel log levels :
- `trace` : all logs
- `debug` : debug level and above
- `info` : default level and above
- `warn` : warnings and above
- `error` : errors and above
- `fatal` : fatal errors and above
- `panic` : panic level only

Example

```bash
# Terminal 1 - Serveur en mode debug
export LOG_LEVEL=debug
go run cmd/server/main.go

# Terminal 2 - Worker en mode debug
export LOG_LEVEL=debug
go run cmd/worker/main.go
```


### Verifying Everything Works

After starting all services, verify the complete setup:

```bash
# 1. Check infrastructure services
cd ../cafe-infra
docker-compose ps

# 2. Check API server
curl http://localhost:8080/health

# 3. Check metrics endpoint
curl http://localhost:8080/metrics | head -20

# 4. Check worker
curl http://localhost:8081/health

# 5. Check Prometheus is scraping (if observability stack is running)
curl http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | select(.labels.job=="cafe-discovery")'

# 6. Access Grafana (if observability stack is running)
# Open http://localhost:3000 in your browser
# Navigate to Dashboards to see CAFE Platform metrics
```

## Post-Quantum Cryptography (PQC)

The service implements post-quantum cryptography for both authentication (JWT) and TLS scanning capabilities.

### PQC JWT Authentication

The service uses hybrid PQC JWT tokens that combine:
- EdDSA (Ed25519): Classical signature algorithm for current security
- ML-DSA-65: Post-quantum signature algorithm for future quantum resistance

This hybrid approach provides security against both classical and quantum attacks. Classic HMAC tokens are not supported.

#### Prerequisites for PQC JWT

The PQC JWT implementation requires the Open Quantum Safe (OQS) library with ML-DSA-65 support.
This is why we provide the necessary docker files to build the correct environment.


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

The application always uses hybrid PQC tokens (EdDSA + ML-DSA-65). No policy configuration is needed - hybrid mode is always enabled. Classic HMAC tokens are not supported.

```bash
# JWT_SECRET is required but not used for signing (kept for API compatibility)
export JWT_SECRET=your-secret-key-here
```

Important: 
- The OQS library must be installed and available for the service to start
- If OQS is not found or ML-DSA-65 is not available, the service will fail to initialize the authentication service
- The service will log an error message listing available algorithms if ML-DSA-65 is not found
- See [docs/PQC_JWT.md](docs/PQC_JWT.md) for detailed installation instructions

#### Security Considerations

⚠️ Important Security Notes:

1. Key Storage: Server private keys are stored in memory. In production:
   - Consider using a Hardware Security Module (HSM)
   - Implement key rotation policies
   - Use secure key management services

2. Token Size: Hybrid tokens are larger than classic tokens (due to ML-DSA-65 signatures). Ensure your HTTP infrastructure can handle larger headers. Fiber buffger sizes are set to 10kb. Please see [fiber config](./internal/app/container.go), lines 124-129.

3. Performance: ML-DSA-65 signatures are slower than EdDSA. Consider:
   - Token caching strategies
   - Signature verification optimization
   - Load testing with hybrid tokens

For more details, see [docs/PQC_JWT.md](docs/PQC_JWT.md).

### PQC TLS Certificate Scanning

The service can scan TLS endpoints to detect post-quantum certificate support. You can generate PQC certificates for testing using the provided tools.

#### Generating PQC Certificates

Quick method with script:
```bash
./scripts/generate-pqc-cert.sh dilithium3 365 localhost
```

Available PQC Algorithms:

| Algorithm    | NIST Level | Usage                               |
| ------------ | ---------- | ----------------------------------- |
| `dilithium2` | 2          | Signatures, medium size             |
| `dilithium3` | 3          | Signatures, recommended             |
| `dilithium5` | 5          | Signatures, maximum security        |
| `falcon512`  | 1          | Signatures, compact                 |
| `falcon1024` | 5          | Signatures, high security           |
| `ED25519`    | -          | Quantum-resistant, widely supported |

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

⚠️ Important: PQC certificates have limitations:

1. Browser support: Browsers do not yet natively support PQC certificates
2. TLS 1.3: PQC support in TLS 1.3 is still experimental
3. Certificate authorities: No public CA currently issues PQC certificates
4. Interoperability: Few servers/clients currently support PQC certificates

For detailed instructions, see [docs/PQC_CERTIFICATES.md](docs/PQC_CERTIFICATES.md).

## API Endpoints

### Authentication

Most endpoints require JWT authentication. The service uses hybrid PQC JWT tokens (EdDSA + ML-DSA-65).

### POST /auth/signup

Register a new user account. Requires Cloudflare Turnstile verification.

Request:
```json
{
  "email": "user@example.com",
  "password": "securepassword",
  "confirm_password": "securepassword",
  "turnstile_token": "0.abcdefghijklmnopqrstuvwxyz..."
}
```

Note: The `turnstile_token` is generated by the Cloudflare Turnstile widget on the frontend. By default, the service uses Cloudflare's free development keys which always pass verification. The service will log a warning when using development keys. For production, configure production keys from your Cloudflare dashboard.

### POST /auth/signin

Sign in and receive a hybrid PQC JWT token. Requires Cloudflare Turnstile verification.

Request:
```json
{
  "email": "user@example.com",
  "password": "securepassword",
  "turnstile_token": "0.abcdefghijklmnopqrstuvwxyz..."
}
```

Note: The `turnstile_token` is generated by the Cloudflare Turnstile widget on the frontend. By default, the service uses Cloudflare's free development keys which always pass verification. The service will log a warning when using development keys. For production, configure production keys from your Cloudflare dashboard.

Response:
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

Request:
```json
{
  "address": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
}
```

Response:
```json
{
  "message": "scan queued successfully",
  "address": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
  "status": "processing"
}
```

### GET /discovery/scans

Returns paginated list of wallet scan results for the authenticated user.

Query Parameters:
- `limit` (optional): Number of results per page (default: 20)
- `offset` (optional): Number of results to skip (default: 0)

Response:
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

Request:
```json
{
  "url": "https://example.com"
}
```

Note: You can specify a custom port in the URL (e.g., `https://example.com:8443`). If no port is specified, port 443 is used by default for HTTPS URLs.

Response:
```json
{
  "message": "scan queued successfully",
  "endpoint": "https://example.com",
  "status": "processing"
}
```

### GET /discovery/tls/scans

Returns paginated list of TLS scan results for the authenticated user.

Query Parameters:
- `limit` (optional): Number of results per page (default: 20)
- `offset` (optional): Number of results to skip (default: 0)

### GET /discovery/rpcs

Returns the list of configured RPC endpoints. No authentication required.

Response:
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

Response:
```json
{
  "status": "ok",
  "app_name": "Cafe Discovery Service",
  "version": "1.0.0",
  "timestamp": "2025-01-15T10:30:00Z"
}
```

### GET /metrics

Prometheus metrics endpoint. Exposes metrics in Prometheus format for scraping. No authentication required.

Response:
Prometheus text format with all available metrics.

Example:
```
# HELP cafe_discovery_wallet_scans_total Total number of wallet scans performed
# TYPE cafe_discovery_wallet_scans_total counter
cafe_discovery_wallet_scans_total{scan_type="wallet"} 42

# HELP cafe_discovery_wallet_scan_duration_seconds Duration of wallet scans in seconds
# TYPE cafe_discovery_wallet_scan_duration_seconds histogram
cafe_discovery_wallet_scan_duration_seconds_bucket{scan_type="wallet",le="0.005"} 5
cafe_discovery_wallet_scan_duration_seconds_bucket{scan_type="wallet",le="0.01"} 10
...
```

Note: This endpoint is used by Prometheus (or other monitoring systems) to scrape metrics. The infrastructure stack in `cafe-infra` includes Prometheus configured to scrape this endpoint.

### Worker Health Check

The worker exposes a health check endpoint on port `8081` (configurable via `WORKER_HEALTH_PORT` environment variable).

Endpoint: `GET http://localhost:8081/health`

Response (healthy):
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

Response (degraded):
Returns HTTP 503 status code when NATS is disconnected or workers are not running.

## Testing

### 1. Register and Authenticate

Note: The signup and signin endpoints require a Cloudflare Turnstile token. By default, the service uses Cloudflare's free development keys which always pass verification. The service will log a warning when using development keys. For production, configure production keys from your Cloudflare dashboard.

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

Getting Turnstile Tokens: In a real application, the Turnstile token is generated by the Cloudflare Turnstile widget embedded in the frontend. For API testing, you can:
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

# Prometheus metrics (no auth required)
curl http://localhost:8080/metrics

# Worker health check (no auth required)
curl http://localhost:8081/health
```

## Risk Scoring

### Wallet Risk Score

The wallet risk score (0.0 to 1.0, where higher = higher risk) is calculated based on:

1. Base Risk: NIST Level 1 (ECDSA-secp256k1) contributes 0.5 base risk (quantum-broken)
2. Network Exposure: Each network where the key is exposed adds up to 0.4 risk
3. Transaction Count: More transactions increase risk (logarithmic scale):
   - 1-10 transactions: +0.05
   - 10-100 transactions: +0.15
   - 100+ transactions: +0.25

Key Exposure Detection: A wallet's public key is considered exposed if it has sent at least one transaction (nonce > 0), making it vulnerable to quantum attacks once quantum computers are available.

Account Type Detection:
- EOA: Externally Owned Account using ECDSA-secp256k1 (quantum-breakable)
- AA: Abstract Account compliant with ERC-4337 (potentially more flexible for PQC migration)

### TLS Risk Score

The TLS risk score (0.0 to 1.0, where higher = higher risk) is a comprehensive assessment of TLS endpoint security, considering both classical and post-quantum cryptography factors.

#### Calculation Method

The risk score uses a weighted combination of multiple security factors:

1. Base Risk (40% weight)
- Based on the worst NIST security level across all TLS components
- Uses detailed NIST levels (kex, sig, cipher, hkdf, session) if available from PQC scan
- Falls back to certificate and cipher suite levels if detailed levels are not available
- Formula: `risk = 1.0 - ((level - 1) / 4)`
  - NIST Level 1 (quantum-broken): 1.0 risk
  - NIST Level 3 (moderate): 0.5 risk
  - NIST Level 5 (PQC-ready): 0.0 risk

2. Cipher Suite Risk (25% weight)
- Evaluates the weakest cipher suite supported
- No cipher suites available: 1.0 risk (critical)
- Uses the same NIST level mapping as base risk

3. Protocol Version Risk (15% weight)
- TLS 1.3: 0.0 risk (most secure)
- TLS 1.2: 0.3 risk (acceptable but older)
- TLS 1.1 or older: 0.8 risk (deprecated, insecure)
- Unknown protocol: 0.5 risk (moderate)

4. Security Features (10% weight)
- Perfect Forward Secrecy (PFS) and OCSP Stapling reduce risk:
  - Both PFS and OCSP: 0.0 additional risk
  - PFS only: 0.2 additional risk
  - OCSP only: 0.3 additional risk
  - Neither: 0.5 additional risk

5. Post-Quantum Cryptography Readiness (10% weight)
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

- 0.0 - 0.1: Very Low Risk - Excellent TLS configuration with PQC support
- 0.1 - 0.4: Low Risk - Good TLS configuration, minor improvements possible
- 0.4 - 0.7: Medium Risk - Acceptable but should be improved
- 0.7 - 1.0: High Risk - Critical security issues, immediate action required

## Observability

The service exposes Prometheus-compatible metrics for monitoring and observability. Metrics are collected passively without affecting business logic.

### Metrics Endpoint

The service exposes a `/metrics` endpoint that provides metrics in Prometheus format:

```bash
curl http://localhost:8080/metrics
```

### Available Metrics

#### Wallet Scan Metrics

- `cafe_discovery_wallet_scans_total` (counter): Total number of wallet scans performed
  - Labels: `scan_type="wallet"`
- `cafe_discovery_wallet_scan_duration_seconds` (histogram): Duration of wallet scans in seconds
  - Labels: `scan_type="wallet"`
  - Buckets: Default Prometheus buckets (0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10)
- `cafe_discovery_wallet_scan_success_total` (counter): Total number of successful wallet scans
  - Labels: `scan_type="wallet"`, `result="success"`
- `cafe_discovery_wallet_scan_error_total` (counter): Total number of failed wallet scans
  - Labels: `scan_type="wallet"`, `result="failure"`

#### TLS Scan Metrics

- `cafe_discovery_tls_scans_total` (counter): Total number of TLS scans performed
  - Labels: `scan_type="tls"`
- `cafe_discovery_tls_scan_duration_seconds` (histogram): Duration of TLS scans in seconds
  - Labels: `scan_type="tls"`
  - Buckets: Default Prometheus buckets (0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10)
- `cafe_discovery_tls_scan_success_total` (counter): Total number of successful TLS scans
  - Labels: `scan_type="tls"`, `result="success"`
- `cafe_discovery_tls_scan_error_total` (counter): Total number of failed TLS scans
  - Labels: `scan_type="tls"`, `result="failure"`

### Metric Collection

Metrics are automatically recorded when:
- Wallet scans are performed via `ScanWallet()` service method
- TLS scans are performed via `ScanTLS()` service method

Both API-initiated scans and worker-processed scans are instrumented, as workers call the same service methods.

### Prometheus Configuration

The infrastructure stack in `cafe-infra` includes Prometheus configured to scrape the `/metrics` endpoint. 

For Docker deployments, Prometheus in `cafe-infra` is already configured to scrape the discovery service. The configuration uses the Docker service name:

```yaml
scrape_configs:
  - job_name: 'cafe-discovery'
    static_configs:
      - targets: ['cafe-discovery-backend:8080']  # Docker service name
    metrics_path: '/metrics'
    scrape_interval: 15s
```

For local development, if you're running the discovery service on `localhost:8080`, you may need to configure Prometheus to scrape it. Add the following to `cafe-infra/prometheus/prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'cafe-discovery'
    static_configs:
      - targets: ['host.docker.internal:8080']  # For Docker Compose on Mac/Windows
      # Or use: ['localhost:8080']  # For Linux or if Prometheus runs on host
    metrics_path: '/metrics'
    scrape_interval: 15s
```

Note: 
- If Prometheus runs in Docker (via `cafe-infra`), use `host.docker.internal:8080` on Mac/Windows to access the host machine
- On Linux, you may need to use `172.17.0.1:8080` or configure Docker networking
- For production deployments, use the appropriate service discovery mechanism (DNS, Kubernetes service discovery, etc.)

After updating the Prometheus configuration, restart Prometheus:
```bash
cd ../cafe-infra
docker-compose restart prometheus
```

Verify Prometheus is scraping the service:
```bash
# Check targets in Prometheus UI
open http://localhost:9090/targets

# Or via API
curl http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | select(.labels.job=="cafe-discovery")'
```

### Metric Design Principles

- Passive instrumentation: Metrics are collected without modifying business logic
- Low cardinality: Labels are carefully chosen to avoid high cardinality (no user IDs, addresses, or endpoints in labels)
- Factual metrics: Metrics record counts, durations, and errors - no business decisions or classifications
- Long-term monitoring: Metrics are suitable for platform monitoring and audit purposes

For more information about the observability stack, see the [cafe-infra](https://github.com/kantika-tech/cafe-infra).

## Background Processing

The application uses NATS for asynchronous message processing:

- Wallet Scans: When a scan is requested via the API, it's queued in NATS and processed by the wallet worker
- TLS Scans: TLS endpoint scans are also queued and processed by the TLS worker
- Scalability: Multiple worker instances can be run to process messages in parallel

## Development Tools

### Public Key Recovery Utility (`cmd/cli/publickey`)

A development utility for testing public key recovery from blockchain transactions. This tool demonstrates how the service recovers public keys from transaction data.

Usage:

```bash
# Set required environment variable
export MORALIS_API_KEY=your_api_key_here

# Run the utility
go run cmd/cli/publickey/getpublickey.go
```

Note: This utility requires a valid Moralis API key to fetch transaction data. The API key must be provided via the `MORALIS_API_KEY` environment variable.

## Security Notes

⚠️ Important: Never commit API keys or sensitive credentials to version control. Always use environment variables or secure configuration management:

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

# Stop Docker services (from cafe-infra)
cd ../cafe-infra
docker-compose down
```

To stop and remove volumes (clears database):

```bash
cd ../cafe-infra
docker-compose down -v
```

## Additional Resources

- [Post-Quantum JWT Documentation](docs/PQC_JWT.md) - Detailed guide on PQC JWT implementation
- [PQC Certificate Generation Guide](docs/PQC_CERTIFICATES.md) - Guide for generating and testing PQC TLS certificates
- [Open Quantum Safe](https://openquantumsafe.org/) - Official OQS project
- [NIST PQC Standards](https://csrc.nist.gov/projects/post-quantum-cryptography) - NIST post-quantum cryptography standards
