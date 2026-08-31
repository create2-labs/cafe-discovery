# Cafe Discovery Service

1. [Cafe Discovery Service](#cafe-discovery-service)
   1. [Features](#features)
   2. [Architecture](#architecture)
      1. [Goals](#goals)
      2. [System Components](#system-components)
         1. [1. API Server (`cmd/server`)](#1-api-server-cmdserver)
         2. [2. Scanners (`cmd/scanner`)](#2-scanners-cmdscanner)
         3. [3. Persistence Service (cafe-persistence)](#3-persistence-service-cafe-persistence)
         4. [4. NATS](#4-nats)
         5. [5. PostgreSQL](#5-postgresql)
         6. [6. Redis](#6-redis)
      3. [Architecture Decisions](#architecture-decisions)
      4. [Plugin-based scan architecture](#plugin-based-scan-architecture)
      5. [Project Structure](#project-structure)
      6. [Dockerfile Structure](#dockerfile-structure)
      7. [Data Flow](#data-flow)
         1. [Wallet Scan](#wallet-scan)
         2. [TLS Scan](#tls-scan)
      8. [Data structure (CPM export contract)](#data-structure-cpm-export-contract)
      9. [Local Development](#local-development)
   3. [CI/CD and Release Process](#cicd-and-release-process)
      1. [Overview](#overview)
      2. [Pipeline Separation](#pipeline-separation)
         1. [1. Pull Request CI (`.github/workflows/ci.yml`)](#1-pull-request-ci-githubworkflowsciyml)
            1. [Running CI Locally](#running-ci-locally)
         2. [2. Docker RC Pipeline (`.github/workflows/docker-rc.yml`)](#2-docker-rc-pipeline-githubworkflowsdocker-rcyml)
         3. [3. Docker Release Pipeline (`.github/workflows/docker-release.yml`)](#3-docker-release-pipeline-githubworkflowsdocker-releaseyml)
      3. [Release Procedure](#release-procedure)
      4. [Security and Auditability](#security-and-auditability)
      5. [Image Tags](#image-tags)
      6. [Version Endpoint](#version-endpoint)
         1. [Version flow (end-to-end)](#version-flow-end-to-end)
   4. [Configuration](#configuration)
      1. [Configuration File (`config.yaml`)](#configuration-file-configyaml)
   5. [Prerequisites](#prerequisites)
   6. [Running the Service](#running-the-service)
      1. [Development Mode (Local, Outside Docker)](#development-mode-local-outside-docker)
      2. [Docker Compose Mode](#docker-compose-mode)
      3. [Step 1: Build OQS base images](#step-1-build-oqs-base-images)
      4. [Step 2: Start Infrastructure Services](#step-2-start-infrastructure-services)
         1. [Step 3: Build and Start Cafe Discovery Services](#step-3-build-and-start-cafe-discovery-services)
         2. [Step 3-bis: Start Services Independently (Advanced)](#step-3-bis-start-services-independently-advanced)
      5. [Environment Variables](#environment-variables)
      6. [Starting in debug mode](#starting-in-debug-mode)
      7. [Verifying Everything Works](#verifying-everything-works)
   7. [Post-Quantum Cryptography (PQC)](#post-quantum-cryptography-pqc)
      1. [PQC JWT Authentication](#pqc-jwt-authentication)
         1. [Prerequisites for PQC JWT](#prerequisites-for-pqc-jwt)
         2. [JWT Token Format](#jwt-token-format)
         3. [Configuration](#configuration-1)
         4. [Security Considerations](#security-considerations)
      2. [PQC TLS Certificate Scanning](#pqc-tls-certificate-scanning)
         1. [Understanding NIST Security Levels and Risk Scores](#understanding-nist-security-levels-and-risk-scores)
            1. [NIST Security Levels](#nist-security-levels)
            2. [Overall NIST Level Calculation](#overall-nist-level-calculation)
            3. [Risk Score Calculation](#risk-score-calculation)
            4. [Understanding "N/A" in Detailed NIST Levels](#understanding-na-in-detailed-nist-levels)
         2. [Generating PQC Certificates](#generating-pqc-certificates)
         3. [Testing with PQC Certificates](#testing-with-pqc-certificates)
         4. [Current Limitations](#current-limitations)
   8. [API Endpoints](#api-endpoints)
      1. [Authentication](#authentication)
      2. [POST /auth/signup](#post-authsignup)
      3. [POST /auth/signin](#post-authsignin)
      4. [POST /discovery/v1/scan](#post-discoveryv1scan)
      5. [GET /discovery/v1/wallets/scans](#get-discoveryv1walletsscans)
      6. [GET /discovery/v1/wallets/scans/:scan_id](#get-discoveryv1walletsscansscan_id)
      7. [GET /discovery/v1/wallets/scans/:scan_id/cbom](#get-discoveryv1walletsscansscan_idcbom)
      7. [GET /discovery/v1/tls/scans](#get-discoveryv1tlsscans)
      8. [GET /discovery/v1/tls/scans/defaults](#get-discoveryv1tlsscansdefaults)
      9. [GET /discovery/v1/tls/scans/:scan_id](#get-discoveryv1tlsscansscan_id)
      10. [GET /discovery/v1/rpcs](#get-discoveryv1rpcs)
      11. [GET /discovery/v1/scanners](#get-discoveryv1scanners)
      12. [Option A: Discovery wallet scan v1 ↔ CPM contract](#option-a-discovery-wallet-scan-v1--cpm-contract)
      13. [Policy assessment (CPM-owned)](#policy-assessment-cpm-owned)
      14. [GET /version](#get-version)
      16. [GET /health](#get-health)
      17. [GET /metrics](#get-metrics)
      18. [AUTH-05: Internal scan authorization lookup for CPM](#auth-05-internal-scan-authorization-lookup-for-cpm)
   9. [Subscription Plans](#subscription-plans)
      1. [Available Plans](#available-plans)
      2. [Plan Management Endpoints](#plan-management-endpoints)
         1. [GET /plans](#get-plans)
         2. [GET /plans/current](#get-planscurrent)
         3. [GET /plans/usage](#get-plansusage)
      3. [Plan Enforcement](#plan-enforcement)
      4. [Worker Health Check](#worker-health-check)
   10. [Testing](#testing)
       1. [1. Register and Authenticate](#1-register-and-authenticate)
       2. [2. Test Unified Scanning](#2-test-unified-scanning)
       3. [3. List Scan Summaries](#3-list-scan-summaries)
       4. [4. Retrieve Scan Details by scan_id](#4-retrieve-scan-details-by-scan_id)
       5. [5. Public Endpoints](#5-public-endpoints)
   11. [Risk Scoring](#risk-scoring)
       1. [Wallet Risk Score](#wallet-risk-score)
       2. [TLS Risk Score](#tls-risk-score)
          1. [Calculation Method](#calculation-method)
          2. [Final Score](#final-score)
          3. [Risk Categories](#risk-categories)
   12. [Observability](#observability)
       1. [Metrics Endpoint](#metrics-endpoint)
       2. [Available Metrics](#available-metrics)
          1. [Wallet Scan Metrics](#wallet-scan-metrics)
          2. [TLS Scan Metrics](#tls-scan-metrics)
       3. [Metric Collection](#metric-collection)
       4. [Prometheus Configuration](#prometheus-configuration)
       5. [Metric Design Principles](#metric-design-principles)
   13. [Background Processing](#background-processing)
   14. [Development Tools](#development-tools)
       1. [Wallet public-key CLIs (moved to `cafe-scanner-wallet`)](#wallet-public-key-clis-moved-to-cafe-scanner-wallet)
   15. [Security Notes](#security-notes)
   16. [Stopping Discovery services](#stopping-discovery-services)
   17. [Additional Resources](#additional-resources)

A Discovery service for identifying cryptographic exposures and quantum vulnerabilities on the Ethereum network and related infrastructure.

> **Deployment:** This repository is **DEV/BUILD only**. Staging and production are deployed only from [cafe-deploy](https://github.com/create2-labs/cafe-deploy). Use the Docker Compose files here for local development and testing only.

> Wallet worker runtime/images have been moved to [cafe-scanner-wallet](https://github.com/create2-labs/cafe-scanner-wallet).

> TLS worker runtime/images have been moved to [cafe-scanner-tls](https://github.com/create2-labs/cafe-scanner-tls). 

## Features

- Wallet Scanning: Scan wallets across multiple EVM-compatible networks
- Key Exposure Detection: Detect whether a wallet's public key has been revealed on-chain
- Account Type Detection: Determine if an address is an EOA (Externally Owned Account) or AA (Abstract Account/ERC-4337)
- Risk Assessment: Calculate risk scores based on exposure across networks
- Quantum Security Level: Assess NIST quantum-security levels
- TLS Scanning: Scan TLS endpoints for post-quantum cryptography (PQC) certificate support
- Post-Quantum JWT: Hybrid PQC JWT tokens (EdDSA + ML-DSA-65) for quantum-resistant authentication
- **Structured cryptographic discovery results**: v1 detail endpoints return scan results by `scan_id` with wallet/TLS posture fields such as `nist_level`, `risk_score`, `current_pq_posture`, and scanner-specific observations.
- **Subscription Plans**: Free and Premium (CAFEIN) plans with usage limits
- **Versioning**: Automatic version tracking via `/version` endpoint and Docker image tags
- **Authentication**: All backend API calls require authentication.

## Architecture



The application is designed to be scalable with a focus on performance.

### Goals

1. Scalability: scanner processes to be able to scale
2. Resilience: NATS messages can be persisted with JetStream; this is not implemented yet
3. Performance: HTTP requests return immediately
4. Decoupling: API and processing are separated
5. Load Distribution: Multiple scanners share the load via NATS queues

### System Components

#### 1. API Server (`cmd/server`)

- Role: HTTP server (Fiber) that exposes REST endpoints. **All API calls require authentication**. Target role: **control-plane** (HTTP API + NATS publish/subscribe + Redis reads).
- Responsibilities:
  - User authentication with hybrid PQC JWT tokens
  - Receiving scan requests (wallet and TLS)
  - Publishing NATS messages for asynchronous processing
  - Consuming `scan.ready` NATS messages (when persistence has written a result to Redis) so GET requests can return results
  - Serving scan list/GET from Redis; on cache miss, **currently** the backend uses **read-through** from PostgreSQL (see [Architecture Decisions](#architecture-decisions) and [docs/CHECKARCH.md](docs/CHECKARCH.md)).
- **Current state (PostgreSQL usage):** The backend **currently** connects to PostgreSQL and uses it for: (1) **Auth and users** (signup/signin, user and plan records), (2) **Plans and usage** (plan limits, scan counts), (3) **Cafe wallets** (CRUD), (4) **Pending scan records** (creating a row when a scan is requested, before the scanner runs), (5) **Read-through** for scan list/GET when Redis is empty (e.g. after sign-in warm or first request). The intended direction is to reserve Postgres for the persistence service only (scan lifecycle) and to serve scan data from Redis only from the API; see [docs/CHECKARCH.md](docs/CHECKARCH.md) for verification and envisioned changes.

#### 2. Scanners (`cmd/scanner`)

- Role: NATS consumers that process scans (**compute-only**: NATS in/out + heartbeat; no Postgres). The scanner process can run **one or both** scanner types (TLS and Wallet) depending on `DISCOVERY_SCANNER_TYPE`.
- **Scanner core** (`internal/scanner/core`): Shared bootstrap — **NATS**, chain config, health server, graceful shutdown. Defines the `Runner` interface and `Deps`. Scanners do **not** have a DB or plan service; they only publish `scan.started` / `scan.completed` / `scan.failed` to NATS for the persistence service.
- **TLS scanner** (`internal/scanner/tlsrunner`): Consumes `cafe.discovery.tls.scan`, runs TLS scans via the TLS plugin (requires OQS/liboqs for PQC scanning).
- **Wallet scanner**: moved to [cafe-scanner-wallet](https://github.com/create2-labs/cafe-scanner-wallet) (dedicated repository and image).
- Responsibilities:
  - Consuming NATS messages (wallet and/or TLS subject)
  - Decoding messages and running scans via the **scan plugins** (`plugin.DecodeMessage`, `plugin.Run`)
  - Publishing scan lifecycle events to NATS (`scan.started`, `scan.completed`, `scan.failed`) for the **persistence service** to write to storage
- **Deployment**: For production you can run one process per type (`DISCOVERY_SCANNER_TYPE=tls` or `wallet`), each with its own Docker image (TLS image uses OQS; Wallet image is Alpine without OQS).

#### 3. Persistence Service (cafe-persistence)

- Role: Single writer to PostgreSQL and Redis for scan lifecycle events. **Extracted to [cafe-persistence](https://github.com/create2-labs/cafe-persistence)** (PERS-D1/D2); no longer built from this repository (PERS-D1b).
- Responsibilities (data plane — see cafe-persistence README):
  - Subscribing to NATS subjects `scan.started`, `scan.completed`, `scan.failed` (queue `cafe.persistence`)
  - Writing scan results idempotently to PostgreSQL (TLS and wallet scan tables) and to Redis (write-through cache for performance)
  - When a scan result has been written to Redis (and PostgreSQL), publishing a NATS message (`scan.ready`); the **backend consumes this message** so GET requests can return the result
  - After a **successful wallet** `scan.completed` write, publishing a **normative observation** JSON on `cafe.discovery.events.wallet.observed.v0_1` (see [Data structure (CPM export contract)](#data-structure-cpm-export-contract)); best-effort, does not roll back the scan if publish fails
  - Enforcing valid scan state transitions
  - Publishing `persistence.ready` on startup so the backend can wait for persistence before initializing default endpoints
- **Startup order**: The backend waits for `persistence.ready` (and scanner heartbeats) before seeding default TLS endpoints. Run cafe-persistence before or with the backend for full functionality.
- **Deployment**: Image `oleglod/cafe-persistence:${PERSISTENCE_VERSION}` via [cafe-deploy](https://github.com/create2-labs/cafe-deploy) (compose service `cafe-persistence`). Legacy rollback: [docs/PERSISTENCE_EXTRACTION.md](docs/PERSISTENCE_EXTRACTION.md).

#### 4. NATS

- Role: Messaging system for asynchronous communication
- Note: NATS is managed in [cafe-infra](https://github.com/kantika-tech/cafe-infra)
- Subjects:
  - `cafe.discovery.wallet.scan`: Wallet scan requests
  - `cafe.discovery.tls.scan`: TLS scan requests
  - `scan.started`, `scan.completed`, `scan.failed`: Scan lifecycle events (consumed by persistence service)
  - `scan.ready`: Published by persistence when a scan result has been written to Redis (and PostgreSQL); consumed by the backend so GET requests can return the result
  - `cafe.discovery.events.wallet.observed.v0_1`: Published by persistence after a successful **wallet** scan write; JSON matches `cafe-contracts` `cafe.discovery.wallet.observed` **v0.1** (informational observation on the bus — not a CPM command; see execution pack **v0.7** in `cafe-crypto-policy-mgt`)
  - `cafe.policy.events.policy.assessment.requested.v0_1`: Published on explicit authenticated user action via **CPM** (`POST /api/cpm/v1/policies/assessment/request`, see `cafe-crypto-policy-mgt`); JSON matches `cafe-contracts` `cafenatsv01.PolicyAssessmentRequested`. Discovery does **not** expose an HTTP assessment trigger.
  - `persistence.ready`: Published by persistence on startup (consumed by backend to know when to seed default endpoints)
- Queues: `cafe.scanners` (scanners), `cafe.persistence` (persistence service)

#### 5. PostgreSQL

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

#### 6. Redis

- Role: **Performance cache only** (write-through).
- Note: Redis is managed in [cafe-infra](https://github.com/kantika-tech/cafe-infra)
- Stores:
  - **User-scoped scan results** (TLS and wallet): Written by the **persistence service** after `scan.completed` / `scan.failed`; read by the API for GET by user+url or user+address.
- Flow:
  - Persistence writes to PostgreSQL then to Redis; when the result is in Redis, persistence publishes a NATS message (`scan.ready`); the backend consumes this message. The backend serves GET requests from Redis; **currently** it also performs read-through from PostgreSQL on cache miss (see [Architecture Decisions](#architecture-decisions)).
- Advantages:
  - Fast in-memory reads; reduces load on PostgreSQL for repeated GETs
  - Low latency for read/write operations

### Architecture Decisions

- **Single-writer principle (target):** Only the **persistence service** should write to PostgreSQL and Redis for **scan lifecycle** (scan results). The backend should act as control-plane (API + NATS + Redis reads only). Scanners are execution-plane (NATS + heartbeat, no Postgres). *Current code still has the backend creating pending scan rows and doing read-through from Postgres; see [docs/CHECKARCH.md](docs/CHECKARCH.md).*
- **Redis as the read path for scans:** Scan list/GET are intended to be served from Redis so that the API does not depend on Postgres for hot path. Persistence writes through to both Postgres and Redis; the backend reads from Redis. On Redis miss, the target behavior is to return a consistent NOT_READY/404-style response rather than rehydrating from Postgres in the backend.
- **Why the backend should not read Postgres for scan data (target):** Keeps the API stateless with respect to the database and avoids duplicate read paths; persistence remains the single writer and Redis the single read source for scan results from the API’s perspective.
- **Future:** Two-network Docker Compose (e.g. control-plane vs data-plane isolation) is not implemented yet; it may be added later to enforce single-writer at the network level.

**Verification:** A code-based verification of Postgres usage (backend vs scanners) and documentation alignment is recorded in [docs/CHECKARCH.md](docs/CHECKARCH.md).

- **Persistence platform (PERS-D0–D6):** Scan lifecycle writing lives in **`cafe-persistence`** (data plane; **PERS-D1b** removed in-repo `cmd/persistence`). Discovery keeps the **identity plane** (auth, plans, `cafe_wallets`) and scan **control plane** (HTTP + NATS publish). **No CP domain code** in this repository — guards W1/W3 call CPM or cafe-persistence for existence checks only (see ADR §9.3). Normative ADR: [docs/ADR/ADR_20260622_persistence.md](docs/ADR/ADR_20260622_persistence.md) ; extraction + rollback: [docs/PERSISTENCE_EXTRACTION.md](docs/PERSISTENCE_EXTRACTION.md) ; PR checklists: [docs/ADR/ADR_20260622_persistence_PR_PLAN.md](docs/ADR/ADR_20260622_persistence_PR_PLAN.md).

### Plugin-based scan architecture

Scans are implemented as **plugins** registered in a central registry:

- **`pkg/scan`**: Defines `ScanTarget`, `ScanResult`, `Plugin` interface (Descriptor, DecodeHTTP, DecodeMessage, Run), and a thread-safe **registry** (`Register`, `Get(kind)`, `GetBySubject`). Kinds: `tls`, `wallet`; plan limit keys: `endpoint` (TLS), `wallet`.
- **TLS plugin** (`internal/scan/tls`): Implements `scan.Plugin` for TLS endpoint scans. Adapter wraps `domain.TLSScanResult` as `scan.ScanResult` while preserving the current v1 detail payload. Consumes NATS subject `cafe.discovery.tls.scan`.
- **Wallet plugin** (`internal/scan/wallet`): Implements `scan.Plugin` for wallet scans. Adapter wraps `domain.ScanResult` as `scan.ScanResult`. Consumes NATS subject `cafe.discovery.wallet.scan`.

Handlers validate requests (optionally via `plugin.DecodeHTTP`) and publish the same NATS messages as before. Workers unmarshal messages, call `plugin.DecodeMessage` then `plugin.Run`. Plan limits use kind-based constants (`scan.KindWallet`, `scan.PlanLimitKeyEndpoint`). Plugin versions are configurable via `scan.plugins.tls.version` and `scan.plugins.wallet.version` in config.

### Project Structure

```
cafe-discovery/
├── cmd/
│   ├── server/            # API server entrypoint
│   ├── scanner/            # Scanner entrypoint (runs TLS and/or Wallet via core + runners)
│   └── cli/
│      └── tls-scan/       # TLS/OQS tooling (moving to cafe-scanner-tls — PR5)
├── internal/
│   ├── app/               # Application container (orchestration)
│   ├── domain/            # Domain models and types
│   ├── handler/           # HTTP handlers (Fiber)
│   ├── metrics/           # Prometheus metrics registration
│   ├── scan/              # Scan plugins (implement pkg/scan.Plugin)
│   │   ├── tls/           # TLS plugin + result adapter
│   │   └── wallet/        # Wallet plugin + result adapter
│   ├── service/           # Business logic
│   └── scanner/            # Scanner runtime and runners
│       ├── core/          # Shared bootstrap (Deps, Setup, Run, Runner interface)
│       ├── tlsrunner/     # TLS scanner runner (plugin + TLSScanner)
│       ├── base_scanner.go # Base NATS subscription + handler
│       ├── tls_scanner.go  # TLS scan message handler
│       ├── wallet_scanner.go
│       └── helper.go      # Concurrency + logging helper
├── pkg/
│   ├── evm/               # EVM client for blockchain interactions
│   ├── nats/              # NATS messaging client
│   ├── postgres/          # PostgreSQL database client
│   ├── pqc/               # Post-quantum cryptography (JWT, KEM)
│   ├── redis/             # Redis database client
│   ├── scan/              # Scan plugin API (kinds, target, result, plugin, registry)
│   └── tls/               # TLS scanner with PQC support
├── docs/
│   ├── PQC_CERTIFICATES.md
│   ├── PQC_JWT.md
│   ├── SCAN_REFACTORING_PLAN.md
│   └── SCAN_PLUGIN_ARCHITECTURE.md
├── scripts/
├── Dockerfile        # API server (OQS)
├── docker-compose.yml
└── config.yaml
```

### Dockerfile Structure

The project uses a multi-stage Docker build approach:

1. **OQS base images** (built in [cafe-crypto-backend](https://github.com/create2-labs/cafe-crypto-backend)):
   - Build: run `scripts/build.sh` in cafe-crypto-backend (see [cafe-crypto-backend/README.md](https://github.com/create2-labs/cafe-crypto-backend))
   - Images: `oleglod/cafe-crypto-backend:build-oqs` and `oleglod/cafe-crypto-backend:runtime-oqs`

2. **`Dockerfile`**:
   - Builds the API server binary
   - Uses `oleglod/cafe-crypto-backend:build-oqs` as base
   - Output: `cafe-discovery-backend` service

3. **Persistence image** (separate repo [cafe-persistence](https://github.com/create2-labs/cafe-persistence)):
   - `Dockerfile` builds the persistence binary; deployed as `oleglod/cafe-persistence`
   - Not built from this repository after PERS-D1b — see [docs/PERSISTENCE_EXTRACTION.md](docs/PERSISTENCE_EXTRACTION.md)
   - Run this service so the backend can receive `persistence.ready` and seed default endpoints; API GET results depend on persistence writing after scanner completion.

4. **Scanner images** are produced by dedicated repositories:
   - TLS scanner: `cafe-scanner-tls` (`oleglod/cafe-scanner-tls`)
   - Wallet scanner: `cafe-scanner-wallet` (`oleglod/cafe-scanner-wallet`)

Build order:
1. Build the OQS base images from [cafe-crypto-backend](https://github.com/create2-labs/cafe-crypto-backend) (see [Step 1: Build OQS base images](#step-1-build-oqs-base-images)).
2. Build discovery services: `docker compose -f docker-compose.yml -f docker-compose.dev.yml build` (or `up --build`). This repository now builds backend only; scanners run from dedicated repositories.

### Data Flow

#### Wallet Scan

```
Client HTTP → Discovery → NATS (publish) → Scanner → NATS (scan.started/completed/failed) → Persistence → PostgreSQL + Redis
               backend           ↓                              ↓
                              Immediate Response              Persistence writes; publishes scan.ready → Backend can return GET result
```

1. Client sends a POST request to `/discovery/v1/scan`
2. API Server validates the request and publishes a NATS message
3. Client receives an immediate response with `scan_id`, `scan_family`, `status: "requested"`, and a detail `location`
4. A scanner consumes the message and processes the scan, then publishes `scan.started` / `scan.completed` or `scan.failed`
5. The **persistence service** consumes those events and saves the result to PostgreSQL and Redis (write-through); then publishes `scan.ready` so the API can return the result on GET

#### TLS Scan

Authenticated Users:
```
Client HTTP → Discovery  → NATS (publish) → Scanner → NATS (scan.started/completed/failed) → Persistence → PostgreSQL + Redis
               backend            ↓
                         Immediate Response
```

1. Client sends a POST request to `/discovery/v1/scan`
2. API Server validates the request and publishes a NATS message to `cafe.discovery.tls.scan`
3. Client receives an immediate response with `scan_id`, `scan_family: "tls"`, `status: "requested"`, and a detail `location`
4. A scanner consumes the message and processes the TLS scan (checks for PQC certificate support), then publishes scan lifecycle events
5. The **persistence service** consumes those events and saves the result to PostgreSQL and Redis; when the result is in Redis, it publishes `scan.ready`; the **backend consumes this message** and can then return the result on GET.

**Note:** All backend API calls require authentication. Unauthenticated users cannot call the API.

### Data structure (CPM export contract)

Discovery remains the **owner** of internal scan models and persistence (for example wallet scan results and `ScanResultEntity`). The **normative wire contract** for a wallet observation is defined in **`cafe-contracts`** (`observation/wallet/v01`, `event_type` `cafe.discovery.wallet.observed`, `event_version` `v0.1`). Discovery maps `domain.ScanResult` to that contract in **`internal/walletobservation`** and uses **`config.ChainConfig.ChainIDByNetwork()`** built from **`blockchains[].name`** + **`chain_id`** in `config.yaml` (validated at startup where `LoadChainConfig` runs).

**Runtime:** the **persistence service** publishes the JSON to NATS subject **`cafe.discovery.events.wallet.observed.v0_1`** after a successful wallet `scan.completed` persistence path (Postgres + Redis + `scan.ready`). Publication is best-effort (logged on validation/publish failure; scan write is not rolled back).

**Integration semantics (execution pack v0.7):** that message is an **observation / informational** event on the bus. **Crypto Policy Management (CPM)** must **not** treat it as an automatic trigger for policy assessment. Assessment is started only from an explicit command (e.g. **`policy.assessment.requested.v0.1`**) or equivalent API, as described in [`cafe_cpm_v1_prompts_0.7.md`](https://github.com/create2-labs/cafe-crypto-policy-mgt/blob/main/cafe_cpm_v1_prompts_0.7.md) in repository **`cafe-crypto-policy-mgt`**. The same wire types may be **embedded** in that command as a snapshot.

| Topic | Rule |
| --- | --- |
| Contract ID | `cafe.discovery.wallet.observed` at version **`v0.1`** (`event_type` / `event_version` on the wire) |
| Wire types & vocabulary | Packaged in **`cafe-contracts`**; CPM owns semantics; exported strings are stable enums / patterns |
| Producer label | JSON field `producer` must be `cafe-discovery` where the contract requires it |
| Chain identity | **Numeric EVM chain IDs** in `chain_ids`, from config mapping; omit unknown networks (no sentinel `0`) |
| Discovery-only fields | User rows, plan limits, scanner job internals, raw CBOM blobs — **not** part of this export |

**Envelope (observation event)** — top-level fields CPM validates for v0.1:

- `event_id`, `event_type`, `event_version`, `occurred_at`, `correlation_id`, `causation_id`, `producer`
- `subject`: `{ "type": "wallet", "id": "<stable wallet subject id>" }`
- `payload`: policy-relevant observation block (see below)

**Payload — observed (policy inputs)**:

| Field | Meaning |
| --- | --- |
| `chain_ids` | Active chains for this observation (numeric IDs) |
| `account_kind` | Normalized account model (see vocabulary) |
| `current_algorithm` | Normalized algorithm identifier (see vocabulary) |
| `public_key_exposed` | Whether the public key is considered exposed for policy purposes |
| `is_multichain` | Whether the wallet is observed across more than one chain |
| `observed_at` | Timestamp of the observation |

**Payload — derived**:

| Field | Meaning |
| --- | --- |
| `current_pq_posture` | Summary `classical_only` \| `hybrid` \| `full_pq` \| `unknown` — in current Discovery export this may be placeholder `unknown` until posture derivation lands; see execution pack |

**Exported vocabulary** — values Discovery must map to when emitting this contract:

- **Account kinds:** `eoa`, `erc4337_smart_account`, `delegated_eoa_7702`, `contract_account`, `unknown`
- **Algorithms:** `secp256k1_ecrecover`, `mldsa44`, `mldsa65`, `falcon512`, and any non-empty string with prefix `hybrid_` for hybrid profiles
- **Subject type (v0.1):** `wallet`
- **PQ posture:** `classical_only`, `hybrid`, `full_pq`, `unknown`

**Canonical JSON fixture:** [`cafe-contracts` `observation/wallet/v01/testdata/cafe_discovery_wallet_observed_v01.json`](https://github.com/create2-labs/cafe-contracts/blob/main/observation/wallet/v01/testdata/cafe_discovery_wallet_observed_v01.json) (module `github.com/create2-labs/cafe-contracts`). Local placeholder tests also use `internal/walletobservation/testdata/`.

**Further reading:** [cafe-crypto-policy-mgt `cafe_cpm_v1_prompts_0.7.md`](https://github.com/create2-labs/cafe-crypto-policy-mgt/blob/main/cafe_cpm_v1_prompts_0.7.md) — authoritative pack for CPM integration (explicit assessment trigger, not auto-consume of observation stream).

### Local Development

- Infrastructure services (PostgreSQL, NATS, Redis) are managed in [cafe-infra](https://github.com/kantika-tech/cafe-infra)
- Run API server, **persistence service**, and scanner(s) as separate processes or via Docker Compose (local only). The backend waits for `persistence.ready` at startup before seeding default endpoints; for full behavior (including GET results after scans), the persistence service must be running.
- Staging/production deployment is done from [cafe-deploy](https://github.com/create2-labs/cafe-deploy)

## CI/CD and Release Process

This project implements a strict, security-focused CI/CD pipeline that enforces quality gates and ensures all published Docker images are secure and traceable.

### Overview

The project produces the **backend image**:
- `oleglod/cafe-discovery-backend`: API server image (`Dockerfile`)

Persistence is published from **[cafe-persistence](https://github.com/create2-labs/cafe-persistence)** as `oleglod/cafe-persistence` (see [docs/PERSISTENCE_EXTRACTION.md](docs/PERSISTENCE_EXTRACTION.md)).

Scanner images are published from dedicated repositories:
- `oleglod/cafe-scanner-tls` (from `cafe-scanner-tls`)
- `oleglod/cafe-scanner-wallet` (from `cafe-scanner-wallet`)

### Pipeline Separation

The CI/CD pipeline is strictly separated into three distinct workflows:

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
- OQS base images built (see [Step 1: Build OQS base images](#step-1-build-oqs-base-images))

**Method 1: Using Docker Compose (Recommended)**

The `docker-compose.yml` file includes CI service definitions that build the CI images:

```bash
# Build CI images (if your compose defines them; otherwise use Method 2)
docker compose build cafe-discovery-backend-ci

# Run CI checks for backend
docker compose run --rm cafe-discovery-backend-ci

```

**Method 2: Using Docker Directly**

You can also build and run the CI images directly with Docker (build context = this repo root, where `Dockerfile` lives):

```bash
docker build \
  --target ci \
  -f Dockerfile \
  -t cafe-discovery-backend:ci .

docker run --rm cafe-discovery-backend:ci

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

- **Build fails with "cafe-crypto-backend:build-oqs not found"**: Pull or build the OQS base images from [cafe-crypto-backend](https://github.com/create2-labs/cafe-crypto-backend) (see [Step 1: Build OQS base images](#step-1-build-oqs-base-images))
- **Linter timeout**: Increase timeout in `.golangci.yml` or run with `--timeout=10m`
- **Tests fail**: Check that all dependencies are available and tests are passing locally
- **govulncheck fails**: Update dependencies with `go get -u ./...` and `go mod tidy`

**CI Image Details:**

The CI images (`ci` target) include:
- Go 1.25.7+ runtime (required to fix GO-2025-4175, GO-2025-4155, and GO-2026-4337 vulnerabilities)
- Open Quantum Safe (OQS) libraries
- `golangci-lint` v2.8.0
- `govulncheck` (latest)
- All project dependencies

The CI images are based on the `builder` stage, which includes the full build environment. They execute the CI checks as the default command when run.

#### 2. Docker RC Pipeline (`.github/workflows/docker-rc.yml`)

**Trigger**: PR label `rc-vX.Y.Z` or `build-vX.Y.Z` (internal PRs only), or manual **Run workflow** (`workflow_dispatch`).

**Purpose**: Build, scan, and push RC images to Docker Hub for staging and later release promotion. This is the **only** workflow that compiles the backend image and injects `APP_VERSION`.

**Registry**: `oleglod/cafe-discovery-backend` on Docker Hub.

**Process**:
1. **Extract version information** from the commit being built:
   - `short_sha` — always (source of truth for promotion)
   - Optional `rc_tag` — when **Target version** is set on manual run, or from PR label (`vX.Y.Z-rc<run_id>`)
   - `app_version` — passed as `--build-arg APP_VERSION=...` (exposed by `GET /version`):
     - `vX.Y.Z-rc<run_id>` when an RC tag is produced
     - `dev-<short_sha>` otherwise

2. **Build and scan** (linux/amd64 local load, then multi-arch push):
   - Docker Scout scans the amd64 image (critical/high; non-blocking by default)
   - Multi-arch push: `linux/amd64`, `linux/arm64`

3. **Publish** (always `sha-<short_sha>`; optional RC tag):
   - `oleglod/cafe-discovery-backend:sha-<short_sha>` — **required** before release
   - `oleglod/cafe-discovery-backend:vX.Y.Z-rc<run_id>` — optional, human-readable RC tag
   - Does **not** push `vX.Y.Z` or `latest` (release promotes those)

**Security Gates**:
- Fork PRs are refused (no secrets on untrusted code)
- Docker Scout runs on the RC build

See [cafe-deploy README — Step 2 (Docker RC)](https://github.com/create2-labs/cafe-deploy/blob/main/README.md#step-2--build-and-push-rc-images-docker-rc) for operational triggers and env pinning.

#### 3. Docker Release Pipeline (`.github/workflows/docker-release.yml`)

**Trigger**: Push of Git tags matching `v*.*.*` (e.g., `v1.2.3`)

**Purpose**: **Promote** an existing RC image to release tags — **no rebuild**, same image digest as `sha-<short_sha>`.

**Registry**: `oleglod/cafe-discovery-backend` on Docker Hub.

**Process**:
1. **Extract version** from the Git tag (`vX.Y.Z`) and resolve `short_sha` from the tagged commit.
2. **Verify** that `oleglod/cafe-discovery-backend:sha-<short_sha>` exists and is multi-arch (`linux/amd64`, `linux/arm64`).
3. **Promote** via `docker buildx imagetools create` (retag only):
   - `oleglod/cafe-discovery-backend:vX.Y.Z`
   - `oleglod/cafe-discovery-backend:latest`

**Important**: Release does **not** rebuild the image and does **not** change `APP_VERSION` baked in at RC build time. `GET /version` still reports whatever was set when Docker RC ran. Aligning `/version` with the semver Docker tag without rebuilding is an open decision — see [cafe-deploy `TODO.md`](https://github.com/create2-labs/cafe-deploy/blob/main/TODO.md) (must preserve promote-without-rebuild).

**Prerequisite**: Docker RC must have been run for the same commit before pushing the Git tag; otherwise promotion fails.

### Release Procedure

Releases are **manual and explicit**. The CI system never creates tags automatically.

**Step-by-step release process**:

1. **Merge PR to `main`**:
   - Ensure the PR has passed all CI checks (lint, tests, govulncheck)
   - Merge the PR into `main`

2. **Run Docker RC** on `main` (Actions → Docker RC → Run workflow):
   - Optionally set **Target version** to the planned semver (`1.2.3` or `v1.2.3`) so `APP_VERSION` and an optional RC tag reflect the release line
   - Confirms `oleglod/cafe-discovery-backend:sha-<short_sha>` exists on Docker Hub

3. **Validate in staging** (cafe-deploy): pin `DISCOVERY_VERSION` to `sha-<short_sha>` or the RC tag; run smokes.

4. **Create Git tag** (manually, after validation):
   ```bash
   git checkout main
   git pull origin main
   git tag v1.2.3
   git push origin v1.2.3
   ```

5. **Docker Release runs automatically**:
   - Promotes `sha-<short_sha>` to `v1.2.3` and `latest` (no rebuild)
   - Fails if the RC image for that commit is missing or not multi-arch

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
- Docker Scout runs on RC builds (critical/high severities)
- Backend and persistence images are built and released from separate repositories; scanner images are versioned independently.

**Failure Handling**:
- Release promotion fails if the RC image `sha-<short_sha>` is absent — fix by running Docker RC for that commit, then re-push the tag or create a new tag on the same commit

### Image Tags

**Docker RC** (build):

- `sha-<short_sha>` — always pushed; source of truth for promotion and staging pins
- `vX.Y.Z-rc<run_id>` — optional, when target version or PR label is provided

**Docker Release** (promote, no rebuild):

- `vX.Y.Z` — from Git tag
- `latest` — most recent release

RC and release images are multi-arch (`linux/amd64`, `linux/arm64`). Image from this repository: `cafe-discovery-backend`. Persistence: `oleglod/cafe-persistence` ([cafe-persistence](https://github.com/create2-labs/cafe-persistence)).

### Version Endpoint

The backend exposes a `/version` endpoint that returns the application version:

```bash
curl http://localhost:8080/version
```

Response:
```json
{
  "version": "v1.2.3"
}
```

`APP_VERSION` is set **only** when Docker RC builds the image (`--build-arg APP_VERSION=...`): `vX.Y.Z-rc<run_id>` when a target version or RC label is provided, otherwise `dev-<short_sha>`. Docker Release promotes tags but does **not** rebuild or change `APP_VERSION`.

#### Version flow (end-to-end)

1. **Docker RC** (`docker-rc.yml`): passes `--build-arg APP_VERSION=...` to the backend image build.
2. **Dockerfile**: embeds `APP_VERSION` into the Go binary via `-ldflags` (`internal/version`). Runtime override via `APP_VERSION` env is also supported (not set by cafe-deploy compose today).
3. **Docker Release** (`docker-release.yml`): retags `sha-<short_sha>` to `vX.Y.Z` / `latest` only — same digest, same baked-in version.
4. **Backend container**: serves `GET /version` on port **8080**, returning `{"version": "..."}`.
5. **Infra** (cafe-deploy): NGINX proxies `location = /api/version` to `http://cafe-discovery-backend:8080/version`.
6. **Frontend** (cafe-frontend): `platformService.getBackendVersion()` calls `/api/version` and displays the value to the user.

`GET /version` may therefore differ from the Docker Hub release tag (e.g. `dev-abc1234` or `v1.2.3-rc123` while the deployed image tag is `v1.2.3`). How to align them without rebuilding at release is documented as an open decision in [cafe-deploy `TODO.md`](https://github.com/create2-labs/cafe-deploy/blob/main/TODO.md).

The response format **must** remain `{"version": "..."}`; the frontend and infra rely on it.

## Configuration

The application can be configured using either:
1. `config.yaml` file (recommended for local Docker runs)
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
JWT_SECRET: "change-me-for-local"

# Cloudflare Turnstile configuration (optional, uses dev keys by default)
TURNSTILE_SECRET_KEY: "1x0000000000000000000000000000000AA"
TURNSTILE_SITE_KEY: "1x00000000000000000AA"

# Logging
LOG_LEVEL: "info"

# Scanner type: "tls" | "wallet" | "all" (default). For separate scanner processes set via DISCOVERY_SCANNER_TYPE.
# DISCOVERY_SCANNER_TYPE: "all"

# Scan plugin versions (optional; default "1.0")
scan:
  plugins:
    tls:
      version: "1.0"
    wallet:
      version: "1.0"

# CORS configuration
CORS_ALLOW_ORIGINS: "http://localhost:3000,http://localhost:3001,http://localhost:5173"
CORS_ALLOW_METHODS: "GET,POST,PUT,DELETE,OPTIONS"

blockchains:
  - name: ethereum-mainnet
    rpc: "https://ethereum-rpc.publicnode.com"          # exposed via GET /discovery/v1/rpcs; live RPC calls run in cafe-scanner-wallet
    moralis_chain_name: "eth"                           # scanner-wallet indexer only (ignored by discovery-backend)
    chain_id: 1   # EIP-155; required for wallet observation export (persistence); must be unique per row
  - name: polygon
    rpc: "https://polygon-bor-rpc.publicnode.com"
    moralis_chain_name: "polygon amoy"
    chain_id: 137
  # ... more networks
```

Note:
- Discovery **does not** call Moralis or blockchain RPC at runtime (orchestrator + NATS only). Wallet scans run in **`cafe-scanner-wallet`**, which consumes `rpc` and `moralis_chain_name` from the shared config and requires `MORALIS_API_KEY` (or a future Etherscan key) in its own environment — see **`cafe-scanner-wallet`** README and **`cafe-deploy`** compose.
- Environment variables always override values from `config.yaml` 
- For local Docker Compose, use service names (e.g., `postgres`, `nats`, `redis`) as hostnames
- The `CONFIG_PATH` environment variable can be used to specify a custom config file path (default: `config.yaml`)
- Each **`blockchains[]`** entry must include a positive **`chain_id`** (validated when `LoadChainConfig` runs — used by API/scanner **and** persistence for `cafe.discovery.wallet.observed` `chain_ids` mapping)

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

Backend and scanner are managed by Docker Compose

### Step 1: Build OQS base images

Before building the discovery services, you must have the OQS base images from [cafe-crypto-backend](https://github.com/create2-labs/cafe-crypto-backend):

```bash
# Option A: Build from cafe-crypto-backend
cd ../cafe-crypto-backend
./scripts/build.sh
cd ../cafe-discovery

# Option B: Pull from Docker Hub
docker pull oleglod/cafe-crypto-backend:build-oqs
docker pull oleglod/cafe-crypto-backend:runtime-oqs
```

This provides the base images:
- `oleglod/cafe-crypto-backend:build-oqs`: Build environment with Open Quantum Safe (OQS) library (liboqs), OpenSSL with oqs-provider, and Go runtime
- `oleglod/cafe-crypto-backend:runtime-oqs`: Minimal runtime image with OQS support

**Note**: 
- The OQS Docker images are built and published from `cafe-crypto-backend`
- This step only needs to be done once, or when you need to update the OQS libraries
- For detailed OQS build instructions, see [cafe-crypto-backend/README.md](../cafe-crypto-backend/README.md)

### Step 2: Start Infrastructure Services

The infrastructure is managed in the `cafe-infra` [cafe-infra](https://github.com/kantika-tech/cafe-infra) repository.
Please refer to it.

For reference, the infrastructure is as follows:
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

**Local development (Docker Compose):**
```bash
# Set required environment variables (optional - can also be set in config.yaml)
export JWT_SECRET=your-secret-key-here

# Build and start services (local use only; staging/prod are deployed from cafe-deploy)
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d --build

# Or start individually
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d cafe-discovery-backend
```

**Docker Compose Configuration (local use only):**

The project uses a two-file Docker Compose setup for local development:

- **`docker-compose.yml`**: Base configuration
  - Contains service definitions, networks, volumes
  - No build contexts, no exposed ports
  - Uses environment variables for configuration

- **`docker-compose.dev.yml`**: Local development overrides
  - Adds build contexts for local development
  - Exposes port `8080` for backend API access
  - Builds images locally using `Dockerfile`

**Services:**

1. **`cafe-discovery-backend`**:
   - API server (default port `8080` internally)
   - Uses `runtime` target from `Dockerfile`
   - Health check: `curl http://localhost:8080/health` (every 30s)
   - Restart policy: `unless-stopped`
   - Exposes `/version` endpoint for version information
   - At startup waits for `persistence.ready` (and scanner heartbeats) before seeding default endpoints

2. **Persistence service** (cafe-persistence; deployed via cafe-deploy):
   - Single writer for scan lifecycle: subscribes to `scan.started`, `scan.completed`, `scan.failed`; writes to PostgreSQL and Redis; publishes `scan.ready` and `persistence.ready`
   - Image `oleglod/cafe-persistence`. Run it so the backend can complete startup and so GET requests return results after scans complete.

3. **Scanners**: externalized to dedicated repositories and deployed from `cafe-deploy`:
   - TLS scanner: `cafe-scanner-tls`
   - Wallet scanner: `cafe-scanner-wallet`

**Configuration:**

The services are configured with:
- **Network**: Connects to external network `cafe-infra_observability` (must exist from `cafe-infra`)
- **Volumes**: Mounts `./config.yaml` to `/app/config.yaml` (read-only)
- **Environment Variables**: Supports environment variable overrides with defaults:
  - `JWT_SECRET` (default: `change-me-for-local`)
  - `POSTGRES_USER` (default: `cafe`)
  - `POSTGRES_PASSWORD` (default: `cafe`)
  - `LOG_LEVEL` (default: `debug` for backend, `info` for scanner)
  - `TURNSTILE_SECRET_KEY` and `TURNSTILE_SITE_KEY` (default: dev keys)
- **Service Discovery**: Uses Docker service names (postgres, nats, redis) from `cafe-infra`
- **Health Checks**: Both services include health check configurations for monitoring

**Dockerfile Structure:**
- **OQS Base Image**: Managed in [cafe-infra/oqs](../cafe-infra/oqs/) - builds `cafe-oqs:build` and `cafe-oqs:runtime`, tagged as `oqs:dev` for compatibility
- `Dockerfile`: Builds the API server using `oqs:dev` as base
  - `runtime` target: Server image (used by cafe-deploy for staging/prod)
  - `ci` target: CI/CD image with linting and testing tools
- Scanner images are built in dedicated repositories:
  - TLS scanner: `cafe-scanner-tls`
  - Wallet scanner: `cafe-scanner-wallet`

**Verify services are running:**
```bash
# Check container status
docker compose ps

# Health check (backend)
curl http://localhost:8080/health


# Metrics endpoint (Prometheus format)
curl http://localhost:8080/metrics

# View logs
docker compose logs -f cafe-discovery-backend
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
  -e POSTGRES_HOST=postgres \
  -e POSTGRES_PORT=5432 \
  -e POSTGRES_DATABASE=cafe \
  -e POSTGRES_USER=cafe \
  -e POSTGRES_PASSWORD=cafe \
  -e NATS_URL=nats://nats:4222 \
  -e REDIS_URL=redis://redis:6379 \
  cafe-discovery-backend:latest
```

**Start the Wallet scanner** (`cafe-scanner-wallet`; requires Moralis or future Etherscan API key):
```bash
docker run --network cafe-infra_observability --rm \
  -p 8082:8081 \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  -e CONFIG_PATH=/app/config.yaml \
  -e DISCOVERY_SCANNER_TYPE=wallet \
  -e SCANNER_HEALTH_PORT=8081 \
  -e LOG_LEVEL=info \
  -e MORALIS_API_KEY=your-api-key-here \
  -e POSTGRES_HOST=postgres \
  -e POSTGRES_PORT=5432 \
  -e POSTGRES_DATABASE=cafe \
  -e POSTGRES_USER=cafe \
  -e POSTGRES_PASSWORD=cafe \
  -e NATS_URL=nats://nats:4222 \
  -e REDIS_URL=redis://redis:6379 \
  oleglod/cafe-scanner-wallet:latest
```

**Note:** 
- Replace image names with the actual tags you built (e.g. `oleglod/cafe-discovery-backend:latest`, `oleglod/cafe-scanner-tls:latest`, `oleglod/cafe-scanner-wallet:latest`)
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

# Scanner type: "tls" | "wallet" | "all" (default). Set "tls" or "wallet" when running separate scanner containers.
export DISCOVERY_SCANNER_TYPE=all

# Worker health check port
export SCANNER_HEALTH_PORT=8081

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

# Cloudflare Turnstile (required for signup/signin protection)
# Development keys are configured by default (always pass verification)
# Development keys (default):
#   Site Key: 1x00000000000000000000AA
#   Secret Key: 1x0000000000000000000000000000000AA
# For staging/production (cafe-deploy), get your keys from https://developers.cloudflare.com/turnstile/
# Note: The service will log a warning when using development keys
export TURNSTILE_SECRET_KEY=1x0000000000000000000000000000000AA  # Dev key (default)
export TURNSTILE_SITE_KEY=1x00000000000000000000AA  # Dev key (default)

# Logging
export LOG_LEVEL=info  # Options: trace, debug, info, warn, error, fatal, panic

# CORS configuration
export CORS_ALLOW_ORIGINS="http://localhost:3000,http://localhost:3001,http://localhost:5173"
export CORS_ALLOW_METHODS="GET,POST,PUT,DELETE,OPTIONS"
```

#### Discovery → CPM internal policy reference (WORKPLAN PR5 / PR6)

CPM (`cafe-crypto-policy-mgt`, **PR5**) exposes **`POST /internal/policies/references/scan`**, gated by **`CAFE_POLICY_REFERENCE_INTERNAL_SERVICE_TOKEN`** (Bearer). When **PR6** lands in Discovery, this service will call CPM over the **Docker/service network** (e.g. `http://cafe-cpm:8080`, not the browser-facing **`/api`** edge); the outbound **`Authorization: Bearer`** must match that secret. Until then, only CPM needs the variable — see **`cafe-deploy`** `compose/25-cpm.yml` and `env/dev.env.template`. Central reference: **`cafe-documentation/docs/security/cpm-auth-only-contract.md`** (§9–§10).

Using config.yaml vs Environment Variables:

- For local Docker Compose: Use `config.yaml` with Docker service names (postgres, nats, redis)
- For staging/production (cafe-deploy): Use environment variables or a secrets management system

### Starting in debug mode

Log levels define how detailed the logs should be.

Available log levels:
- `trace`: all logs
- `debug`: debug level and above
- `info`: default level and above
- `warn`: warnings and above
- `error`: errors and above
- `fatal`: fatal errors and above
- `panic`: panic level only

Example:

```bash
# Terminal 1 - Server in debug mode
export LOG_LEVEL=debug
go run cmd/server/main.go

# Terminal 2 - Worker in debug mode
export LOG_LEVEL=debug
go run cmd/scanner/main.go
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

# 4. Check scanners
curl http://localhost:8081/health   # TLS scanner

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

1. Key Storage: Server private keys are stored in memory. For staging/production:
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

2. Run a test HTTPS server (e.g. using [cafe-crypto-backend](https://github.com/create2-labs/cafe-crypto-backend) runtime image with OpenSSL OQS, or a local server with PQC support)

3. Scan with the API:
```bash
curl -X POST http://localhost:8080/discovery/v1/scan \
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

Note: The `turnstile_token` is generated by the Cloudflare Turnstile widget on the frontend. By default, the service uses Cloudflare's free development keys which always pass verification. The service will log a warning when using development keys. For staging/production (cafe-deploy), configure production keys from your Cloudflare dashboard.

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

Note: The `turnstile_token` is generated by the Cloudflare Turnstile widget on the frontend. By default, the service uses Cloudflare's free development keys which always pass verification. The service will log a warning when using development keys. For staging/production (cafe-deploy), configure production keys from your Cloudflare dashboard.

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

### POST /discovery/v1/scan

Unified scan request endpoint for wallet and TLS scans. Requires authentication. The scan is processed asynchronously via NATS and returns a `scan_id` immediately; clients use that `scan_id` with the wallet or TLS detail routes.

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
  "scan_id": "550e8400-e29b-41d4-a716-446655440000",
  "scan_family": "wallet",
  "status": "requested",
  "location": "/api/discovery/v1/wallets/scans/550e8400-e29b-41d4-a716-446655440000"
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
  "scan_id": "660e8400-e29b-41d4-a716-446655440000",
  "scan_family": "tls",
  "status": "requested",
  "location": "/api/discovery/v1/tls/scans/660e8400-e29b-41d4-a716-446655440000"
}
```

Note: The endpoint detects the scan family based on the provided field (`address` for wallets, `url` for TLS endpoints). You cannot specify both fields in the same request.

| Layer | Path |
|-------|------|
| Discovery backend (Fiber) | `POST /discovery/v1/scan` |
| Edge / frontend / scripts | `POST /api/discovery/v1/scan` |

### GET /discovery/v1/wallets/scans

Returns a paginated list of wallet scan summaries for the authenticated user. List responses are intentionally lightweight; use `scan_id` with `GET /discovery/v1/wallets/scans/:scan_id` for detail.

Query Parameters:
- `limit` (optional): Number of results per page (default: 20)
- `offset` (optional): Number of results to skip (default: 0)
- `address` (optional): Filter by wallet address
- `chain_id` (optional): Filter by chain ID; requires `address`

Response:
```json
{
  "items": [
    {
      "scan_id": "550e8400-e29b-41d4-a716-446655440000",
      "target_address": "0x742d35cc6634c0532925a3b844bc454e4438f44e",
      "chain_ids": [1, 137],
      "status": "completed",
      "created_at": "2025-01-15T10:30:00Z"
    }
  ],
  "total": 1,
  "limit": 20,
  "offset": 0
}
```

### GET /discovery/v1/wallets/scans/:scan_id

Returns the wallet scan detail for a single `scan_id`. Pending scans return only `scan_id` and `status`; terminal scans include a `result` object.

Response:
```json
{
  "scan_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "completed",
  "result": {
    "target_address": "0x742d35cc6634c0532925a3b844bc454e4438f44e",
    "chain_ids": [1, 137],
    "wallet_type": "eoa",
    "current_pq_posture": "not_pq_ready",
    "algorithm": "ECDSA-secp256k1",
    "nist_level": 1,
    "risk_score": 0.85,
    "key_exposed": true,
    "networks": ["ethereum-mainnet", "polygon"],
    "scanned_at": "2025-01-15T10:30:00Z"
  }
}
```

Example:
```bash
curl -X GET "http://localhost:8080/discovery/v1/wallets/scans/550e8400-e29b-41d4-a716-446655440000" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

Address contract note: wallet addresses are accepted in any valid EVM hex casing, and Discovery returns canonical lowercase addresses in machine-oriented API fields.

### GET /discovery/v1/wallets/scans/:scan_id/cbom

Returns a CycloneDX v1.7 CBOM envelope for **this** `scan_id`, generated on read from persisted scan fields (not stored as a blob). Available only when the scan is **`completed` success** (**G4**, IMM-6b-7). Any other lifecycle state (`requested`, `started`, `failed`, `plan_limit_exceeded`) returns **404** `not_found`.

OpenAPI: [`openapi/discovery-v1.yaml`](openapi/discovery-v1.yaml) (`getWalletScanCbom`).

Example:
```bash
curl -X GET "http://localhost:8080/discovery/v1/wallets/scans/550e8400-e29b-41d4-a716-446655440000/cbom" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

### GET /discovery/v1/tls/scans

Returns a paginated list of TLS scan summaries for the authenticated user. Use `scan_id` with `GET /discovery/v1/tls/scans/:scan_id` for detail.

Query Parameters:
- `limit` (optional): Number of results per page (default: 20)
- `offset` (optional): Number of results to skip (default: 0)

Response:
```json
{
  "items": [
    {
      "scan_id": "660e8400-e29b-41d4-a716-446655440000",
      "endpoint": "https://example.com",
      "status": "completed",
      "created_at": "2025-01-15T10:30:00Z"
    }
  ],
  "total": 1,
  "limit": 20,
  "offset": 0
}
```

Example:
```bash
curl -X GET "http://localhost:8080/discovery/v1/tls/scans?limit=10&offset=0" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

### GET /discovery/v1/tls/scans/defaults

Returns the shared catalog of default TLS endpoint scans. Requires authentication.

Response:
```json
{
  "items": [
    {
      "scan_id": "770e8400-e29b-41d4-a716-446655440000",
      "endpoint": "https://example.com",
      "status": "completed",
      "created_at": "2025-01-15T10:30:00Z",
      "is_default": true
    }
  ],
  "total": 1,
  "limit": 1,
  "offset": 0
}
```

### GET /discovery/v1/tls/scans/:scan_id

Returns the TLS scan detail for a single `scan_id`. Pending scans return only `scan_id` and `status`; terminal scans include a `result` object. Default endpoint scans are visible to authenticated users and include `is_default: true`.

Response:
```json
{
  "scan_id": "660e8400-e29b-41d4-a716-446655440000",
  "status": "completed",
  "result": {
    "endpoint": "https://example.com",
    "tls_version": "TLS 1.3",
    "cipher_suite": "TLS_AES_256_GCM_SHA384",
    "key_exchange": "X25519",
    "current_pq_posture": "not_pq_ready",
    "url": "https://example.com",
    "host": "example.com",
    "port": 443,
    "nist_level": 1,
    "risk_score": 0.75,
    "pqc_risk": "critical",
    "pqc_mode": "classical",
    "supported_pqc": [],
    "recommendations": ["Upgrade to PQC certificates"],
    "scanned_at": "2025-01-15T10:30:00Z"
  }
}
```

Example:
```bash
curl -X GET "http://localhost:8080/discovery/v1/tls/scans/660e8400-e29b-41d4-a716-446655440000" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

### Option A: Discovery wallet scan v1 ↔ CPM contract

**Option A** is the post-V1 CPM integration path: real user-owned wallet scans via the authenticated Discovery backend (not mock scan placeholders or direct DB access from CPM/frontend). Product definition and Option A vs Option B: **[cafe-crypto-policy-mgt `workplans/CPM_post_v_1_option_a_scan_context.md`](https://github.com/create2-labs/cafe-crypto-policy-mgt/blob/main/workplans/CPM_post_v_1_option_a_scan_context.md)**.

Maintainer reference for HTTP/mapping (list + detail under **`/discovery/v1/wallets/scans`**, CPM explore with **`policy_context`**, persist / list by **`scan_id`**, async assessment without client **`policy_context`**): **[docs/CPM_OPTION_A_DISCOVERY_V1_CONTRACT.md](docs/CPM_OPTION_A_DISCOVERY_V1_CONTRACT.md)** — URL matrix, flow summary, and normative **§3.1** mapping from **`WalletScanDetail`** into CPM explore (cross‑checked with **`cafe-crypto-policy-mgt/internal/api/explore_policy_context.go`**).

#### Scan immutability & per-execution history (maintainers)

Product invariants (**`scan_id`** stable per row, immutable terminal **`result`**, multi-row history per address, **W1–W8** with CPM) are defined in **[`cafe-crypto-policy-mgt/workplans/WORKPLAN_API.md` §2.2](https://github.com/create2-labs/cafe-crypto-policy-mgt/blob/main/workplans/WORKPLAN_API.md)** (see also **§4.2.1** for list envelopes).

Implementation is split across PRs **IMM-1…IMM-12** in **[`IMMUTABILITE_PR.md`](IMMUTABILITE_PR.md)**. Gap analysis, **no-backfill** data policy, Redis vs Postgres roles, and deployment ordering: **[`docs/SCAN_IMMUTABILITY_MIGRATION.md`](docs/SCAN_IMMUTABILITY_MIGRATION.md)** (IMM-1). Start **IMM-2** only after that document is reviewed.

**IMM-6b smoke scripts** (plan quota ledger, guards, usage API, integration tests) live in sibling repo **`cafe-deploy/scripts/`** — not run by default; pass explicit modes: `--software` (go test + vet + vuln + lint), `--postgres`, `--api`, or `--all`. Suite: `test-discovery-imm6b-all.sh` (includes **IMM-6b-8** `test-discovery-imm6b8-plan-quota-integration.sh`). IMM-6b-6 ledger backfill was cancelled (no prod data; DB reset). Persistence quota integration tests: **cafe-persistence** (`internal/persistence/*`); Discovery handler/service/repository tests remain in-repo. See **`cafe-deploy/README.md`** § *Discovery/CPM smoke scripts*.

### Policy assessment (CPM-owned)

Policy assessment HTTP is **not** served by Discovery. Use **Crypto Policy Management (CPM)**:

- **Edge (browser / gateway):** `POST /api/cpm/v1/policies/assessment/request`
- **CPM backend (in-process):** `POST /cpm/v1/policies/assessment/request` (after ingress strips `/api`)

Discovery still publishes wallet **observations** on the bus; CPM owns the explicit `policy.assessment.requested.v0.1` command (see `cafe-crypto-policy-mgt` and WORKPLAN_API_PR **PR13g**).

### GET /discovery/v1/rpcs

Returns the list of configured RPC endpoints. **No authentication required.**

| Layer | Path |
|-------|------|
| Discovery backend (Fiber) | `GET /discovery/v1/rpcs` |
| Edge / frontend / scripts | `GET /api/discovery/v1/rpcs` |

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

Example (direct backend):
```bash
curl -sS "http://localhost:8080/discovery/v1/rpcs" | jq .
```

Example (via edge):
```bash
curl -sS "https://localhost/api/discovery/v1/rpcs" | jq .
```

### GET /discovery/v1/scanners

Returns the list of scanner types currently available (scanners that have announced their presence via NATS). Useful for monitoring and to know which scan types (wallet, TLS) can be processed. **No authentication required.**

| Layer | Path |
|-------|------|
| Discovery backend (Fiber) | `GET /discovery/v1/scanners` |
| Edge / frontend / scripts | `GET /api/discovery/v1/scanners` |

**Response**:
```json
{
  "scanners": [
    {
      "type": "tls",
      "count": 2,
      "ids": ["uuid-1", "uuid-2"]
    },
    {
      "type": "wallet",
      "count": 1,
      "ids": ["uuid-3"]
    }
  ]
}
```

- `type`: Scanner type (`tls` or `wallet`).
- `count`: Number of scanner instances currently available for this type.
- `ids`: List of scanner instance IDs (for debugging/ops).

Example (direct backend):
```bash
curl -sS "http://localhost:8080/discovery/v1/scanners" | jq .
```

Example (via edge):
```bash
curl -sS "https://localhost/api/discovery/v1/scanners" | jq .
```

### GET /version

Get the backend version information.

**Authentication**: Not required

**Response**:
```json
{
  "version": "v1.2.3"
}
```

The version is embedded at Docker RC build time via `-ldflags` (`internal/version`) from the `APP_VERSION` build argument, with optional runtime override via the `APP_VERSION` environment variable. Docker Release does not change this value.

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

### AUTH-05: Internal scan authorization lookup for CPM

Discovery exposes an **internal-only** authorization endpoint that the Crypto Policy Management service (CPM) calls to determine whether an authenticated principal may read or use a given scan. This is the Discovery-side counterpart of CPM AUTH-02: CPM authenticates the caller (AUTH-01) and **delegates scan visibility decisions to Discovery**. **Discovery remains the authoritative source for scan visibility. CPM authenticates the caller and delegates scan visibility decisions to Discovery. CPM must not read Discovery persistence directly.**

**Endpoint:** `POST /internal/authz/scans/{scanId}/can-read`

**Privacy & isolation:**

- The endpoint is internal-only. It must not be exposed through public ingress; the production deployment routes only known service callers (currently CPM) to this path.
- The response envelope is intentionally minimal: it contains only `allowed`, `reason_code`, and `request_id`. Discovery never returns the scan owner, tenant id, wallet address, endpoint URL, scan target, email, or any other scan attribute on this endpoint. Deny responses leak nothing about whether the scan exists.
- Service credentials, raw session tokens, and the `Authorization` header are never logged. Logs include `request_id`, `route`, `outcome`, `reason_code`, `user_id`, and `tenant_id`; they do not include scan metadata or request bodies.

**Required headers:**

| Header | Required | Notes |
| --- | --- | --- |
| `Authorization: Bearer <service-token>` | yes | Static internal service token configured via `DISCOVERY_INTERNAL_AUTHZ_SERVICE_TOKEN`. **Temporary** until mTLS or a signed service JWT is available; comparison is constant-time. |
| `X-User-Id` | yes | The authenticated principal id propagated by CPM from its session. **Discovery only trusts this header after the service-auth check passes.** Missing header returns `401 SCAN_AUTHZ_PRINCIPAL_REQUIRED`. |
| `X-Tenant-Id` | optional | Propagated by CPM when present. Currently informational because the Discovery scan model has no tenant column; a TODO tracks future enforcement (`AUTH-05 tenant scoping`). |
| `X-Request-Id` | optional | Sanitized and echoed in both the response body (`request_id`) and the `X-Request-Id` response header. When missing, Discovery generates a random opaque id so logs and responses always carry one. |

The endpoint expects an empty body; the `scanId` is taken from the URL path.

**Reason codes and HTTP status mapping:**

| Outcome | HTTP | `reason_code` | Meaning |
| --- | --- | --- | --- |
| Allowed | `200` | `SCAN_AUTHZ_ALLOWED` | The principal owns or otherwise has visibility on the scan. |
| Denied (cross-user / not authorized) | `403` | `SCAN_AUTHZ_FORBIDDEN` | The scan exists but the principal is not allowed to read it. |
| Denied (scan not visible / unknown) | `403` | `SCAN_AUTHZ_NOT_VISIBLE` | The scan does not exist or is not visible. Returned as `403` for this rollout to align with CPM AUTH-02. Anti-enumeration `404` hardening is deferred to a later PR. |
| Malformed scan id | `400` | `SCAN_AUTHZ_SCAN_ID_MALFORMED` | The path segment is not a valid identifier (UUID). |
| Missing principal | `401` | `SCAN_AUTHZ_PRINCIPAL_REQUIRED` | `X-User-Id` is absent or unusable. |
| Missing/invalid service auth | `401` | `SCAN_AUTHZ_SERVICE_AUTH_REQUIRED` | The `Authorization` bearer token is missing, malformed, or does not match `DISCOVERY_INTERNAL_AUTHZ_SERVICE_TOKEN`. |
| Endpoint disabled | `503` | `SCAN_AUTHZ_DISABLED` | `DISCOVERY_INTERNAL_AUTHZ_ENABLED=false`. CPM is expected to fail closed. |
| Decision unavailable | `503` | `SCAN_AUTHZ_UNAVAILABLE` | The repository or downstream lookup failed. CPM is expected to fail closed. |

**Response examples:**

Allow:

```json
{
  "allowed": true,
  "reason_code": "SCAN_AUTHZ_ALLOWED",
  "request_id": "trace-abc-123"
}
```

Deny (forbidden / not visible — both shapes are identical except for `reason_code`):

```json
{
  "allowed": false,
  "reason_code": "SCAN_AUTHZ_FORBIDDEN",
  "request_id": "trace-abc-123"
}
```

Decision unavailable:

```json
{
  "allowed": false,
  "reason_code": "SCAN_AUTHZ_UNAVAILABLE",
  "request_id": "trace-abc-123"
}
```

**Fail-closed semantics (CPM AUTH-02 perspective):**

- `200 + allowed=true` is the only success signal. Any other response (including transport errors) **must be treated by CPM as a deny** for the requested action.
- `5xx` responses indicate that the decision could not be resolved. CPM fails closed (returns `503` to its caller) and does not cache the outcome.
- `403` is the canonical deny for both "forbidden" and "not visible" in this rollout. CPM does not differentiate; the reason code is provided for traceability and metrics.

**Authorization rule (current):**

- A principal can read a scan if and only if Discovery's authoritative scan model says so:
  - Wallet scans: `scan.user_id == principal.user_id`.
  - TLS scans: `scan.user_id == principal.user_id`, plus default endpoints (`scan.default = true`) which are visible to any authenticated principal.
- The `X-Tenant-Id` header is propagated to the decision service; Discovery's scan model has no tenant column today, so tenant scoping is currently a no-op. A `TODO(auth-05-tenant)` in `internal/service/scan_authz.go` tracks adding the comparison once the model carries `tenant_id`.

**Configuration:**

```bash
# default true; when false, the endpoint replies 503 SCAN_AUTHZ_DISABLED so CPM fails closed.
export DISCOVERY_INTERNAL_AUTHZ_ENABLED=true

# REQUIRED in any environment that exposes the endpoint to CPM. Treat as a secret.
# TODO: replace this static token with mTLS or a signed service JWT.
export DISCOVERY_INTERNAL_AUTHZ_SERVICE_TOKEN=<shared-secret-with-cpm>
```

**Observability:**

- Counter `discovery_scan_authz_decisions_total{outcome,reason_code,route}` records every decision. Labels are deliberately low-cardinality; `user_id`, `tenant_id`, `scan_id`, and `request_id` are never used as labels.
- Structured logs are emitted at `info` for `allowed`/`denied` and `warn` for `malformed`/`unavailable`, carrying `request_id`, `route`, `outcome`, `reason_code`, `user_id`, and `tenant_id`. Service tokens, session tokens, scan metadata, emails, and request bodies are never logged.

**Relationship to CPM AUTH-02:**

CPM's scan-authorization adapter (`POST <ScanAuthorizationURL>/{scanId}/can-read`) is wired to this endpoint. CPM:

1. Authenticates the bearer token from its own caller.
2. Extracts `scanId` from the request payload (selection, validation, wallet challenge, draft save, persist, etc.).
3. Calls Discovery with the propagated `X-User-Id`, optional `X-Tenant-Id`, and `X-Request-Id`, plus the configured service bearer token.
4. Maps the response to its own `403`/`503` outcomes per AUTH-02.

This endpoint is the only sanctioned integration mode between CPM and Discovery for scan visibility; CPM **must not** read Discovery's PostgreSQL or Redis directly.

## Subscription Plans

The service supports subscription plans with usage limits for wallet and TLS endpoint scans.

### Available Plans

1. **Free Plan**:
   - Wallet scans: 5 per time period
   - TLS endpoint scans: 5 per time period
   - Price: Free
   - Status: Active

2. **CAFEIN Premium Plan**:
   - Wallet scans: Unlimited
   - TLS endpoint scans: Unlimited
   - Price: $29.99/month
   - Status: Coming soon (currently inactive)

### Plan Management Endpoints

#### GET /plans

Get all available subscription plans.

**Authentication**: Required (JWT token)

**Response**:
```json
[
  {
    "id": "uuid",
    "name": "Free Plan",
    "type": "FREE",
    "wallet_scan_limit": 5,
    "endpoint_scan_limit": 5,
    "price": 0,
    "is_active": true
  },
  {
    "id": "uuid",
    "name": "CAFEIN Premium Plan",
    "type": "PREMIUM",
    "wallet_scan_limit": 0,
    "endpoint_scan_limit": 0,
    "price": 29.99,
    "is_active": false
  }
]
```

**Note**: `wallet_scan_limit` and `endpoint_scan_limit` of `0` indicate unlimited scans.

#### GET /plans/current

Get the current user's subscription plan.

**Authentication**: Required (JWT token)

**Response**:
```json
{
  "id": "uuid",
  "name": "Free Plan",
  "type": "FREE",
  "wallet_scan_limit": 5,
  "endpoint_scan_limit": 5,
  "price": 0,
  "is_active": true
}
```

#### GET /plans/usage

Get current usage statistics for the authenticated user. Counts follow the **success-only ledger** (IMM-6b P1): `used` never decreases when a user deletes a scan from history. Ledger rows are written on each successful scan completion (**IMM-6b-4**).

**Authentication**: Required (JWT token)

**Response**:
```json
{
  "wallet_scans_used": 3,
  "wallet_scans_visible": 2,
  "wallet_scans_deleted_by_user": 1,
  "wallet_scans_in_flight": 0,
  "wallet_scan_limit": 5,
  "endpoint_scans_used": 2,
  "endpoint_scans_visible": 2,
  "endpoint_scans_deleted_by_user": 0,
  "endpoint_scans_in_flight": 1,
  "endpoint_scan_limit": 5,
  "wallet_scans_left": 2,
  "endpoint_scans_left": 3
}
```

| Field | Meaning |
|-------|---------|
| `wallet_scans_used` / `endpoint_scans_used` | Successful scans counted in the append-only ledger |
| `wallet_scans_visible` / `endpoint_scans_visible` | Active (non soft-deleted) success rows in Postgres |
| `wallet_scans_deleted_by_user` / `endpoint_scans_deleted_by_user` | `used − visible` — successes hidden by user DELETE |
| `wallet_scans_in_flight` / `endpoint_scans_in_flight` | Scans not yet terminal (PENDING/RUNNING); omitted when zero |
| `wallet_scans_left` / `endpoint_scans_left` | Remaining quota slots based on ledger `used`; `-1` if unlimited |

### Plan Enforcement

- **All API access requires authentication.** Plan limits are enforced based on the authenticated user's assigned plan.
- **Unlimited plans**: Plans with `wallet_scan_limit` or `endpoint_scan_limit` of `0` have no restrictions

### Worker Health Check

The scanner exposes a health check endpoint on port `8081` (configurable via `SCANNER_HEALTH_PORT`).

Endpoint: `GET http://localhost:8081/health`

Response (healthy, both scanners running):
```json
{
  "status": "ok",
  "app_name": "Cafe Discovery Worker",
  "timestamp": "2025-01-15T10:30:00Z",
  "checks": {
    "nats": { "connected": true },
    "scanners": {
      "wallet": { "running": true },
      "tls": { "running": true }
    }
  }
}
```

When running with `DISCOVERY_SCANNER_TYPE=tls` or `DISCOVERY_SCANNER_TYPE=wallet`, only the corresponding scanner key appears under `checks.scanners`.

Response (degraded):
Returns HTTP 503 when NATS is disconnected or the started scanner(s) are not running.

## Testing

### 1. Register and Authenticate

Note: The signup and signin endpoints require a Cloudflare Turnstile token. By default, the service uses Cloudflare's free development keys which always pass verification. The service will log a warning when using development keys. For staging/production (cafe-deploy), configure production keys from your Cloudflare dashboard.

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

The `/discovery/v1/scan` endpoint automatically detects whether you're scanning a wallet or TLS endpoint and returns a `scan_id` for follow-up detail requests:

```bash
# Queue a wallet scan (automatically detected from "address" field)
curl -X POST http://localhost:8080/discovery/v1/scan \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"address": "0x13f735c915bba9136Db794F6b1f42566B24861B8"}'

# Queue a TLS endpoint scan (automatically detected from "url" field)
curl -X POST http://localhost:8080/discovery/v1/scan \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"url": "https://example.com"}'

# Queue a TLS scan with custom port (e.g., 8443)
curl -X POST http://localhost:8080/discovery/v1/scan \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"url": "https://localhost:8443"}'
```

### 3. List Scan Summaries

The list endpoints return lightweight scan summaries. Use the returned `scan_id` with the corresponding detail endpoint.

```bash
# List wallet scan summaries
curl -X GET "http://localhost:8080/discovery/v1/wallets/scans?limit=10&offset=0" \
  -H "Authorization: Bearer $TOKEN" | jq .

# List TLS scan summaries
curl -X GET "http://localhost:8080/discovery/v1/tls/scans?limit=10&offset=0" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

### 4. Retrieve Scan Details by scan_id

Use wallet scan IDs with `/discovery/v1/wallets/scans/:scan_id` and TLS scan IDs with `/discovery/v1/tls/scans/:scan_id`.

```bash
# Fetch wallet scan detail
curl -X GET "http://localhost:8080/discovery/v1/wallets/scans/550e8400-e29b-41d4-a716-446655440000" \
  -H "Authorization: Bearer $TOKEN" | jq .

# Fetch TLS scan detail
curl -X GET "http://localhost:8080/discovery/v1/tls/scans/660e8400-e29b-41d4-a716-446655440000" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

Pending scans return `{"scan_id": "...", "status": "requested"}`. Terminal scans include a `result` object with wallet or TLS discovery fields.

### 5. Public Endpoints

```bash
# List configured RPC endpoints (no auth required; backend path)
curl http://localhost:8080/discovery/v1/rpcs

# List available scanners (no auth required; backend path)
curl http://localhost:8080/discovery/v1/scanners

# Health check (no auth required)
curl http://localhost:8080/health

# Prometheus metrics (no auth required)
curl http://localhost:8080/metrics

# Scanner health check (no auth required)
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

Metrics are automatically recorded on scan lifecycle processing paths.

### Prometheus Configuration

The infrastructure stack in `cafe-infra` includes Prometheus configured to scrape the `/metrics` endpoint. 

For local Docker Compose, Prometheus in `cafe-infra` is already configured to scrape the discovery service. The configuration uses the Docker service name:

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
- For staging/production (deployed from cafe-deploy), use the appropriate service discovery there.

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

- **Wallet scans**: API publishes to `cafe.discovery.wallet.scan`; the Wallet scanner (plugin) consumes messages, decodes with `plugin.DecodeMessage`, runs the scan with `plugin.Run`, and persists results.
- **TLS scans**: API publishes to `cafe.discovery.tls.scan`; the TLS scanner (plugin) does the same. TLS scanning uses OQS for PQC support.
- **Scalability**: scanner images are produced by dedicated repositories (`cafe-scanner-tls`, `cafe-scanner-wallet`).

## Development Tools

### Wallet public-key CLIs (moved to `cafe-scanner-wallet`)

Wallet recovery / scan CLIs no longer live in this repo. Use:

```bash
# RPC + transaction hash (no Moralis)
cd ../cafe-scanner-wallet
go run ./cmd/cli/wallet-scan <rpc-url> <tx-hash>

# Address-based (Moralis + WalletScanEngine)
export MORALIS_API_KEY=your_api_key_here
go run ./cmd/cli/publickey --address 0x...
```

See [`cafe-scanner-wallet` README](https://github.com/create2-labs/cafe-scanner-wallet) and `cmd/cli/*/README.md` there. TLS/OQS CLI docs remain under `cmd/cli/tls-scan/` until PR5.

## Security Notes

⚠️ Important: Never commit API keys or sensitive credentials to version control. Always use environment variables or secure configuration management:

- Use environment variables for all API keys
- Never hardcode credentials in source code
- Use `.env` files (and add them to `.gitignore`) for local development
- Use secret management for staging/production (cafe-deploy)

## Stopping Discovery services

To stop all services:

```bash
docker compose down
```

## Additional Resources

- [CAFE functional specifications](https://github.com/create2-labs/cafe-documentation/blob/main/functional-specifications.md) — product behavior (English)
- [CAFE technical specifications](https://github.com/create2-labs/cafe-documentation/blob/main/technical-specifications.md) — architecture and testing (English)
- [Option A: Discovery v1 wallet scans ↔ CPM](docs/CPM_OPTION_A_DISCOVERY_V1_CONTRACT.md) — contract reference for **`policy_context`** and related CPM routes
- [Scan immutability & migration strategy (IMM-1)](docs/SCAN_IMMUTABILITY_MIGRATION.md) — gap vs **WORKPLAN_API.md** §2.2, rollout **IMM-2…IMM-8**
- [Scan immutability PR plan](IMMUTABILITE_PR.md) — per-PR branches and acceptance criteria
- [Post-Quantum JWT Documentation](docs/PQC_JWT.md) - Detailed guide on PQC JWT implementation
- [PQC Certificate Generation Guide](docs/PQC_CERTIFICATES.md) - Guide for generating and testing PQC TLS certificates
- [Open Quantum Safe](https://openquantumsafe.org/) - Official OQS project
- [NIST PQC Standards](https://csrc.nist.gov/projects/post-quantum-cryptography) - NIST post-quantum cryptography standards
