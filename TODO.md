# Cafe Discovery — backlog

Items deferred; not blocking current IMM work unless noted.

## IMM-6b — Plan quota (success-only ledger + guards + CBOM)

**Context:** **IMM-6** (mergé [#73](https://github.com/create2-labs/cafe-discovery/pull/73)) compte les lignes Postgres — **DELETE** diminue l’usage ; comptage à l’acceptation POST — **incorrect** vs produit.

**Cible (WORKPLAN §2.2.1 P1, G1–G4) :**

- **`used`** = **`completed` success** only (ledger) ; **monotonic** on DELETE
- POST : **`successful + in_flight < limit`** + cap **`in_flight < min(limit, 3)`**
- Completion : slot atomique ; si quota dépassé → **`failed`** + **`PLAN_LIMIT_EXCEEDED`**, adresse gardée, **résultat jeté**
- **`GET /plans/usage`** : **`visible`**, **`deleted_by_user`**
- CBOM **404** sauf **completed success**

**Plan:** **IMM-6b-1** ([#83](https://github.com/create2-labs/cafe-discovery/pull/83)), **IMM-6b-2** ([#84](https://github.com/create2-labs/cafe-discovery/pull/84)), **IMM-6b-3** ([#84](https://github.com/create2-labs/cafe-discovery/pull/84) — garde POST G1/G2), **IMM-6b-4** ([#85](https://github.com/create2-labs/cafe-discovery/pull/85) — commit atomique persistence G3), **IMM-6b-5** ([#87](https://github.com/create2-labs/cafe-discovery/pull/87) — `GET /plans/usage` breakdown) livré ; **IMM-6b-6** (backfill) **annulé** (reset DB) ; **IMM-6b-7…8** — [`IMMUTABILITE_PR.md`](./IMMUTABILITE_PR.md#imm-6b--quotas-plan--ledger-success-only-garde-post-commit-atomique-option-b).

**Repos:** `cafe-discovery` only.

---

## IMM-9 follow-up — Deduplicate CPM internal HTTP client (`cpmpolicyref`)

**Context:** IMM-9 added `ActiveWalletCPMContextForTarget` beside existing `PersistedPoliciesReferenceScan` in `internal/cpmpolicyref/http_client.go`.

**Improvement:** Extract a small private helper (e.g. `postInternalJSON(ctx, path, reqBody, respDest)`) shared by both methods.

**Acceptance:** Same behaviour and tests ; no change to paths or wire types.

**Repos:** `cafe-discovery` only.
