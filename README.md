# Cafe Discovery Service

A Discovery service for identifying cryptographic exposures and quantum vulnerabilities on the Ethereum network and related infrastructure.

## Features

- **Wallet Scanning**: Scan wallets across multiple EVM-compatible networks
- **Key Exposure Detection**: Detect whether a wallet's public key has been revealed on-chain
- **Account Type Detection**: Determine if an address is an EOA (Externally Owned Account) or AA (Abstract Account/ERC-4337)
- **Risk Assessment**: Calculate risk scores based on exposure across networks
- **Quantum Security Level**: Assess NIST quantum-security levels

## Architecture

The application uses an asynchronous message-based architecture with NATS and PostgreSQL to improve scalability and performance.

### System Components

#### 1. API Server (`cmd/server`)

- **Role**: HTTP server (Fiber) that exposes REST endpoints
- **Responsibilities**:
  - User authentication
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
│   └── postgres/          # PostgreSQL database client
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

# PostgreSQL configuration (defaults match docker-compose.yml)
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_DATABASE=cafe
export POSTGRES_USER=cafe
export POSTGRES_PASSWORD=cafe
export POSTGRES_SSLMODE=disable

# NATS configuration
export NATS_URL=nats://localhost:4222

# Moralis API (required for wallet scanning features)
# Get your API key from https://moralis.io
export MORALIS_API_KEY=your_api_key_here
export MORALIS_API_URL=https://deep-index.moralis.io

# Logging
export LOG_LEVEL=info  # Options: trace, debug, info, warn, error, fatal, panic
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

## API Endpoints

### Authentication

Most endpoints require JWT authentication. See the authentication endpoints below.

### POST /auth/signup

Register a new user account.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "securepassword"
}
```

### POST /auth/signin

Sign in and receive a JWT token.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "securepassword"
}
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "uuid",
    "email": "user@example.com"
  }
}
```

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

```bash
# Register a new user
curl -X POST http://localhost:8080/auth/signup \
  -H "Content-Type: application/json" \
  -d '{"email": "test@example.com", "password": "testpassword123"}'

# Sign in and get JWT token
TOKEN=$(curl -s -X POST http://localhost:8080/auth/signin \
  -H "Content-Type: application/json" \
  -d '{"email": "test@example.com", "password": "testpassword123"}' \
  | jq -r '.token')

echo "Token: $TOKEN"
```

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
# Queue a TLS scan
curl -X POST http://localhost:8080/discovery/tls/scan \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"url": "https://example.com"}'

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
```

## Risk Scoring

The risk score (0.0 to 1.0, where higher = higher risk) is calculated based on:

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

