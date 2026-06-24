# Cafe Discovery — backlog

Items deferred; not blocking current IMM work unless noted.

---

## PERS-D0 — Zero CP in Discovery (ADR boundary)

**Rule (normative):** `cafe-discovery` must **not** own Crypto Policy (CP) domain logic — no CP tables, no draft/policy payloads, no explore/ranking/persist implementation.

**Allowed today and at D6:**

- HTTP guards that delegate to CPM or **cafe-persistence** for **existence / count** only (`internal/cpmpolicyref/` for W1/W3).
- Discovery-defined HTTP error codes on guard failure (`409` etc.).

**Forbidden:**

- Importing CPM policy types or storing `crypto_policy_*` data in this repo.
- Reading or interpreting `crypto_policies.payload` in Discovery handlers.

**References:** [docs/ADR/ADR_20260622_persistence.md](docs/ADR/ADR_20260622_persistence.md) §4.1, §5.1, §9.3, §11.2 ; execution plan [docs/ADR/ADR_20260622_persistence_PR_PLAN.md](docs/ADR/ADR_20260622_persistence_PR_PLAN.md).

---

## IMM-9 follow-up — Deduplicate CPM internal HTTP client (`cpmpolicyref`)

**Context:** IMM-9 added `ActiveWalletCPMContextForTarget` beside existing `PersistedPoliciesReferenceScan` in `internal/cpmpolicyref/http_client.go`.

**Improvement:** Extract a small private helper (e.g. `postInternalJSON(ctx, path, reqBody, respDest)`) shared by both methods.

**Acceptance:** Same behaviour and tests ; no change to paths or wire types.

**Repos:** `cafe-discovery` only.
