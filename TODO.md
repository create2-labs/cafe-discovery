# Cafe Discovery — backlog

Items deferred; not blocking current IMM work unless noted.

---

## IMM-9 follow-up — Deduplicate CPM internal HTTP client (`cpmpolicyref`)

**Context:** IMM-9 added `ActiveWalletCPMContextForTarget` beside existing `PersistedPoliciesReferenceScan` in `internal/cpmpolicyref/http_client.go`.

**Improvement:** Extract a small private helper (e.g. `postInternalJSON(ctx, path, reqBody, respDest)`) shared by both methods.

**Acceptance:** Same behaviour and tests ; no change to paths or wire types.

**Repos:** `cafe-discovery` only.
