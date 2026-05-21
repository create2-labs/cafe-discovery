# Option A maintainer reference: Discovery wallet scan v1 ↔ CPM

**What is Option A?** The post-V1 integration where CPM and the frontend use **authenticated Discovery** wallet scan list/detail as policy input (scan rows persist behind Discovery today; not direct CPM/frontend DB access). Product scope, Option A vs future Option B, and constraints: [`cafe-crypto-policy-mgt/workplans/CPM_post_v_1_option_a_scan_context.md`](https://github.com/create2-labs/cafe-crypto-policy-mgt/blob/main/workplans/CPM_post_v_1_option_a_scan_context.md).

This document binds **Crypto Policy Management (CPM) Option A** flows to the **canonical Discovery v1 wallet scan APIs**. It complements (does not replace) authoritative workplans and OpenAPI in each repository:

| Source | Path / location |
|--------|-----------------|
| **Option A definition (product / architecture)** | [`cafe-crypto-policy-mgt/workplans/CPM_post_v_1_option_a_scan_context.md`](https://github.com/create2-labs/cafe-crypto-policy-mgt/blob/main/workplans/CPM_post_v_1_option_a_scan_context.md) |
| API narrative & merged PR index | [`cafe-crypto-policy-mgt/workplans/WORKPLAN_API_PR.md`](https://github.com/create2-labs/cafe-crypto-policy-mgt/blob/main/workplans/WORKPLAN_API_PR.md) |
| Stable API narrative | [`cafe-crypto-policy-mgt/workplans/WORKPLAN_API.md`](https://github.com/create2-labs/cafe-crypto-policy-mgt/blob/main/workplans/WORKPLAN_API.md) |
| Post‑V1 frontend sequence | [`cafe-crypto-policy-mgt/workplans/CPM_FRONTEND_PR_PLAN_V1.md`](https://github.com/create2-labs/cafe-crypto-policy-mgt/blob/main/workplans/CPM_FRONTEND_PR_PLAN_V1.md) (+ **F** follow‑ups for real scan wiring) |
| Discovery v1 contract | This repo — [`openapi/discovery-v1.yaml`](../openapi/discovery-v1.yaml) |
| CPM v1 contract | Sibling repo — `cafe-crypto-policy-mgt/openapi/cpm-v1.yaml` ([GitHub](https://github.com/create2-labs/cafe-crypto-policy-mgt/blob/main/openapi/cpm-v1.yaml)) |
| CPM `policy_context` validation (source of truth for mapping) | `cafe-crypto-policy-mgt/internal/api/explore_policy_context.go` ([GitHub](https://github.com/create2-labs/cafe-crypto-policy-mgt/blob/main/internal/api/explore_policy_context.go)) |
| Observation vocabulary enums | [`cafe-contracts/observation/wallet/v01/vocabulary.go`](https://github.com/create2-labs/cafe-contracts/blob/main/observation/wallet/v01/vocabulary.go) |

Historical note: **`GET /discovery/wallet-policy-contexts`** existed in an older integration path and was **removed** in favor of v1 (**PR11a** — see WORKPLAN_API_PR). Clients, scripts, and UI must **not** treat it as an active route.

---

## 1. URL matrix (direct backend vs edge)

Ingress strips the **`/api`** prefix before traffic reaches Discovery or CPM. Use the edge form for browsers and external scripts when a gateway fronts the stack.

### Discovery — wallet scans (owner JWT)

| Capability | Discovery backend (`cmd/server`) | Typical edge |
|------------|----------------------------------|--------------|
| List synopsis | `GET /discovery/v1/wallets/scans` | `GET /api/discovery/v1/wallets/scans` |
| Detail by `scan_id` | `GET /discovery/v1/wallets/scans/{scan_id}` | `GET /api/discovery/v1/wallets/scans/{scan_id}` |

**Pagination (list):** response shape is **`{ total, limit, offset, items }`** per [`PaginatedScanList`](../openapi/discovery-v1.yaml) — each element is **`ScanListItem`** (synopsis). There is **no** legacy `{ contexts: [...] }` envelope.

### CPM — explore, persist, list by scan, assessment

| Capability | CPM backend (in‑process path) | Typical edge |
|------------|-------------------------------|--------------|
| Synchronous explore ( **`policy_context` required** ) | `POST /cpm/v1/policies/decisions/explore` | `POST /api/cpm/v1/policies/decisions/explore` |
| Persist policy ( **`binding`** rules; discovery binding needs UUID `scan_id` ) | `POST /cpm/v1/policies` | `POST /api/cpm/v1/policies` |
| List policies for a scan | `GET /cpm/v1/policies?scan_id=` | `GET /api/cpm/v1/policies?scan_id=` |
| Async assessment ( **`policy_context` forbidden** — server reloads scan detail ) | `POST /cpm/v1/policies/assessment/request` | `POST /api/cpm/v1/policies/assessment/request` |

---

## 2. Option A flows (which body carries what)

### 2.1 Synchronous preview — **`POST …/policies/decisions/explore`**

Request body (**CPM**) includes **`policy_context`** and **`selection_request`**. Optionally **`scan_id`** at the top level for AUTH / binding semantics (see CPM OpenAPI and AUTH‑02).

**`policy_context`** parsing in CPM supports two shapes (strict where noted):

1. **Discovery v1 detail envelope** — mirrors **`WalletScanDetail`**: top‑level **`scan_id`**, **`status`**, and nested **`result`** (`WalletScanResult`). Extra keys inside **`result`** are ignored during JSON decode into the slim struct; malformed **`result`** fails validation.
2. **Flat Option A wire** — the same evaluator fields at the top level (legacy / hand‑built clients).

The nested form is what a frontend typically builds by serializing **`GET /discovery/v1/wallets/scans/{scan_id}`** (or the **`result`** plus top‑level **`scan_id`/`status`** fields) into **`policy_context`**.

### 2.2 Async assessment — **`POST …/policies/assessment/request`**

Body: **`scan_id`** + **`selection_request`** (**required**). **`policy_context` must not** be sent — OpenAPI forbids unknown properties on this schema; CPM loads wallet observation from Discovery scan detail internally (**PR13g** narrative).

### 2.3 Persistence & policy listing

**`POST …/policies`** persists **`scan_id`** per **`binding`** (default **`discovery`** requires a UUID). **`GET …/policies?scan_id=`** lists owner‑scoped persisted policies referencing that scan.

---

## 3. Normative mapping — v1 **`WalletScanDetail` → CPM explore `policy_context`**

Implementations **must** match **`parsePolicyContextFlexible`**, **`observationFromWalletPolicyContext`**, **`normalizeWireAccountKind`**, **`mapWirePQPostureToV01Exported`**, and **`normalizeWireAlgorithmID`** in CPM (**`explore_policy_context.go`**). This section is aligned with those functions as of Option A (**do not invent silent aliases beyond what the code accepts**).

### 3.1 Field‑by‑field table (required A2 deliverable)

| (a) Discovery v1 path (`GET …/wallets/scans/{scan_id}`) | (b) `policy_context` path (flat or via `result`) | (c) Rule | (d) Type / closed values |
|---|---|---|---|
| `scan_id` | `policy_context.scan_id` (top‑level envelope) **or** top‑level flat | Copied trimmed for wire metadata (**AUTH**/logging); evaluator uses evaluator payload derived from **`result`** | UUID string (`WalletScanDetail`) |
| `status` | `policy_context.status` (top‑level envelope) **or** top‑level flat | Copied trimmed; **lifecycle only** (`ScanLifecycleStatus`) — metadata, not PQ posture | `requested` \| `started` \| `completed` \| `failed` |
| `result.target_address` | **`result.target_address`** → wire **`target_address`** (and **`wallet_address`** when parsing nested **`result`**) | **1:1** (trim spaces). **Does not** populate evaluator **`walletobserved.Payload`** (that struct has **no address** field — see `explore_policy_context.go` comments: address / status / **`scan_id`** are wire metadata for AUTH/logging) | string |
| `result.target_address` | Flat: `wallet_address` and/or `target_address` | Nested envelope sets both **`WalletAddress`** and **`TargetAddress`** from **`target_address`**. Flat clients may supply either field on the slim wire shape | string |
| `result.wallet_type` | `result.wallet_type` or flat **`wallet_type`** | Passed through **`normalizeWireAccountKind`** → must resolve to valid **`AccountKind`** in `v01` or **`unknown`** is used when rules match | Discovery OpenAPI **`WalletScanResult.wallet_type`** enum: `eoa` \| `smart_account` \| `contract` \| `unknown`. CPM also accepts synonyms **`EOA`**, **`AA`**, **`CONTRACT`**, **`SMART_ACCOUNT`** (maps to **`erc4337_smart_account`**), delegated kind strings accepted by **`v01.AccountKind`**, otherwise **`unknown`**. Empty / unknown mapping → **`unknown`** if not valid |
| `result.chain_ids[]` | `result.chain_ids` or flat **`chain_ids`** | Append copy of slice; **no implicit default chain** — empty array stays empty (no fabricated `[1]`) | `int64` array |
| `result.current_algorithm` | N/A in OpenAPI (wire may omit) | Parsed from **`current_algorithm`** with fallback to **`algorithm`** (`pickAlgorithm`) | string; after **`normalizeWireAlgorithmID`**: must be non‑empty valid **`IsValidAlgorithmID`** or normalized (e.g. `secp256k1` → `secp256k1_ecrecover`). If still empty after normalization, CPM defaults to **`secp256k1_ecrecover`** |
| `result.algorithm` | same as above | Used when **`current_algorithm`** absent | string |
| `result.current_pq_posture` | `result.current_pq_posture` or flat **`current_pq_posture`** | **`mapWirePQPostureToV01Exported`**: empty → **`unknown`**; exported v01 values (`classical_only`, `hybrid`, `full_pq`, `unknown`) pass if valid; Discovery labels **`pq_ready` → `full_pq`**, **`not_pq_ready` → `classical_only`**, **`hybrid`/`unknown` → lowered as‑is**; anything else → **error** | Discovery enum: `pq_ready` \| `hybrid` \| `not_pq_ready` \| `unknown`. Internal v01: `classical_only` \| `hybrid` \| `full_pq` \| `unknown` |
| `result.scanned_at` | `result.scanned_at` or flat **`scanned_at`** | If non‑empty: must parse as **`time.RFC3339Nano`** or **`time.RFC3339`**; else **error**. If empty: allowed (zero timestamp in payload) | `date-time` string |
| `result.key_exposed` | `result.key_exposed` or flat **`key_exposed`** | **1:1** boolean → **`PublicKeyExposed`** | boolean |
| `result.observations`, `nist_level`, `risk_score`, `type`, `networks`, `first_seen`, `last_seen` | — | **Not read** by current explore `policy_context` parser (subset struct) | Present on API for UI / future use |

**List vs detail (`GET …/wallets/scans`):** synopsis items (`ScanListItem`) expose **`scan_id`, `target_address`, `chain_ids`, `created_at`, `status`** only. **Posture and algorithm** for explore should be taken from **detail** once the scan reaches a state where **`result`** is populated (see OpenAPI descriptions for **`result`** immutability after terminal states).

---

## 4. Reviewer checklist

- Every row in **§3.1** traced to **`openapi/discovery-v1.yaml`** (`WalletScanDetail`, `WalletScanResult`, `ScanListItem`, `ScanLifecycleStatus`) and CPM **`explore_policy_context.go`**.
- No documentation presents **`wallet-policy-contexts`** as a current public path without the **historical / removed** qualification.
- **Explore** vs **assessment** body rules (**`policy_context`** required vs forbidden) repeated in onboarding copy wherever integrators choose HTTP routes.
