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
- **CycloneDX v1.7 CBOMs**: All scan results are returned as CycloneDX v1.7-based Cryptographic Bill of Materials (CBOMs) with NIST SP 800-57 key states and lifecycle metadata. Note: CAFE extends CycloneDX with custom fields (e.g., `nist_level`, `quantum_vulnerable`, `key_exposed`) that are not part of the standard specification.

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
│   └── install_oqs_openssl_debian.sh  # OQS installation script (legacy, see cafe-infra/oqs)
├── Dockerfile-discovery-backend  # API server image (uses oqs:dev)
├── Dockerfile-discovery-worker   # Worker image (uses oqs:dev)
├── docker-compose.yml     # Docker Compose configuration to manage backend and nats worker
└── config.yaml            # Configuration file
```

### Dockerfile Structure

The project uses a multi-stage Docker build approach with a shared base image:

1. OQS Base Image (managed in [cafe-infra](https://github.com/kantika-tech/cafe-infra)):
   - Build instructions: See [cafe-infra/oqs/README.md](https://github.com/kantika-tech/cafe-infra/oqs/README.md)
   - Generated images: `cafe-oqs:build` and `cafe-oqs:runtime`

2. `Dockerfile-discovery-backend`:
   - Builds the API server binary
   - Uses `oqs:dev` as base image (must be built from cafe-infra)
   - Creates a slim runtime image with only necessary dependencies
   - Output: `cafe-discovery-backend` service

3. `Dockerfile-discovery-worker`:
   - Builds the worker binary
   - Uses `oqs:dev` as base image (must be built from cafe-infra)
   - Creates a slim runtime image with only necessary dependencies
   - Output: `cafe-discovery-worker` service

Build Order:
1. First, build the OQS base image from `cafe-infra` (see [Step 1: Build OQS Base Image](#step-1-build-oqs-base-image))
2. Then, build the services using Docker Compose: `docker compose build` (or `docker compose up --build`)

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

## CI/CD and Release Process

This project implements a strict, security-focused CI/CD pipeline that enforces quality gates and ensures all published Docker images are secure and traceable.

### Overview

The project produces **two Docker images**:
- `oleglod/cafe-discovery-backend`: API server image
- `oleglod/cafe-discovery-worker`: Background worker image

Both images are built from their respective Dockerfiles (`Dockerfile-discovery-backend` and `Dockerfile-discovery-worker`) and are always released together as a unit.

### Pipeline Separation

The CI/CD pipeline is strictly separated into two distinct workflows:

#### 1. Pull Request CI (`.github/workflows/ci.yml`)

**Trigger**: Pull requests targeting `main`

**Purpose**: Quality assurance and security validation before code is merged.

**Steps** (executed in `oleglod/cafe-oqs:build` container):
1. Checkout repository
2. Download Go dependencies (`go mod download`)
3. Run linter (`golangci-lint run ./...`)
4. Run unit tests (`go test ./...`)
5. Run vulnerability scanning (`govulncheck ./...`)

**Security Gates**:
- All steps must pass for the PR to be mergeable
- `govulncheck` failures block PR merges
- No Docker images are built or published

**Important**: This workflow does NOT build or publish Docker images. It only validates code quality and security.

##### Running CI Locally

You can run the same CI checks locally before creating a pull request. This helps catch issues early and ensures your PR will pass CI.

**Prerequisites:**
- Docker and Docker Compose installed
- OQS base image built (see [Step 1: Build OQS Base Image](#step-1-build-oqs-base-image))

**Method 1: Using Docker Compose (Recommended)**

The `docker-compose.yml` file includes CI service definitions that build the CI images:

```bash
# Build both CI images
docker compose build cafe-discovery-backend-ci
docker compose build cafe-discovery-worker-ci

# Run CI checks for backend
docker compose run --rm cafe-discovery-backend-ci

# Run CI checks for worker
docker compose run --rm cafe-discovery-worker-ci
```

**Method 2: Using Docker Directly**

You can also build and run the CI images directly with Docker:

```bash
# Build backend CI image
docker build \
  --target ci \
  -f Dockerfile-discovery-backend \
  -t cafe-discovery-backend:ci .

# Run backend CI checks
docker run --rm cafe-discovery-backend:ci

# Build worker CI image
docker build \
  --target ci \
  -f Dockerfile-discovery-worker \
  -t cafe-discovery-worker:ci .

# Run worker CI checks
docker run --rm cafe-discovery-worker:ci
```

**Method 3: Running Individual Checks Locally (Without Docker)**

If you have Go, `golangci-lint`, and `govulncheck` installed locally:

```bash
# Download dependencies
go mod download

# Run linter
golangci-lint run ./...

# Run tests
go test ./...

# Run vulnerability scanner
govulncheck ./...
```

**What the CI Checks Do:**

1. **`go mod download`**: Downloads all Go module dependencies
2. **`golangci-lint run ./...`**: Runs static analysis and linting on all Go files
   - Checks code style, potential bugs, security issues
   - Uses configuration from `.golangci.yml` or `.golangci.yml-strict`
   - Timeout: 5 minutes (configurable)
3. **`go test ./...`**: Runs all unit tests
   - Executes tests in all packages
   - Reports test coverage and failures
4. **`govulncheck ./...`**: Scans for known vulnerabilities
   - Checks against Go vulnerability database
   - Reports any known security issues in dependencies

**Troubleshooting:**

- **Build fails with "oqs:dev not found"**: Make sure you've built the OQS base image (see [Step 1: Build OQS Base Image](#step-1-build-oqs-base-image))
- **Linter timeout**: Increase timeout in `.golangci.yml` or run with `--timeout=10m`
- **Tests fail**: Check that all dependencies are available and tests are passing locally
- **govulncheck fails**: Update dependencies with `go get -u ./...` and `go mod tidy`

**CI Image Details:**

The CI images (`ci` target) include:
- Go 1.25.5 runtime (required to fix GO-2025-4175 and GO-2025-4155 vulnerabilities)
- Open Quantum Safe (OQS) libraries
- `golangci-lint` v2.8.0
- `govulncheck` (latest)
- All project dependencies

The CI images are based on the `builder` stage, which includes the full build environment. They execute the CI checks as the default command when run.

#### 2. Docker Release Pipeline (`.github/workflows/docker-release.yml`)

**Trigger**: Push of Git tags matching `v*.*.*` (e.g., `v1.2.3`)

**Purpose**: Build, scan, and publish production Docker images.

**Process**:
1. **Extract version information** from the Git tag:
   - Full version: `vX.Y.Z` (from tag)
   - Minor version: `vX.Y` (derived)
   - Short commit SHA (for traceability)

2. **Build both images** (multi-arch: `linux/amd64`, `linux/arm64`):
   - `oleglod/cafe-discovery-backend`
   - `oleglod/cafe-discovery-worker`

3. **Security scanning** (Docker Scout):
   - Scan both images for critical and high-severity vulnerabilities
   - If **either** image fails the scan, the entire job fails
   - **No images are published** if scanning fails

4. **Publish images** (only if both scans pass):
   - Both images are tagged identically:
     - `vX.Y.Z` (full version from tag)
     - `vX.Y` (minor version)
     - `sha-<short-sha>` (commit SHA for traceability)
     - `build` (latest build tag)
   - All tags are multi-arch manifests

**Security Gates**:
- Docker Scout vulnerability scanning blocks publication
- Both images must pass scanning; if one fails, nothing is published
- All published images are traceable to a Git tag and commit SHA

### Release Procedure

Releases are **manual and explicit**. The CI system never creates tags automatically.

**Step-by-step release process**:

1. **Merge PR to `main`**:
   - Ensure the PR has passed all CI checks (lint, tests, govulncheck)
   - Merge the PR into `main`

2. **Create Git tag** (manually, after merge):
   ```bash
   git checkout main
   git pull origin main
   git tag v1.2.3
   git push origin v1.2.3
   ```

3. **CI automatically**:
   - Detects the tag push
   - Builds both Docker images (multi-arch)
   - Scans both images with Docker Scout
   - If scans pass, publishes both images with all tags
   - If scans fail, publishes nothing

### Security and Auditability

**Versioning Policy**:
- Versions are **never auto-generated**
- All versions come from manually created Git tags
- Format: `vX.Y.Z` (semantic versioning)

**Traceability**:
- Every published image is tagged with:
  - Git tag (`vX.Y.Z`)
  - Commit SHA (`sha-<short-sha>`)
- Images can be traced back to exact source code commits

**Security Enforcement**:
- `govulncheck` blocks PR merges (prevents vulnerable code from entering `main`)
- Docker Scout blocks image publication (prevents vulnerable images from being published)
- Both images are always released together (ensures consistency)

**Failure Handling**:
- If either image fails scanning, **nothing is published**
- This ensures both services are always at the same security level
- Failed releases require fixing vulnerabilities and re-tagging

### Image Tags

Both `oleglod/cafe-discovery-backend` and `oleglod/cafe-discovery-worker` receive identical tags:

- `v1.2.3`: Full semantic version (from Git tag)
- `v1.2`: Minor version (for compatibility)
- `sha-abc1234`: Commit SHA (for traceability)
- `build`: Latest build (points to most recent release)

All tags are multi-arch manifests supporting `linux/amd64` and `linux/arm64`.

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

# PostgreSQL configuration (for Docker, use service name 'postgres')
POSTGRES_HOST: "postgres"
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

### Development Mode (Local, Outside Docker)

To run the backend locally for debugging:

1. **Create a local configuration file** (copy from `config.yaml` and modify for localhost):

```bash
# Create config.local.yaml with localhost values
cp config.yaml config.local.yaml
# Edit config.local.yaml to use localhost instead of Docker service names
```

Or use the provided `config.local.yaml` template (already created with localhost values).

2. **Ensure infrastructure services are running** (PostgreSQL, NATS, Redis):
   - Either run them via Docker Compose from `cafe-infra`
   - Or run them locally on your machine

3. **Set environment variables** (optional, can override config file values):

```bash
export CONFIG_PATH=config.local.yaml
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export NATS_URL="nats://localhost:4222"
export REDIS_URL="redis://localhost:6379"
export JWT_SECRET="your-secret-key-here"
export MORALIS_API_KEY="your-api-key-here"
```

4. **Run the server**:

```bash
CONFIG_PATH=config.local.yaml go run cmd/server/main.go
```

**Note**: The `CONFIG_PATH` must point to a YAML file (not `.env`). The YAML file contains both:
- Viper configuration (POSTGRES_HOST, NATS_URL, etc.)
- Chain configuration (blockchains section)

You can also use environment variables to override any value from the config file (environment variables have highest priority).

### Docker Compose Mode

Backend and worker are managed by Docker Compose

### Step 1: Build OQS Base Image

Before building the discovery services, you must first build the OQS base image from the `cafe-infra` repository:

```bash
# Navigate to cafe-infra
cd ../cafe-infra/oqs

# Build the OQS images (builds both cafe-oqs:build and cafe-oqs:runtime)
./build.sh

# Tag the build image as oqs:dev for compatibility with cafe-discovery Dockerfiles
docker tag cafe-oqs:build oqs:dev

# Return to cafe-discovery
cd ../../cafe-discovery
```

This creates the base images containing:
- `cafe-oqs:build`: Build environment with Open Quantum Safe (OQS) library (liboqs), OpenSSL with oqs-provider, and Go runtime
- `cafe-oqs:runtime`: Minimal runtime image with OQS support
- `oqs:dev`: Tagged alias of `cafe-oqs:build` for compatibility

**Note**: 
- The OQS Dockerfile has been moved from `cafe-discovery` to `cafe-infra/oqs`
- This step only needs to be done once, or when you need to update the OQS libraries
- For detailed OQS build instructions, see [cafe-infra/oqs/README.md](../cafe-infra/oqs/README.md)

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

#### Step 3: Build and Start Cafe Discovery Services

From the `cafe-discovery` directory:

```bash
# Set required environment variables (optional - can also be set in config.yaml)
export JWT_SECRET=your-secret-key-here
export MORALIS_API_KEY=your_api_key_here

# Build the images using Docker Compose
docker compose build

# Start both server and worker
docker compose up -d

# Or build and start in one command
docker compose up -d --build

# Or start individually
docker compose up -d cafe-discovery-backend
docker compose up -d cafe-discovery-worker
```

**Important**: The images are built using Docker Compose, which automatically:
- Builds the Docker images using `Dockerfile-discovery-backend` and `Dockerfile-discovery-worker`
- These Dockerfiles use `oqs:dev` as the base image (must be built from `cafe-infra/oqs` in Step 1)
- Manages the build context and dependencies

**Docker Compose Services:**

The `docker-compose.yml` file defines four services:

1. **`cafe-discovery-backend`** (runtime):
   - API server running on port `8080`
   - Uses `runtime` target from `Dockerfile-discovery-backend`
   - Health check: `curl http://localhost:8080/health` (every 30s)
   - Restart policy: `unless-stopped`

2. **`cafe-discovery-worker`** (runtime):
   - Background worker processing NATS messages
   - Health check endpoint on port `8081` (configurable via `WORKER_HEALTH_PORT`)
   - Health check: `wget http://localhost:8081/health` (every 30s)
   - Restart policy: `unless-stopped`

3. **`cafe-discovery-backend-ci`** (build-only):
   - CI build target for backend (not started by default)
   - Uses `ci` target from `Dockerfile-discovery-backend`
   - Used for CI/CD pipelines

4. **`cafe-discovery-worker-ci`** (build-only):
   - CI build target for worker (not started by default)
   - Uses `ci` target from `Dockerfile-discovery-worker`
   - Used for CI/CD pipelines

**Configuration:**

The services are configured with:
- **Network**: Connects to external network `cafe-infra_observability` (must exist from `cafe-infra`)
- **Volumes**: Mounts `./config.yaml` to `/app/config.yaml` (read-only)
- **Environment Variables**: Supports environment variable overrides with defaults:
  - `JWT_SECRET` (default: `change-me-in-production`)
  - `MORALIS_API_KEY` (required, no default)
  - `POSTGRES_USER` (default: `cafe`)
  - `POSTGRES_PASSWORD` (default: `cafe`)
  - `LOG_LEVEL` (default: `debug` for backend, `info` for worker)
  - `TURNSTILE_SECRET_KEY` and `TURNSTILE_SITE_KEY` (default: dev keys)
- **Service Discovery**: Uses Docker service names (postgres, nats, redis) from `cafe-infra`
- **Health Checks**: Both services include health check configurations for monitoring

**Dockerfile Structure:**
- **OQS Base Image**: Managed in [cafe-infra/oqs](../cafe-infra/oqs/) - builds `cafe-oqs:build` and `cafe-oqs:runtime`, tagged as `oqs:dev` for compatibility
- `Dockerfile-discovery-backend`: Builds the API server using `oqs:dev` as base
  - `runtime` target: Production-ready server image
  - `ci` target: CI/CD image with linting and testing tools
- `Dockerfile-discovery-worker`: Builds the worker using `oqs:dev` as base
  - `runtime` target: Production-ready worker image
  - `ci` target: CI/CD image with linting and testing tools

**Verify services are running:**
```bash
# Check container status
docker compose ps

# Health check (backend)
curl http://localhost:8080/health

# Health check (worker)
curl http://localhost:8081/health

# Metrics endpoint (Prometheus format)
curl http://localhost:8080/metrics

# View logs
docker compose logs -f cafe-discovery-backend
docker compose logs -f cafe-discovery-worker
```

**Stop services:**
```bash
docker compose down
```

#### Step 3-bis: Start Services Independently (Advanced)

If you prefer to run services independently without Docker Compose, you can use `docker run` directly:

**Start the backend:**
```bash
docker run --network cafe-infra_observability --rm \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  -e CONFIG_PATH=/app/config.yaml \
  -e LOG_LEVEL=debug \
  -e JWT_SECRET=your-secret-key-here \
  -e MORALIS_API_KEY=your-api-key-here \
  -e POSTGRES_HOST=postgres \
  -e POSTGRES_PORT=5432 \
  -e POSTGRES_DATABASE=cafe \
  -e POSTGRES_USER=cafe \
  -e POSTGRES_PASSWORD=cafe \
  -e NATS_URL=nats://nats:4222 \
  -e REDIS_URL=redis://redis:6379 \
  cafe-discovery-backend:latest
```

**Start the worker:**
```bash
docker run --network cafe-infra_observability --rm \
  -p 8081:8081 \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  -e CONFIG_PATH=/app/config.yaml \
  -e WORKER_HEALTH_PORT=8081 \
  -e LOG_LEVEL=info \
  -e JWT_SECRET=your-secret-key-here \
  -e MORALIS_API_KEY=your-api-key-here \
  -e POSTGRES_HOST=postgres \
  -e POSTGRES_PORT=5432 \
  -e POSTGRES_DATABASE=cafe \
  -e POSTGRES_USER=cafe \
  -e POSTGRES_PASSWORD=cafe \
  -e NATS_URL=nats://nats:4222 \
  -e REDIS_URL=redis://redis:6379 \
  cafe-discovery-worker:latest
```

**Note:** 
- Replace `cafe-discovery-backend:latest` and `cafe-discovery-worker:latest` with the actual image names/tags you built
- The network `cafe-infra_observability` must exist (created by `cafe-infra`)
- All environment variables can be overridden as needed
- Using Docker Compose (Step 3) is recommended for easier management

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
export REDIS_URL="redis://redis:6379"


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
docker compose ps

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

#### Understanding NIST Security Levels and Risk Scores

The TLS scanning service evaluates endpoints using **NIST quantum-security levels** and calculates a comprehensive **risk score** to assess overall security posture.

##### NIST Security Levels

NIST levels range from 1 (quantum-broken) to 5 (PQC-ready):

- **Level 1**: Quantum-broken - Vulnerable to quantum computer attacks (e.g., RSA, ECDSA)
- **Level 2**: Low quantum resistance
- **Level 3**: Moderate quantum resistance (e.g., Ed25519, TLS 1.3 with classical crypto)
- **Level 4**: High quantum resistance
- **Level 5**: PQC-ready - Post-quantum cryptography ready (e.g., ML-KEM, Dilithium)

The service evaluates multiple components:
- **Certificate**: Signature algorithm and public key algorithm of the X.509 certificate
- **Key Exchange (KEX)**: Key exchange method used during TLS handshake (e.g., X25519, ML-KEM, ECDHE)
- **Signature**: Signature algorithm used during TLS handshake (may differ from certificate signature)
- **Cipher**: Encryption cipher suite negotiated (e.g., TLS_AES_256_GCM_SHA384)
- **HKDF**: Key derivation function used for key derivation
- **Session**: Session management and resumption mechanisms

**Important Distinction:**
- **Certificate NIST Level**: Based on the certificate's signature algorithm (e.g., ECDSA-SHA256 = Level 1)
- **Detailed NIST Levels**: Based on the actual TLS handshake and protocol components
  - These are **independent** of the certificate (except Signature which may use the certificate)
  - Key Exchange, Cipher, HKDF, and Session are **not related** to the certificate
  - They reflect the actual cryptographic algorithms used during the TLS connection

##### Overall NIST Level Calculation

The **overall NIST level** displayed represents the **worst (minimum) level** across all components:

```
Overall NIST Level = min(certificate, kex, sig, cipher, hkdf, session)
```

**Why the minimum?** Security is only as strong as the weakest component. If the certificate is Level 1 but key exchange is Level 5, an attacker can still exploit the weak certificate.

**Example:**
- Certificate: Level 1 (ECDSA-SHA384 - quantum-vulnerable)
- Key Exchange: Level 3 (X25519MLKEM768 - hybrid PQC)
- Signature: Level 3
- Cipher: Level 5
- HKDF: Level 3
- Session: Level 5

**Overall NIST Level: 1** (because the certificate is the weakest link)

##### Risk Score Calculation

The **risk score** (0.0 to 1.0, where 1.0 = highest risk) uses a **weighted average** approach to better reflect overall security:

**Components:**
1. **Base Risk (40% weight)**: Uses a weighted average of all NIST levels
   - Critical components (certificate, signature) have 2x weight
   - Other components (kex, cipher, hkdf, session) have 1x weight
   - Blends worst level (30%) with average (70%) to reflect that one weak component matters but doesn't dominate

2. **Cipher Suite Risk (25% weight)**: Based on weakest cipher suite

3. **Protocol Risk (15% weight)**: TLS 1.3 = 0.0, TLS 1.2 = 0.3, older = 0.8

4. **Security Features (10% weight)**: PFS and OCSP stapling reduce risk

5. **PQC Readiness (10% weight)**: PQC support significantly reduces quantum risk

**Why weighted average?** While the overall NIST level correctly identifies the weakest component, the risk score reflects that having strong components (Level 3-5) in most areas reduces overall risk compared to having everything at Level 1.

**Example (same endpoint as above):**
- Certificate: Level 1
- Other components: Level 3-5
- Protocol: TLS 1.3
- PFS: Enabled
- PQC Mode: Hybrid

**Risk Score: ~0.35 (35%)** - Moderate risk due to weak certificate, but mitigated by strong other components and PQC support.

**Interpretation:**
- **0.0-0.2 (0-20%)**: Low risk - Well configured, PQC-ready
- **0.2-0.4 (20-40%)**: Moderate risk - Mostly secure with some weaknesses
- **0.4-0.7 (40-70%)**: High risk - Significant security concerns
- **0.7-1.0 (70-100%)**: Critical risk - Immediate action required

##### Understanding "N/A" in Detailed NIST Levels

When you see **"N/A"** or **"Estimated"** for Detailed NIST Security Levels, it means:

1. **PQC Scan Not Available**: The endpoint does not support post-quantum cryptography extensions, or the PQC scan (OQS/OpenSSL) could not be performed.

2. **Estimated Values**: The frontend will display estimated levels based on:
   - **Signature**: Uses the certificate's NIST level
   - **Cipher**: Uses the worst cipher suite's NIST level
   - **Key Exchange**: Estimated based on protocol version (TLS 1.3 = Level 3, older = Level 1) and PQC readiness
   - **HKDF/Session**: Estimated based on protocol version (TLS 1.3 = Level 3)

3. **Why This Happens**: 
   - Most endpoints don't yet support PQC extensions
   - The detailed component-level analysis requires PQC-capable scanning
   - Classical TLS scans only provide certificate and cipher suite information

**Example Scenario:**
```
NIST Security Level: Level 1 (from certificate)
Detailed NIST Levels: 
  - Key Exchange: Level 3 (X25519 - TLS 1.3)
  - Signature: Level 3 (ECDSA from certificate)
  - Cipher: Level 5 (TLS_AES_256_GCM_SHA384)
  - HKDF: Level 3 (TLS 1.3 key derivation)
  - Session: Level 5 (TLS 1.3 session management)
Risk Score: 66%

Explanation:
- Certificate is Level 1 (ECDSA-SHA256 - quantum-vulnerable)
- Key Exchange is Level 3 (X25519 - independent of certificate)
- Cipher suite is Level 5 (TLS_AES_256_GCM_SHA384 - independent of certificate)
- Protocol is TLS 1.3 (good)
- OCSP Stapling enabled (good)
- But certificate weakness dominates, resulting in:
  - Overall NIST Level = 1 (worst component = certificate)
  - Risk Score = 66% (weighted average, certificate has high weight but other components reduce risk)
```

**Key Point:** The detailed NIST levels (KEX, Cipher, HKDF, Session) are **NOT related to the certificate**. They reflect the actual TLS protocol components used during the connection. Only the **Signature** level may be related to the certificate if the certificate's signature algorithm is used during the handshake.

**To Get Accurate Detailed Levels:**
- The endpoint must support post-quantum cryptography extensions
- The server must be configured with PQC algorithms (ML-KEM, Dilithium, etc.)
- The scan must successfully connect using OQS/OpenSSL with PQC support

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

Unified scan endpoint that automatically detects whether the request is for a wallet scan or TLS endpoint scan. Requires authentication. The scan is processed asynchronously via NATS.

**For Wallet Scans:**
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
  "type": "wallet",
  "status": "processing"
}
```

**For TLS Endpoint Scans:**
Request:
```json
{
  "url": "https://example.com"
}
```

Response:
```json
{
  "message": "scan queued successfully",
  "endpoint": "https://example.com",
  "type": "tls",
  "status": "processing"
}
```

Note: The endpoint automatically detects the scan type based on the provided field (`address` for wallets, `url` for TLS endpoints). You cannot specify both fields in the same request.

### GET /discovery/scans

Returns paginated list of CBOMs (Cryptographic Bill of Materials) for wallet scans for the authenticated user.

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
      "networks": ["ethereum-mainnet", "polygon"],
      "scanned_at": "2025-01-15T10:30:00Z",
      "cbom": {
        "bomFormat": "CycloneDX",
        "specVersion": "1.7",
        "version": 1,
        "metadata": {
          "timestamp": "2025-01-15T10:30:00Z"
        },
        "type": "wallet",
        "components": [
          {
            "type": "cryptographic-primitive",
            "name": "ECDSA-secp256k1",
            "nist_level": 1,
            "quantum_vulnerable": true,
            "key_exposed": true,
            "assetType": "related-crypto-material",
            "state": "active",
            "customStates": [
              {
                "name": "quantum-vulnerable",
                "description": "Key relies on cryptographic algorithms considered vulnerable to future cryptographic quantum attacks"
              }
            ]
          }
        ]
      }
    }
  ],
  "total": 1,
  "limit": 20,
  "offset": 0,
  "count": 1
}
```

Note: Each result is a CBOM (Cryptographic Bill of Materials) that includes both the scan data and the structured `cbom.components` array describing the cryptographic primitives.

### GET /discovery/cbom/*

Returns a CBOM (Cryptographic Bill of Materials) JSON record for a wallet address or TLS endpoint. Automatically detects the type based on the parameter format. Requires authentication.

Path Parameters:
- `*`: Either:
  - Ethereum wallet address (EOA) in hex format (e.g., `0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb`)
  - TLS endpoint URL (e.g., `https://example.com` or URL-encoded `https%3A%2F%2Fexample.com`)

**For Wallet Addresses:**

Response:
```json
{
  "address": "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb",
  "type": "EOA",
  "algorithm": "ECDSA-secp256k1",
  "nist_level": 1,
  "key_exposed": true,
  "risk_score": 0.85,
  "first_seen": "2025-03-10T15:22:00Z",
  "last_seen": "2025-10-16T08:10:00Z",
  "networks": ["ethereum-mainnet", "polygon"],
  "scanned_at": "2025-01-15T10:30:00Z",
  "cbom": {
    "bomFormat": "CycloneDX",
    "specVersion": "1.7",
    "version": 1,
    "metadata": {
      "timestamp": "2025-01-15T10:30:00Z"
    },
    "type": "wallet",
    "components": [
      {
        "type": "cryptographic-primitive",
        "name": "ECDSA-secp256k1",
        "nist_level": 1,
        "quantum_vulnerable": true,
        "key_exposed": true,
        "assetType": "related-crypto-material",
        "state": "active",
        "customStates": [
          {
            "name": "quantum-vulnerable",
            "description": "Key relies on cryptographic algorithms considered vulnerable to future cryptographic quantum attacks"
          }
        ]
      }
    ]
  }
}
```

**For TLS Endpoints:**

Response:
```json
{
  "url": "https://example.com",
  "host": "example.com",
  "port": 443,
  "protocol": "TLS 1.3",
  "nist_level": 1,
  "risk_score": 0.75,
  "pqc_risk": "critical",
  "pqc_mode": "classical",
  "supported_pqc": [],
  "recommendations": ["Upgrade to PQC certificates"],
  "scanned_at": "2025-01-15T10:30:00Z",
  "certificate": {
    "subject": "CN=example.com",
    "issuer": "CN=Let's Encrypt",
    "signature_algorithm": "ECDSA-secp256r1",
    "nist_level": 1,
    "is_pqc_ready": false
  },
  "cipher_suites": [...],
  "kex_algorithm": "X25519",
  "kex_pqc_ready": false,
  "pfs": true,
  "ocsp_stapled": true,
  "nist_levels": {
    "kex": 1,
    "sig": 1,
    "cipher": 1
  },
  "cbom": {
    "bomFormat": "CycloneDX",
    "specVersion": "1.7",
    "version": 1,
    "metadata": {
      "timestamp": "2025-01-15T10:30:00Z",
      "lifecycles": [
        {
          "phase": "discovery",
          "description": "Point-in-time cryptographic discovery of live TLS endpoints observed over the network"
        }
      ]
    },
    "type": "tls-endpoint",
    "components": [
      {
        "type": "certificate",
        "subject": "CN=example.com",
        "nist_level": 1,
        "quantum_vulnerable": true
      },
      {
        "type": "key-exchange",
        "algorithm": "X25519",
        "pqc_ready": false,
        "nist_level": 1
      },
      {
        "type": "cipher-suite",
        "name": "TLS_AES_256_GCM_SHA384",
        "nist_level": 1
      }
    ]
  }
}
```

Examples:
```bash
# Wallet address (via nginx HTTPS)
curl -k https://localhost/api/discovery/cbom/0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb \
  -H "Authorization: Bearer $TOKEN"

# Wallet address (directly to backend)
curl http://cafe-discovery-backend:8080/discovery/cbom/0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb \
  -H "Authorization: Bearer $TOKEN"

# TLS endpoint (URL-encoded in path)
curl -k "https://localhost/api/discovery/cbom/https%3A%2F%2Fexample.com" \
  -H "Authorization: Bearer $TOKEN"

# TLS endpoint (directly to backend, URL-encoded)
curl "http://cafe-discovery-backend:8080/discovery/cbom/https%3A%2F%2Fexample.com" \
  -H "Authorization: Bearer $TOKEN"
```

Note: For TLS endpoints, the URL must be URL-encoded when passed as a path parameter. The endpoint automatically detects whether the parameter is a wallet address (starts with `0x`) or a URL (starts with `http://` or `https://`).

### POST /discovery/tls/scan

**Deprecated**: Use the unified `/discovery/scan` endpoint instead. This endpoint is kept for backward compatibility.

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

Returns paginated list of CBOMs (Cryptographic Bill of Materials) for TLS endpoint scans for the authenticated user.

Query Parameters:
- `limit` (optional): Number of results per page (default: 20)
- `offset` (optional): Number of results to skip (default: 0)

Response:
```json
{
  "results": [
    {
      "url": "https://example.com",
      "host": "example.com",
      "port": 443,
      "protocol": "TLS 1.3",
      "nist_level": 1,
      "risk_score": 0.75,
      "pqc_risk": "critical",
      "pqc_mode": "classical",
      "supported_pqc": [],
      "recommendations": ["Upgrade to PQC certificates"],
      "scanned_at": "2025-01-15T10:30:00Z",
      "certificate": {
        "subject": "CN=example.com",
        "issuer": "CN=Let's Encrypt",
        "signature_algorithm": "ECDSA-secp256r1",
        "public_key_algorithm": "ECDSA",
        "key_size": 256,
        "nist_level": 1,
        "is_pqc_ready": false,
        "not_before": "2025-01-01T00:00:00Z",
        "not_after": "2025-04-01T00:00:00Z"
      },
      "cipher_suites": [
        {
          "name": "TLS_AES_256_GCM_SHA384",
          "key_exchange": "X25519",
          "encryption": "AES-256-GCM",
          "mac": "SHA384",
          "nist_level": 1,
          "is_pqc_ready": false
        }
      ],
      "kex_algorithm": "X25519",
      "kex_pqc_ready": false,
      "pfs": true,
      "ocsp_stapled": true,
      "nist_levels": {
        "kex": 1,
        "sig": 1,
        "cipher": 1
      },
      "cbom": {
        "bomFormat": "CycloneDX",
        "specVersion": "1.7",
        "version": 1,
        "metadata": {
          "timestamp": "2025-01-15T10:30:00Z",
          "lifecycles": [
            {
              "phase": "discovery",
              "description": "Point-in-time cryptographic discovery of live TLS endpoints observed over the network"
            }
          ]
        },
        "type": "tls-endpoint",
        "components": [
          {
            "type": "certificate",
            "subject": "CN=example.com",
            "issuer": "CN=Let's Encrypt",
            "nist_level": 1,
            "quantum_vulnerable": true,
            "pqc_ready": false
          },
          {
            "type": "key-exchange",
            "algorithm": "X25519",
            "pqc_ready": false,
            "nist_level": 1,
            "quantum_vulnerable": true
          },
          {
            "type": "signature-algorithm",
            "name": "ECDSA-secp256r1",
            "nist_level": 1,
            "quantum_vulnerable": true
          },
          {
            "type": "cipher-suite",
            "name": "TLS_AES_256_GCM_SHA384",
            "nist_level": 1,
            "quantum_vulnerable": true
          }
        ]
      }
    }
  ],
  "total": 1,
  "limit": 20,
  "offset": 0,
  "count": 1
}
```

Note: Each result is a CBOM (Cryptographic Bill of Materials) that includes both the scan data and the structured `cbom.components` array describing all cryptographic primitives.

Example:
```bash
# List TLS scan results with CBOM data
curl -X GET "http://localhost:8080/discovery/tls/scans?limit=10&offset=0" \
  -H "Authorization: Bearer $TOKEN" | jq .

# Via nginx (HTTPS)
curl -k "https://localhost/api/discovery/tls/scans?limit=10&offset=0" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

### GET /discovery/tls/scans/anonymous

Returns list of CBOMs for anonymous TLS scan results from Redis for the current user's token. Also includes default endpoints that are visible to everyone.

Note: Requires a token in the Authorization header (even for anonymous users).

Response:
```json
{
  "results": [
    {
      "url": "https://example.com",
      "host": "example.com",
      "port": 443,
      "protocol": "TLS 1.3",
      "nist_level": 1,
      "risk_score": 0.75,
      "scanned_at": "2025-01-15T10:30:00Z",
      "cbom": {
        "bomFormat": "CycloneDX",
        "specVersion": "1.7",
        "version": 1,
        "metadata": {
          "timestamp": "2025-01-15T10:30:00Z",
          "lifecycles": [
            {
              "phase": "discovery",
              "description": "Point-in-time cryptographic discovery of live TLS endpoints observed over the network"
            }
          ]
        },
        "type": "tls-endpoint",
        "components": [...]
      }
    }
  ],
  "total": 1,
  "count": 1
}
```

Example:
```bash
# Get anonymous TLS scan CBOMs
curl -X GET "http://localhost:8080/discovery/tls/scans/anonymous" \
  -H "Authorization: Bearer YOUR_ANONYMOUS_TOKEN" | jq .
```

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

### 2. Test Unified Scanning

The `/discovery/scan` endpoint automatically detects whether you're scanning a wallet or TLS endpoint:

```bash
# Queue a wallet scan (automatically detected from "address" field)
curl -X POST http://localhost:8080/discovery/scan \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"address": "0x13f735c915bba9136Db794F6b1f42566B24861B8"}'

# Queue a TLS endpoint scan (automatically detected from "url" field)
curl -X POST http://localhost:8080/discovery/scan \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"url": "https://example.com"}'

# Queue a TLS scan with custom port (e.g., 8443)
curl -X POST http://localhost:8080/discovery/scan \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"url": "https://localhost:8443"}'
```

### 3. Retrieve Scan Results (CBOMs)

All scan result endpoints now return CBOMs (Cryptographic Bill of Materials) instead of raw scan data:

```bash
# List wallet scan CBOMs (paginated)
curl -X GET "http://localhost:8080/discovery/scans?limit=10&offset=0" \
  -H "Authorization: Bearer $TOKEN" | jq .

# List TLS scan CBOMs (paginated)
curl -X GET "http://localhost:8080/discovery/tls/scans?limit=10&offset=0" \
  -H "Authorization: Bearer $TOKEN" | jq .

# List anonymous wallet scan CBOMs
curl -X GET "http://localhost:8080/discovery/scans/anonymous" \
  -H "Authorization: Bearer $TOKEN" | jq .

# List anonymous TLS scan CBOMs
curl -X GET "http://localhost:8080/discovery/tls/scans/anonymous" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

Each result in the `results` array is a **CycloneDX v1.7-based CBOM** (extended with custom fields) that includes:
- All scan data (address/url, risk_score, nist_level, etc.)
- A `cbom` object with:
  - `bomFormat`: `"CycloneDX"` (format identifier)
  - `specVersion`: `"1.7"` (specification version)
  - `version`: Document version (currently `1`)
  - `metadata`: Metadata object with `timestamp` (ISO-8601 UTC) and `lifecycles` (for TLS)
  - `type`: `"wallet"` or `"tls-endpoint"`
  - `components`: Array describing cryptographic primitives with NIST SP 800-57 key states (for wallets)

### 4. Retrieve CBOM (Cryptographic Bill of Materials)

**All scan list endpoints now return CBOMs directly!** The endpoints `/discovery/scans` and `/discovery/tls/scans` return lists of CBOMs instead of raw scan data.

#### List Wallet CBOMs

Get a list of CBOMs for all your wallet scans:

```bash
# List wallet scan CBOMs (paginated)
curl -X GET "http://localhost:8080/discovery/scans?limit=10&offset=0" \
  -H "Authorization: Bearer $TOKEN" | jq .

# List anonymous wallet scan CBOMs
curl -X GET "http://localhost:8080/discovery/scans/anonymous" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

Each result in the `results` array is a complete **CycloneDX v1.7-based CBOM** (extended with custom fields) with:
- All scan metadata (address, type, algorithm, risk_score, etc.)
- A `cbom` object containing:
  - `bomFormat`: `"CycloneDX"` (format identifier)
  - `specVersion`: `"1.7"` (specification version)
  - `version`: Document version (currently `1`)
  - `metadata`: Metadata object with `timestamp` (ISO-8601 UTC)
  - `type`: `"wallet"`
  - `components`: Array with cryptographic primitives including NIST SP 800-57 key states (`state: "active"`, `assetType: "related-crypto-material"`, and `customStates` for quantum-vulnerable keys)

#### List TLS Endpoint CBOMs

Get a list of CBOMs for all your TLS endpoint scans:

```bash
# List TLS scan CBOMs (paginated)
curl -X GET "http://localhost:8080/discovery/tls/scans?limit=10&offset=0" \
  -H "Authorization: Bearer $TOKEN" | jq .

# List anonymous TLS scan CBOMs
curl -X GET "http://localhost:8080/discovery/tls/scans/anonymous" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

Each result in the `results` array is a complete **CycloneDX v1.7-based CBOM** (extended with custom fields) with:
- All scan metadata (url, host, port, protocol, risk_score, etc.)
- A `cbom` object containing:
  - `bomFormat`: `"CycloneDX"` (format identifier)
  - `specVersion`: `"1.7"` (specification version)
  - `version`: Document version (currently `1`)
  - `metadata`: Metadata object with `timestamp` (ISO-8601 UTC) and `lifecycles` array declaring the discovery phase
  - `type`: `"tls-endpoint"`
  - `components`: Array describing all cryptographic primitives (certificate, key-exchange, signature-algorithm, cipher-suite)

#### Get Specific CBOM by Address/URL

You can also retrieve a specific CBOM using the `/discovery/cbom/*` endpoint:

```bash
# Get CBOM for a specific wallet address
curl http://localhost:8080/discovery/cbom/0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb \
  -H "Authorization: Bearer $TOKEN" | jq .

# Get CBOM for a specific TLS endpoint (URL must be URL-encoded)
curl "http://localhost:8080/discovery/cbom/https%3A%2F%2Fexample.com" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

Note: URLs must be URL-encoded when passed as path parameters. The endpoint automatically detects if the parameter is a wallet address (starts with `0x`) or a URL (starts with `http://` or `https://`).

#### CBOM Structure

All CBOMs returned by the API are **based on CycloneDX v1.7** and include:

> **Note on CycloneDX Compliance**: CAFE CBOMs follow the CycloneDX v1.7 structure and include standard fields (`bomFormat`, `specVersion`, `version`, `metadata`, `components`), but are **not strictly compliant** because they extend the specification with custom fields outside the standard. These custom fields (e.g., `nist_level`, `quantum_vulnerable`, `key_exposed`, `pqc_ready`) are added to provide cryptographic discovery and post-quantum risk analysis capabilities specific to CAFE's use case.

- **Scan metadata**: All original scan data (address/url, risk_score, nist_level, etc.)
- **CBOM object**: A structured `cbom` object containing:
  - `bomFormat`: Always `"CycloneDX"` (CycloneDX format identifier)
  - `specVersion`: Always `"1.7"` (CycloneDX specification version)
  - `version`: CBOM document version (currently `1`)
  - `metadata`: Metadata object containing:
    - `timestamp`: ISO-8601 UTC timestamp of scan execution
    - `lifecycles`: (TLS only) Array declaring the CBOM lifecycle phase:
      - `phase`: `"discovery"` - Indicates this is a point-in-time discovery CBOM
      - `description`: Explains that this represents network observations
  - `type`: Type of CBOM (`"wallet"` or `"tls-endpoint"`)
  - `components`: Array of cryptographic primitives with details:
    - For wallets: cryptographic-primitive components with NIST SP 800-57 key states
    - For TLS: certificate, key-exchange, signature-algorithm, and cipher-suite components

**Wallet Components** include:
- Type and name of the cryptographic primitive (CycloneDX standard)
- `assetType`: `"related-crypto-material"` (CycloneDX standard - indicates cryptographic material)
- `state`: `"active"` (CycloneDX standard - NIST SP 800-57 key state)
- `customStates`: (if quantum-vulnerable) Array with custom state (CycloneDX standard extension):
  - `name`: `"quantum-vulnerable"`
  - `description`: Explains vulnerability to future quantum attacks
- **Custom fields (CAFE-specific, not in CycloneDX spec)**:
  - `nist_level`: NIST security level (1-5)
  - `quantum_vulnerable`: Boolean indicating quantum vulnerability
  - `key_exposed`: Boolean indicating if the key has been exposed on-chain

**TLS Components** include:
- Type and name of the cryptographic primitive (CycloneDX standard)
- **Custom fields (CAFE-specific, not in CycloneDX spec)**:
  - `nist_level`: NIST security level (1-5)
  - `quantum_vulnerable`: Boolean indicating quantum vulnerability
  - `pqc_ready`: Boolean indicating post-quantum cryptography readiness (for applicable components)
  - Additional TLS-specific fields: `subject`, `issuer`, `signature_algorithm`, `key_size`, `not_before`, `not_after`, etc.

### 5. Public Endpoints

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
docker compose restart prometheus
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

## Stopping Discovery services

To stop all services:

```bash
docker compose down
```

## Additional Resources

- [Post-Quantum JWT Documentation](docs/PQC_JWT.md) - Detailed guide on PQC JWT implementation
- [PQC Certificate Generation Guide](docs/PQC_CERTIFICATES.md) - Guide for generating and testing PQC TLS certificates
- [Open Quantum Safe](https://openquantumsafe.org/) - Official OQS project
- [NIST PQC Standards](https://csrc.nist.gov/projects/post-quantum-cryptography) - NIST post-quantum cryptography standards
