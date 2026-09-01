# CAFE Discovery Scan Refactoring Plan

> **Historical design plan, not current API reference.** This document is retained as design history for the scan plugin and file-scan ideas. The active HTTP contract is documented in `README.md` under `/discovery/v1`: scan requests use `POST /discovery/v1/scan`, list routes return scan summaries, and details are fetched by `scan_id` through `/discovery/v1/wallets/scans/:scan_id` or `/discovery/v1/tls/scans/:scan_id`. The file-scan route and CBOM historique concepts below are historical proposals, not active endpoints.

l'idee est de pouvoir ajouter, gérer et utiliser facilement differents scanners pour le discovery
jusque là on a deux scanners TLS et Wallet
on veut ajouter un scanner file
**Scope:** Discovery only (TLS, wallet, file). No policy or remediation. No migration logic. **Backward compatibility is not required** — nothing is in production yet; API and response shapes can be designed cleanly.

---

## 1. Proposed Updated Architecture Overview

### 1.1 High-Level Flow

**TLS and Wallet (backend-run scan):**

```
HTTP Request (body: address | url)
       │
       ▼
Handler: resolve kind → get Plugin → DecodeHTTP(body) → ScanTarget
       → check limits (PlanLimitKey) → publish NATS (plugin Subject)
       │
       ▼
NATS message (per-subject)
       │
       ▼
Worker: unmarshal → plugin.DecodeMessage(msg) → ScanTarget
       → shared helper (concurrency, logging, tracing)
       → plugin.Run(ctx, userID, ScanTarget, opts) → ScanResult
       → persist structured result (findings, classification, metadata) in Postgres
       │
       ▼
API: list = scan IDs only; GET CBOM by scan_id → load → scan.ScanResult → ToCBOM() on demand → JSON
```

**File (local-first, no backend scanning):**

```
Frontend (browser): user selects file
       │
       ▼
Frontend (WASM or JS): analyzes file locally — no file bytes sent to backend
       │
       ▼
Frontend: historical proposal, not active API: POST /discovery/v1/scan/file with body { file_sha256, findings, classification, [metadata] }
       │
       ▼
Backend handler: get File plugin → DecodeHTTP(body) → *FileTarget (no file data; file_sha256 + findings + classification)
       → check limits (PlanLimitKey = "file") → persist directly (synchronous — no NATS, no worker)
       → persist: file_sha256, findings, classification, metadata, trust_level = "self_reported", scanner_version, ruleset_version
       │
       ▼
API: list = scan IDs only; GET CBOM by scan_id → load → FileScanResult.ToCBOM() on demand → JSON
```

- **Backend never receives raw file bytes.** File analysis runs only in the frontend (WASM or JS). Backend receives only structured findings + `file_sha256`. FileTarget must not contain file content.
- **TLS / Wallet** → async via NATS and worker. **File** → synchronous persist only (no worker, no NATS). File scan is not heavy (just persist); no need for async, semaphore, or worker. Cleaner architecture.

- **ScanTarget** is the typed input to a scan (no raw string in the Runner).
- **ScanResult** is the unified output contract: ScanKind, ScannedAt, Findings, Classification, ToCBOM.
- **Persistence** is structured only (findings, classification, metadata). CBOM is never stored; it is generated on demand via `ToCBOM()`. List endpoints do **not** call ToCBOM() for each row — they return scan IDs only; full CBOM is fetched by scan_id (see §3.4).
- **Plan limits** use stable kind constants (`tls`, `wallet`, `file`); PlanService is refactored to be data-driven (no hardcoded switch on scan type).
- **Workers** remain one per NATS subject; common logic lives in a shared helper (semaphore, unmarshal error, logging, tracing, panic recovery).

### 1.2 Plugin Contract

Each discovery plugin provides:

- **PluginDescriptor**: Kind, Subject, PlanLimitKey, Capabilities, Version.
- **DecodeHTTP(body []byte)**: HTTP request body → plugin’s **ScanTarget**. Used by handlers only; API format is separate from NATS.
- **DecodeMessage(msg any)**: unmarshaled NATS message (e.g. `*WalletScanMessage`, `*TLSScanMessage`) → plugin’s **ScanTarget**. Used by workers only; no re-unmarshal, no mixing of API and message formats. Clear separation = clean architecture.
- **Run**: `Run(ctx, userID, target ScanTarget, opts RunOptions) (ScanResult, error)`. The **plugin** receives `ScanTarget` (concretely a pointer, e.g. `*TLSTarget`) and performs the type assertion internally (e.g. `target.(*TLSTarget)`); handler/worker never cast. No generics: Go cannot store heterogeneous `Runner[T]` in a single registry without losing type information and forcing type assertions at the call site.
- **Persistence**: plugin-specific repository that stores structured data (findings, classification, metadata). The repository returns entities that can be converted to a type implementing **scan.ScanResult** (pkg/scan) so that list/detail handlers can call `ToCBOM()`.
- **For File plugin only:** `Run()` does **not** perform analysis. `Run()` validates and persists client-reported findings only. No backend scanning; no worker.

### 1.3 Kind Constants and Billing

- **Stable kind constants** (used in API, plugin registry, and as PlanLimitKey):  
  `KindTLS = "tls"`, `KindWallet = "wallet"`, `KindFile = "file"`.
- **Internal naming** may differ where needed (e.g. DB column or plan field still named "endpoint" for TLS). Billing and external contract use the unified kind values.
- **PlanLimitKey** in PluginDescriptor must be one of these (e.g. TLS plugin uses `KindTLS` even if internally we say "endpoint").

### 1.4 Trust boundary: Discovery vs Remediation

- **Discovery** (this refactor): lightweight, developer-first. Identifies what crypto is present (TLS, wallet, or client-reported file findings). No policy enforcement, no transformation. File scans are **self-reported** (trust_level = `"self_reported"`); backend does not verify file contents.
- **Remediation**: Cifer-backed secure transformation (out of scope here). Where CAFE needs attested or verified actions (e.g. key rotation, re-encryption), that belongs in the Remediation layer, not in Discovery.
- **Do not** introduce TEE or attestation in the Discovery layer. Do not modify Remediation logic or add a policy layer in this refactor. The separation is intentional: **Discovery = what’s there; Remediation = what to do about it.** That separation is clean, credible, and scalable.

**Strategic point:** This actually makes CAFE stronger: **Discovery = lightweight, developer-first** (identify crypto in TLS, wallets, or client-reported files); **Remediation = Cifer-backed secure transformation** (when attested or verified actions are needed). Clear boundary, credible story, scalable.

### 1.5 File scan (local-first) and SHA binding

**SHA binding (advisory):** SHA256 is computed **locally (client) before analysis**. Findings are **logically bound** to that SHA in the stored record. Backend stores SHA + findings together. Backend does **not** verify that the SHA corresponds to actual file content — we do not prove that the findings correspond to the file. **Discovery is advisory.** If a reviewer asks “How do you prove the findings correspond to the file?”, the answer is: we don’t; trust_level is `"self_reported"`.

### 1.6 File scan constraints (summary)

- **File scanning does NOT run on the backend.** Analysis (WASM or JS) runs in the frontend only.
- **Backend does NOT receive raw file bytes.** Only structured findings + `file_sha256` (and optional metadata such as file name/size) are sent.
- **Frontend** performs file analysis, then POSTs `{ file_sha256, findings, classification, ... }` to the backend. Backend persists findings and metadata only; generates CBOM via `ToCBOM()` on demand.
- **trust_level** for file scans is always `"self_reported"` (stored and exposed in CBOM/metadata). No TEE or attestation in Discovery.
- **FileTarget** must not contain file data (no content, no raw bytes). It carries `file_sha256`, client-sent findings/classification/metadata, and **scanner_version** / **ruleset_version** (so divergent results across clients can be explained by different rules/versions). TLS and Wallet remain backend-run plugins; PlanLimitKey = `"file"` for billing; unified ScanResult interface is kept for file (FileScanResult implements ScanResult).
- **File plugin has no worker.** Handler calls DecodeHTTP then persists synchronously. TLS/Wallet use NATS + worker; File does not.

---

## 2. New Types and Interfaces to Introduce

### 2.1 Scan Target (typed input)

**Package:** `pkg/scan`.

```go
// ScanTarget is a marker interface for typed scan inputs.
// Implementations: TLSTarget, WalletTarget, FileTarget.
type ScanTarget interface {
    // ScanKind returns the plugin kind (tls, wallet, file).
    ScanKind() string
}

// TLSTarget carries TLS scan input (e.g. URL, optional port).
type TLSTarget struct { ... }

// WalletTarget carries wallet scan input (e.g. normalized address).
type WalletTarget struct { ... }

// FileTarget carries the client-reported file scan payload. Must NOT contain file content or raw bytes.
// Frontend sends file_sha256 + findings + classification (analysis done locally via WASM/JS).
type FileTarget struct {
    FileSHA256      string         // required; SHA256 computed locally before analysis; findings are logically bound to this
    Findings        []Finding     // client-computed findings from local analysis
    Classification  string        // client-computed classification
    ScannerVersion  string         // e.g. "1.2.0" — important for credibility; explains divergent results across clients
    RulesetVersion  string         // e.g. "2024.01" — which rules were used; two clients, two rulesets → explainable divergence
    FileName        string         // optional, for display
    FileSize        int64          // optional, for display
    // No file content, no raw bytes — backend never receives file data
}
```

Each implements `ScanKind() string` so the registry can identify the plugin without string parsing.

**Convention:** Always pass **pointers** to targets: `*TLSTarget`, `*WalletTarget`, `*FileTarget`. DecodeHTTP and DecodeMessage return `ScanTarget` (the concrete value behind the interface is a pointer, e.g. `*TLSTarget`). Run receives `ScanTarget` and the plugin asserts to the pointer type (e.g. `target.(*TLSTarget)`). Services and plugin internals take `*TLSTarget`, `*WalletTarget`, `*FileTarget` — never value types.

### 2.2 Finding and ScanResult (output contract)

**Package:** `pkg/scan`.

```go
// Finding represents a single discovery finding (e.g. one primitive, one cert, one key).
type Finding struct {
    Type        string         // e.g. "certificate", "key-exchange", "cryptographic-primitive"
    Name        string
    NISTLevel   int
    QuantumVuln bool
    Details     map[string]any // optional plugin-specific fields
}

// ScanResult is the contract for any discovery scan result.
// TLS and Wallet domain types (or adapters) must implement this.
type ScanResult interface {
    ScanKind() string
    ScannedAt() time.Time
    Findings() []Finding
    Classification() string   // e.g. "legacy", "hybrid", "pq-ready", "unknown"
    ToCBOM() (map[string]any, error)
}
```

- **Classification** is a single string per result (e.g. overall posture). Exact values can be defined per kind (TLS: legacy/hybrid/pq-ready; wallet: similar; file: idem).
- **Findings** are the structured list used for CBOM components and for persistence. Persisted storage should store findings (and classification, metadata) so that loading from DB and calling `Findings()` / `Classification()` / `ToCBOM()` does not require storing CBOM JSON.

**Location:** Both **Finding** and **ScanResult** are defined in **pkg/scan** only. Domain types (internal/domain) and adapters (internal/scan/*) implement `scan.ScanResult` and use `scan.Finding`; they do not duplicate these types.

### 2.3 Plugin interface (no generics)

**Package:** `pkg/scan`.

Go does not allow storing heterogeneous generic interfaces (e.g. `Runner[TLSTarget]`, `Runner[WalletTarget]`) in a single typed registry without losing type information: you would end up with `Runner interface{}` and a type assertion at the call site. **Solution:** do not use generics. Use a single **Plugin** interface; each plugin implements **Run(ctx, userID, target ScanTarget, opts)** and performs the **cast inside the plugin** (e.g. `target.(*TLSTarget)`). Handler and worker only call `plugin.Run(ctx, userID, target, opts)` with `ScanTarget`; they never do type assertions.

```go
// PluginDescriptor identifies the plugin and its limits/capabilities.
type PluginDescriptor struct {
    Kind          string   // "tls" | "wallet" | "file"
    Subject       string   // NATS subject (e.g. cafe.discovery.wallet.scan)
    PlanLimitKey  string   // same as Kind for billing
    Capabilities  []string // e.g. "cms-pkcs7", "jwt-jwe", "pem-x509", "json-header"
    Version       string   // e.g. "1.0" for compatibility evolution
}

type RunOptions struct {
    IsDefault bool // for TLS: default endpoint vs user-scanned
}

// Plugin is the full contract: descriptor, decode (HTTP vs NATS), run.
// DecodeHTTP and DecodeMessage are split so API format and NATS message format stay separate;
// handler uses DecodeHTTP(body), worker uses DecodeMessage(unmarshaledMsg) — no re-unmarshal, no mixing.
// Each implementation casts target to its concrete type inside Run.
type Plugin interface {
    Descriptor() *PluginDescriptor   // pointer: avoid copies, allows future extension, consistent with registry
    DecodeHTTP(body []byte) (ScanTarget, error)   // HTTP request body → ScanTarget (handler only)
    DecodeMessage(msg any) (ScanTarget, error)    // unmarshaled NATS message → ScanTarget (worker only)
    Run(ctx context.Context, userID *uuid.UUID, target ScanTarget, opts RunOptions) (ScanResult, error)
}
```

**For File plugin only:** `Run()` does **not** perform analysis. `Run()` validates and persists client-reported findings only. No backend scanning; avoids ambiguity.

**Example (TLS plugin):** the cast is contained in the plugin; handler/worker stay clean.

```go
func (p *TLSPlugin) Run(ctx context.Context, userID *uuid.UUID, target scan.ScanTarget, opts scan.RunOptions) (scan.ScanResult, error) {
    tlsTarget, ok := target.(*scan.TLSTarget)
    if !ok {
        return nil, errors.New("invalid target type for TLS plugin")
    }
    return p.service.ScanTLS(ctx, userID, tlsTarget, opts)
}
```

- **DecodeHTTP(body []byte)**: handler parses HTTP body (e.g. `{"address":"0x..."}` or `{"url":"https://..."}`) and calls `plugin.DecodeHTTP(body)` → ScanTarget. API shape is owned by the plugin but stays HTTP-only.
- **DecodeMessage(msg any)**: worker unmarshals NATS message into the typed struct (e.g. `WalletScanMessage`), then calls `plugin.DecodeMessage(msg)` → ScanTarget. Message shape is owned by the plugin; no second JSON unmarshal, no mixing of HTTP and NATS formats.
- No string-based discrimination: handler resolves plugin by kind (route/body); plugin owns validation and target shape for both HTTP and message.

### 2.4 Plugin registration

**Package:** `pkg/scan`.

```go
func Register(p Plugin)   // p is the interface; e.g. &TLSPlugin{}, &WalletPlugin{}
func Get(kind string) Plugin
func GetBySubject(subject string) Plugin
func Kinds() []string
```

The registry stores **Plugin** (interface). For TLS/Wallet: handler calls `plugin.DecodeHTTP(body)` then publishes to NATS; worker unmarshals, calls `plugin.DecodeMessage(msg)` then `plugin.Run(...)`. For File: handler calls `plugin.DecodeHTTP(body)` then **persists synchronously** (no NATS, no worker). HTTP format and NATS message format stay separate for TLS/Wallet; no re-unmarshal, no mixing.

### 2.5 Worker helper (shared logic and log/trace contract)

**Package:** `internal/worker`.

- **New types:** None beyond a helper function or small struct, plus a **standardized log/trace payload** (struct or map) that the helper always populates and passes to logging/tracing.
- **Concept:** A shared helper that:
  - Takes: subject name, maxConcurrent, unmarshal fn, run fn (that receives the decoded message and returns error).
  - Runs the run fn under a semaphore, with consistent error handling and **standardized logging/tracing** (see contract below).
  - Each worker unmarshals the NATS message into the typed struct (e.g. `WalletScanMessage`), calls **plugin.DecodeMessage(msg)** → ScanTarget, then **plugin.Run(ctx, userID, target, opts)**; the plugin does the internal cast to its concrete target type.

**Mandatory log/trace contract.** The helper must standardize the following so that all workers emit consistent, queryable logs and traces. Without this, deduplication would still leave inconsistent log shapes across TLS vs Wallet vs File workers.

| Field | Description |
|-------|-------------|
| **correlation_id** | Unique id for the request/message (from NATS message header or generated). |
| **scan_id** | Id of the scan result once persisted (if applicable); empty until after Run. |
| **user_id** | User who triggered the scan (from message). |
| **kind** | Plugin kind: `tls` \| `wallet` \| `file`. |
| **subject** | NATS subject (e.g. `cafe.discovery.wallet.scan`). |
| **duration** | Time taken for Run (and optional unmarshal). |
| **error_classification**
Example contract:

```go
// ProcessWithConcurrency runs fn with semaphore and enforces the log/trace contract.
// Worker passes context with correlation_id; helper logs start/end with kind, subject, user_id, duration, error_classification.
func ProcessWithConcurrency(ctx context.Context, name string, kind string, subject string, sem chan struct{}, msg *nats.Msg, fn func() (scanID string, err error)) error
```

TLS/Wallet workers unmarshal message → plugin.DecodeMessage(msg) → plugin.Run; they delegate concurrency and **all structured logging/tracing** to the helper so that logs remain coherent across plugins.

### 2.6 Plan service (usage counters)

**Package:** `internal/service` (and optionally `pkg/scan` for the interface).

```go
// UsageCounter returns the number of scans for a user for a given kind (for plan limits).
type UsageCounter interface {
    CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
}
```

- **PlanService** will accept a **map[string]UsageCounter** keyed by kind (`KindTLS`, `KindWallet`, `KindFile`) instead of two fixed repo parameters.
- **Plan** and **PlanUsage** will use a **data-driven** representation for limits:
  - Option A: Keep columns `WalletScanLimit`, `EndpointScanLimit` and add `FileScanLimit`; internally map kind → column (`wallet`→WalletScanLimit, `tls`→EndpointScanLimit, `file`→FileScanLimit).
  - Option B: Single JSON column or key-value table `scan_limits` (kind → limit). Refactor plan prefers Option A for minimal DB change; PlanService uses an internal map kind → limit/usage and avoids a long switch.

- **Stable kind constants** in one place: `pkg/scan` (see kinds.go):

```go
const (
    KindTLS    = "tls"
    KindWallet = "wallet"
    KindFile   = "file"
)
```

PlanLimitKey in descriptors must match these. Internal names (e.g. "endpoint" in error messages or DB) can remain where needed, with a single mapping layer (kind → display name or limit key).

---

## 3. CBOM Generation Strategy (Persistence vs On-Demand)

### 3.1 What to persist in Postgres

- **Structured scan result only:**
  - **Findings:** list of Finding (or JSON column that deserializes to `[]Finding`).
  - **Classification:** string (e.g. legacy, hybrid, pq-ready, unknown).
  - **Metadata:** scan timestamp, user_id, target identifier (address, url, file_id), plugin version, etc.
- **Do NOT persist:** CBOM JSON blob. No new column that stores the full CBOM.

### 3.2 Current state vs target

- **Today:** Entities (e.g. `ScanResultEntity`, `TLSScanResultEntity`) store flat fields and JSON columns (certificate, cipher_suites, recommendations, etc.). Handlers build CBOM from entities/DTOs in multiple places (scanResultToCBOM, getWalletCBOM, tlsScanResultToCBOM, getTLSCBOM).
- **Target:** Entities (or new versions of them) store:
  - A **findings** structure (or equivalent) and **classification** and **metadata** in a form that allows reconstructing the scan.ScanResult interface (pkg/scan: Findings(), Classification(), ScannedAt(), ScanKind()).
  - Domain types (or adapters) built from these entities implement **scan.ScanResult** (from pkg/scan) and **ToCBOM()**. All API responses that today return “CBOM” will load the entity, convert to ScanResult, call **ToCBOM()** once, and return that JSON.

### 3.3 Mapping current entities to findings/classification

- **Wallet:** Current fields (Algorithm, NISTLevel, KeyExposed, Networks, etc.) can be mapped to a single or multiple **Finding**s and a **Classification** string (e.g. from NISTLevel and KeyExposed). No mandatory schema change in Phase 1 if we add an adapter that implements scan.ScanResult from existing wallet/TLSScanResult DTOs and derives Findings/Classification from existing fields.
- **TLS:** Certificate, cipher suites, KEX, recommendations, etc. map to multiple Findings; overall NIST level and PQC mode → Classification.
- **File (new):** Local-first; backend persists only what the frontend sends. See §3.5.

Introducing **Finding** and **ScanResult** in `pkg/scan` allows the refactor to be done in steps: first have domain types (or adapters) implement `scan.ScanResult` (with Findings/Classification derived from current fields) and centralize ToCBOM(); then optionally add explicit findings/classification columns and migrate persistence later.

### 3.4 List endpoints vs CBOM cost: list returns scan IDs only

**Problem:** If "list latest scans" returns 100 full CBOMs, calling `ToCBOM()` x100 per request is non-negligible (CPU and latency).

**Mitigation:**

- **List endpoints** (current API: `GET /discovery/v1/wallets/scans`, `GET /discovery/v1/tls/scans`) return lightweight scan summaries with `scan_id`. No full CBOM or heavy detail payload is returned for each row.
- A **dedicated detail endpoint** returns a single scan by `scan_id` (current API: `GET /discovery/v1/wallets/scans/:scan_id`, `GET /discovery/v1/tls/scans/:scan_id`). Earlier `CBOM historique` naming in this plan is historical.

This keeps "CBOM never stored" while avoiding ToCBOM() x100 on list. Backward compatibility is not required (nothing in production), so the API can be designed this way from the start.

### 3.5 Persistence schema for file scans (local-first)

- **Table (e.g. `file_scan_results`):**  
  `id`, `user_id`, `file_sha256` (required), `findings` (JSON/JSONB), `classification`, `trust_level`, **`scanner_version`**, **`ruleset_version`**, `file_name`, `file_size`, `scanned_at`, `created_at`, etc.  
  **No raw file bytes, no file content.** Backend never stores or receives file data. scanner_version and ruleset_version explain divergent results across clients.
- **trust_level:** Always `"self_reported"` for file scans. Stored in the entity and exposed in CBOM (e.g. in metadata or top-level) so consumers know the origin. No TEE or attestation in Discovery.
- **CBOM:** Generated on demand via `FileScanResult.ToCBOM()` from persisted findings and metadata. Same unified ScanResult interface; file implements it with trust_level in the output.

---

## 4. Files to Modify

### 4.1 New files (create)

| Path | Purpose |
|------|--------|
| `pkg/scan/target.go` | ScanTarget interface, TLSTarget, WalletTarget, FileTarget (structs; always pass *TLSTarget, *WalletTarget, *FileTarget) |
| `pkg/scan/result.go` | Finding struct, ScanResult interface |
| `pkg/scan/plugin.go` | PluginDescriptor, RunOptions, **Plugin interface** (Descriptor, **DecodeHTTP**, **DecodeMessage**, Run(..., target ScanTarget, ...)); no Runner[T], no struct Plugin |
| `pkg/scan/registry.go` | Register, Get, GetBySubject, Kinds |
| `pkg/scan/kinds.go` | KindTLS, KindWallet, KindFile constants |
| `pkg/cbom/builder.go` | BaseCBOM(kind, scannedAt, components, metadataExtra) helper used by ToCBOM() implementations |
| `internal/worker/helper.go` | ProcessWithConcurrency + **mandatory log/trace contract**: correlation_id, scan_id, user_id, kind, subject, duration, error_classification |
| `internal/scan/wallet/plugin.go` | Wallet plugin implementing **Plugin**: DecodeHTTP/DecodeMessage return *WalletTarget as ScanTarget; Run(target) casts to *WalletTarget then call DiscoveryService. Always use pointer targets. |
| `internal/scan/tls/plugin.go` | TLS plugin implementing **Plugin**: DecodeHTTP/DecodeMessage return *TLSTarget as ScanTarget; Run(target) casts to *TLSTarget then call TLSService. Always use pointer targets. |
| `internal/scan/file/plugin.go` | File plugin (local-first): DecodeHTTP(body) parses client payload { file_sha256, findings, classification, scanner_version, ruleset_version, ... } → *FileTarget. Run(target) **does not perform analysis**; validates and persists client-reported findings only. No worker, no NATS — synchronous persist in handler. trust_level = "self_reported". |
| `internal/domain/file_scan_result.go` | FileScanResultEntity (file_sha256, findings, classification, trust_level, scanner_version, ruleset_version, file_name, file_size, scanned_at, user_id, ...); FileScanResult DTO implementing scan.ScanResult with ToCBOM() including trust_level. No file content. |
| `internal/repository/file_scan_result_repository.go` | CRUD + CountByUserID (UsageCounter) for file_scan_results. |

### 4.2 Existing files to modify

| Path | Changes |
|------|--------|
| `internal/domain/models.go` | Have wallet domain type implement `scan.ScanResult` (import Finding from pkg/scan): ScanKind(), ScannedAt(), Findings(), Classification(), ToCBOM(); or use adapters in internal/scan/*. |
| `internal/domain/tls_scan_result.go` | Same: have TLS domain type implement `scan.ScanResult` (or adapter in internal/scan/tls). |
| `internal/domain/scan_result.go` | ScanResultEntity: optionally add findings/classification columns later; or keep current schema and derive from existing fields in adapter. |
| `internal/service/discovery.go` | ScanWallet: accept *WalletTarget (or address from it); return type implementing ScanResult. Optionally keep current signature and adapt at plugin boundary. |
| `internal/service/tls.go` | ScanTLS: accept *TLSTarget (or URL from it); return type implementing ScanResult. Optionally keep current signature and adapt at plugin boundary. |
| `internal/worker/wallet_worker.go` | Use shared helper; unmarshal to WalletScanMessage, plugin.DecodeMessage(msg) → ScanTarget, call plugin.Run(ctx, userID, target, opts). |
| `internal/worker/tls_worker.go` | Same: shared helper; unmarshal to TLSScanMessage, plugin.DecodeMessage(msg) → ScanTarget, call plugin.Run(ctx, userID, target, opts). |
| `internal/worker/base_worker.go` | Possibly extend or keep as-is; workers still use BaseWorker for Subscribe + handleMessage; the handler body uses the shared helper. |
| `internal/handler/discovery.go` | UnifiedScan: resolve kind, get plugin, **DecodeHTTP(body)** → ScanTarget, check limits, publish. **List endpoints:** return list of scan IDs only. **CBOM:** dedicated endpoint GET CBOM by scan_id → load result → result.ToCBOM() → c.JSON(). |
| `internal/handler/tls.go` | Scan: plugin **DecodeHTTP(body)** and publish. **List:** return scan IDs only. **CBOM by scan_id:** load entity → ScanResult → ToCBOM(). |
| `internal/service/plan.go` | Use KindTLS, KindWallet, KindFile. Refactor GetPlanUsage and CheckScanLimit to take map[string]UsageCounter (or a struct of counters). Replace switch(scanType) with lookup by kind. Map "endpoint" → KindTLS internally for backward compatibility if needed. PlanUsage struct: use kind-based fields or a map (e.g. ScansUsed[kind], ScanLimit[kind]). |
| `internal/domain/plan.go` | Add FileScanLimit if adding file scans; keep WalletScanLimit, EndpointScanLimit. IsUnlimited(scanType): use kind constants; internal mapping from "tls" to endpoint limit. |
| `internal/repository/scan_result_repository.go` | Implement UsageCounter (CountByUserID) if not already. |
| `internal/repository/tls_scan_result_repository.go` | Implement UsageCounter. |
| `internal/app/container.go` | Wire plugin registry: register wallet and TLS plugins; pass UsageCounter map to PlanService; register workers as today (one per subject). |
| `cmd/worker/main.go` | No structural change; workers remain TLS and Wallet only (File has no worker). |
| `pkg/nats/messages.go` | No change to message types (keep WalletScanMessage, TLSScanMessage). |
| `pkg/nats/nats.go` | Keep existing subjects (SubjectWalletScan, SubjectTLSScan). |

### 4.3 Files not to modify (or minimal)

- **CLI** (wallet → `cafe-scanner-wallet` PR4 ; TLS/OQS docs → `cafe-scanner-tls` PR5): migrated out of Discovery.
- **Native/CGO** (TLS PQC): No change.
- **Auth, middleware, config**: No change unless PlanHandler needs the new PlanService signature.
- **Redis repos** (anonymous scan counts): No change to key layout; only ensure the “scan type” passed to checkScanLimits uses the same kind as PlanLimitKey where needed.

---

## 5. Migration Strategy in Phases (Minimal Breakage)

### Phase 1: Interfaces and kinds (no API or behavior change)

1. Add `pkg/scan`: kinds, ScanTarget, Finding, ScanResult interface, PluginDescriptor, RunOptions, **Plugin interface** (Descriptor, **DecodeHTTP**, **DecodeMessage**, Run(..., target ScanTarget, ...)), registry (Register(Plugin), Get, GetBySubject, Kinds).
2. Add `pkg/cbom`: BaseCBOM helper.
3. Implement ScanResult for existing wallet and TLS domain types (or thin adapters): ScanKind(), ScannedAt(), Findings() (derive from current fields), Classification() (derive), ToCBOM() (using BaseCBOM + kind-specific components). Do not change HTTP responses yet; ensure ToCBOM() output matches current CBOM shape.
4. Add `internal/worker/helper.go`: ProcessWithConcurrency that enforces the **log/trace contract** (correlation_id, scan_id, user_id, kind, subject, duration, error_classification). Use it in TLS and Wallet workers.

**Exit criterion:** Tests pass; list/detail/CBOM responses unchanged; workers still process messages the same way (internally they can build a Target and call a Runner that wraps the existing service methods).

### Phase 2: Plugin registration and typed runners

5. Add `internal/scan/wallet/plugin.go`: type implementing **Plugin**; DecodeHTTP/DecodeMessage return *WalletTarget as ScanTarget; Run(target) casts to *WalletTarget, calls DiscoveryService.ScanWallet. Always pass *WalletTarget.
6. Add `internal/scan/tls/plugin.go`: type implementing **Plugin**; DecodeHTTP/DecodeMessage return *TLSTarget as ScanTarget; Run(target) casts to *TLSTarget, calls TLSService.ScanTLS. Always pass *TLSTarget.
7. Register both plugins in container (Register(walletPlugin), Register(tlsPlugin)); services keep existing signatures; each plugin’s Run adapts target and returns ScanResult.
8. Refactor PlanService: introduce UsageCounter; implement it for ScanResultRepository and TLSScanResultRepository; add kind constants; refactor GetPlanUsage/CheckScanLimit to use a map[string]UsageCounter and kind-based limit lookup (keep Plan struct columns; map KindTLS → EndpointScanLimit, KindWallet → WalletScanLimit). Update PlanHandler and discovery/tls handlers to pass the new PlanService dependency if needed.

**Exit criterion:** Plan limits and API behavior unchanged; plugins (Plugin interface) are registered; workers call plugin.Run(ctx, userID, target, opts) with ScanTarget (cast done inside plugin).

### Phase 3: Handlers and workers use plugins; CBOM only via ToCBOM()

9. Discovery handler: UnifiedScan uses plugin registry by kind (from body: address → wallet, url → tls); **DecodeHTTP(body)** → ScanTarget; check limits with plugin.PlanLimitKey; publish to plugin.Descriptor.Subject. Remove duplicated scanWallet/scanTLS logic.
10. TLS handler: Scan uses TLS plugin **DecodeHTTP(body)** and publish. **List handlers:** return scan IDs only. **CBOM by scan_id:** single endpoint loads entity → ScanResult → ToCBOM() → JSON. Same for discovery (wallet/tls/file).
11. Workers: TLS worker unmarshals to TLSScanMessage, calls **plugin.DecodeMessage(msg)** → ScanTarget, then **plugin.Run(ctx, userID, target, opts)**; same for Wallet. Both use the shared helper. Persistence remains in services (called from inside plugin.Run). HTTP format and NATS message format stay separate (no re-unmarshal in worker).

**Exit criterion:** List endpoints return scan IDs only; CBOM only via dedicated by-scan_id endpoint (ToCBOM() once per request); one worker per subject; limits and subjects unchanged.

### Phase 4: Persistence shape (findings, classification, metadata)

12. (Optional) Add findings and classification to entities: e.g. `Findings JSON` (or JSONB), `Classification string` in scan_results and tls_scan_results. Populate them when saving (derive from current fields in the same commit or a follow-up). Keep backward compatibility: if Findings is empty, derive from existing columns in ScanResult implementation.
13. Ensure all code paths that return scan results load from Postgres and go through ScanResult.ToCBOM(); confirm no CBOM JSON is ever stored.

**Exit criterion:** Persisted data includes structured findings/classification where desired; CBOM remains generated only in memory.

### Phase 5: File scan plugin (local-first, when required)

14. Add **FileTarget** (file_sha256, findings, classification, **scanner_version**, **ruleset_version**, optional file_name/file_size; **no file content**). Add **file_scan_results** persistence (file_sha256, findings, classification, trust_level = "self_reported", scanner_version, ruleset_version, metadata). File plugin: DecodeHTTP parses client POST body; Run(*FileTarget) **validates and persists only** (no analysis). **No file worker** — handler persists synchronously. FileScanResult implements ScanResult; ToCBOM() includes trust_level. Add FileScanLimit, UsageCounter, register plugin. Frontend (WASM/JS) performs analysis and POSTs findings + file_sha256; backend never receives raw file bytes. SHA binding and advisory nature documented (§1.5). Discovery vs Remediation trust boundary (§1.4).

---

## 6. Edge Cases and Risks

### 6.1 API design (no backward compatibility required)

- **Scope:** Nothing is in production; backward compatibility is **not** required. List endpoints can return scan IDs only; CBOM is fetched by scan_id via a dedicated endpoint. PlanUsage and response shapes can be designed cleanly (e.g. kind-based keys) without preserving legacy fields.

### 6.2 Kind mapping (internal vs billing)

- **"endpoint" vs "tls":** Today CheckScanLimit and error messages use "endpoint". PlanLimitKey and stable kind are "tls". We need a single place that maps KindTLS to the limit field (EndpointScanLimit) and to user-facing strings ("endpoint") so that logs and errors stay consistent. Risk: mixed use of "tls" and "endpoint" in different layers. Mitigation: use kind constants everywhere in new code; introduce a small mapping (KindTLS → limit key "endpoint" for DB/plan columns, KindTLS → "TLS endpoint" for messages) in one module.

### 6.3 Default endpoints (TLS)

- TLS has “default” endpoints (userID=nil, IsDefault=true). Run receives RunOptions.IsDefault. Plan limits must not count default scans. Current logic already excludes them. Ensure when moving to plugin.Run(..., target, opts), default endpoint path still passes IsDefault=true and is not counted in UsageCounter.

### 6.4 Anonymous scans (obsolete)

- **Update:** All API access now requires authentication. The anonymous list endpoints (e.g. la liste TLS historique/anonymous) have been removed. Anonymous scan limits and listing are no longer used.

### 6.5 DecodeHTTP / DecodeMessage and validation

- **DecodeHTTP(body)** (handler): validates HTTP input (e.g. address format, URL scheme). Failed validation returns a clear error; request is not published to NATS. API format is separate from NATS.
- **DecodeMessage(msg)** (worker): receives already-unmarshaled NATS message (e.g. `*WalletScanMessage`); converts to ScanTarget. No re-unmarshal of JSON; message format stays separate from HTTP. Validation can be repeated here for safety (e.g. normalize address again) or considered trusted after handler validation; plugin owns both paths.

### 6.6 Performance

- ToCBOM() is called on every list/detail/CBOM request. Ensure it’s cheap (no heavy recomputation). Current CBOM building is already in-memory from loaded entities; same here. If we add a structured findings column, avoid N+1 when loading lists.

### 6.7 Plugin registration order

- Registry must be populated before handlers/workers run. Do registration in container init or main; no dynamic plugin loading required for this refactor.

### 6.8 File scan: trust and verification

- File scan results are **self-reported** (trust_level = `"self_reported"`). Backend does not verify file contents or re-run analysis. No TEE or attestation in Discovery. **SHA binding is advisory:** backend stores SHA256 + findings but does not verify that the SHA corresponds to actual file content; we do not prove that findings correspond to the file. Discovery is advisory. Consumers of the CBOM can use trust_level (and scanner_version, ruleset_version) to interpret the result. Remediation layer (out of scope) may impose higher assurance where needed.

---

## 7. Proposed Directory Structure

```
cafe-discovery/
├── cmd/
│   ├── api/           # (if split from server)
│   ├── server/
│   │   └── main.go
│   └── (cli/ removed — wallet/TLS CLIs in scanner repos PR4/PR5)
├── internal/
│   ├── app/
│   │   └── container.go      # wire plugins, workers, PlanService with UsageCounter map
│   ├── domain/
│   │   ├── models.go         # Wallet/TLS domain types; implement scan.ScanResult (Finding from pkg/scan)
│   │   ├── scan_result.go    # ScanResultEntity (optional findings/classification columns)
│   │   ├── tls_scan_result.go
│   │   ├── file_scan_result.go   # file_sha256, findings, classification, trust_level (no file content)
│   │   └── plan.go           # Plan; IsUnlimited(kind) using constants
│   ├── handler/
│   │   ├── discovery.go      # UnifiedScan; list = scan IDs only; CBOM by scan_id via ToCBOM()
│   │   ├── tls.go            # Scan via plugin; list = scan IDs only; CBOM by scan_id
│   │   └── ...
│   ├── service/
│   │   ├── discovery.go      # ScanWallet(*WalletTarget or address) → ScanResult
│   │   ├── tls.go             # ScanTLS(*TLSTarget or url, opts) → ScanResult
│   │   ├── plan.go           # kind-based limits; UsageCounter map
│   │   └── ...
│   ├── repository/
│   │   ├── scan_result_repository.go       # + UsageCounter
│   │   ├── tls_scan_result_repository.go   # + UsageCounter
│   │   ├── file_scan_result_repository.go # file scans (local-first); + UsageCounter
│   │   └── ...
│   ├── worker/
│   │   ├── base_worker.go
│   │   ├── helper.go         # ProcessWithConcurrency / shared logic
│   │   ├── wallet_worker.go   # unmarshal → WalletTarget → plugin Runner; use helper
│   │   ├── tls_worker.go     # unmarshal → TLSTarget → plugin Runner; use helper
│   └── scan/                 # plugin implementations (optional subdir)
│       ├── wallet/
│       │   └── plugin.go
│       ├── tls/
│       │   └── plugin.go
│       └── file/
│           └── plugin.go
├── pkg/
│   ├── scan/
│   │   ├── kinds.go          # KindTLS, KindWallet, KindFile
│   │   ├── target.go         # ScanTarget, TLSTarget, WalletTarget, FileTarget
│   │   ├── result.go         # Finding, ScanResult interface
│   │   ├── plugin.go         # PluginDescriptor, RunOptions, Plugin interface (no Runner[T])
│   │   └── registry.go       # Register, Get, GetBySubject, Kinds
│   ├── cbom/
│   │   └── builder.go        # BaseCBOM
│   ├── nats/
│   │   ├── nats.go           # subjects unchanged
│   │   └── messages.go       # message types unchanged
│   └── tls/
│       └── ...
└── docs/
    ├── SCAN_PLUGIN_ARCHITECTURE.md
    └── SCAN_REFACTORING_PLAN.md  # this document
```

- **pkg/scan** holds the contract (targets, result interface, plugin descriptor, registry).
- **internal/scan** holds discovery-plugin implementations (wallet, tls, file) that depend on internal services and repositories.
- **internal/worker** keeps one worker per NATS subject for TLS and Wallet only; File plugin has no worker (sync persist in handler).

---

## 8. Summary Checklist

- [ ] ScanTarget interface + TLSTarget, WalletTarget, FileTarget (structs); **always pass pointers** (*TLSTarget, *WalletTarget, *FileTarget). No string-based dispatch.
- [ ] Each plugin implements **Plugin** (Descriptor, **DecodeHTTP**, **DecodeMessage**, Run(..., target ScanTarget, ...)); handler uses DecodeHTTP, worker uses DecodeMessage; cast from ScanTarget to concrete type is done **inside** the plugin only. HTTP format ≠ NATS message format; no mixing, no re-unmarshal.
- [ ] One worker per NATS subject for TLS and Wallet; File has no worker (sync persist). Shared helper for concurrency, logging, error handling; no generic multi-kind worker.
- [ ] Persist only structured data (findings, classification, metadata); never persist CBOM JSON; generate CBOM only via ToCBOM().
- [ ] ScanResult interface (pkg/scan): ScanKind(), ScannedAt(), Findings(), Classification(), ToCBOM(); TLS and Wallet domain types or adapters implement it.
- [ ] Kind constants: KindTLS, KindWallet, KindFile; PlanLimitKey matches; billing uses these keys; internal "endpoint" mapping in one place.
- [ ] PlanService refactored to avoid hardcoded scan-type branching (map of UsageCounter, kind-based limit lookup).
- [ ] PluginDescriptor extended with Capabilities and Version.
- [ ] List endpoints return scan IDs only; dedicated endpoint returns full CBOM by scan_id (no ToCBOM() x N on list).
- [ ] Worker helper enforces log/trace contract: correlation_id, scan_id, user_id, kind, subject, duration, error_classification.
- [ ] No Runner[T] generics; Plugin interface with Run(..., target ScanTarget, ...); registry is simply map[string]Plugin; no type assertion in handler/worker.
- [ ] Discovery only; no policy/remediation; no migration logic. Backward compatibility not required (not in production).
- [ ] **Trust boundary:** Discovery = lightweight, developer-first (what’s there); Remediation = Cifer-backed secure transformation (out of scope). No TEE/attestation in Discovery. Documented in §1.4.
- [ ] **File scan local-first:** No backend file scanning; no raw file bytes to backend. Frontend (WASM/JS) analyzes and sends findings + file_sha256; backend persists synchronously (no worker, no NATS). File plugin Run() validates and persists only — does not perform analysis. trust_level = "self_reported"; scanner_version and ruleset_version in FileTarget and persistence. SHA binding is advisory (§1.5); backend does not verify SHA vs content. FileTarget has no file data (§1.5, §2.1, §3.5).
