# Cafe Discovery Service

A Discovery service for identifying cryptographic exposures and quantum vulnerabilities on the Ethereum network and related infrastructure.

## Features

- **Wallet Scanning**: Scan wallets across multiple EVM-compatible networks
- **Key Exposure Detection**: Detect whether a wallet's public key has been revealed on-chain
- **Account Type Detection**: Determine if an address is an EOA (Externally Owned Account) or AA (Abstract Account/ERC-4337)
- **Risk Assessment**: Calculate risk scores based on exposure across networks
- **Quantum Security Level**: Assess NIST quantum-security levels

## Architecture

The service follows Clean Architecture principles:

```
cafe-discovery/
├── cmd/server/          # Application entrypoint
├── internal/
│   ├── app/            # Application container (orchestration)
│   ├── domain/         # Domain models and types
│   ├── handler/        # HTTP handlers (Fiber)
│   └── service/        # Business logic
├── pkg/
│   └── evm/            # EVM client for blockchain interactions
└── configs/            # Configuration management
```

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

## Running the Service

1. Install dependencies:
```bash
go mod tidy
```

2. Run the server:
```bash
go run cmd/server/main.go
```

Or set a custom config path:
```bash
CONFIG_PATH=config.yaml go run cmd/server/main.go
```

The service will start on `http://localhost:8080` by default.

## API Endpoints

### POST /discovery/scan

Scans a wallet address across all configured networks.

**Request:**
```json
{
  "address": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
}
```

**Response:**
```json
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
  "connections": []
}
```

Note: The `connections` field is reserved for future use to show connected wallet addresses from transaction analysis.

### GET /discovery/rpcs

Returns the list of configured RPC endpoints.

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

Health check endpoint.

## Testing

Test the endpoints with curl:

```bash
# Scan a wallet
curl -X POST http://localhost:8080/discovery/scan \
  -H "Content-Type: application/json" \
  -d '{"address": "0x13f735c915bba9136Db794F6b1f42566B24861B8"}'

# List configured RPC endpoints
curl http://localhost:8080/discovery/rpcs

# Health check
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

