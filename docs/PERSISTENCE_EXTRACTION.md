# Persistence extraction (PERS-D1 → D1b)

Scan lifecycle persistence was extracted to **[cafe-persistence](https://github.com/create2-labs/cafe-persistence)** in **PERS-D1**. Stack deploy switched to `oleglod/cafe-persistence` in **PERS-D2**. **PERS-D6d** renamed the compose service to `cafe-persistence` (option A; aligned with platform repo).

**PERS-D1b** removes `cmd/persistence`, `internal/persistence/`, and `Dockerfile-discovery-persistence` from this repository. Production and staging already run the cafe-persistence image; Discovery ships only the backend binary.

## Where persistence lives now

| Concern | Repository / image |
|---------|-------------------|
| NATS consumer, scan DDL, Postgres/Redis writers | [create2-labs/cafe-persistence](https://github.com/create2-labs/cafe-persistence) |
| Docker image (current) | `oleglod/cafe-persistence:${PERSISTENCE_VERSION}` |
| Compose service name | `cafe-persistence` (see `cafe-deploy/compose/20-discovery.yml`) |
| API server (control plane) | This repo — `oleglod/cafe-discovery-backend:${DISCOVERY_VERSION}` |

Normative ADR: [ADR_20260622_persistence.md](./ADR/ADR_20260622_persistence.md).

## Legacy rollback reference

Use this table if you must roll back **after PERS-D1b** (no in-repo rebuild). Rollback is **compose / registry only** — revert Discovery code does not restore `cmd/persistence`.

| Question | Answer |
|----------|--------|
| **Where does the legacy tag live?** | Docker Hub: `oleglod/cafe-discovery-persistence`. Git source (last in-repo tree): commit [`aa49485d23c978015a874cf4b5b6a06efb89d5b6`](https://github.com/create2-labs/cafe-discovery/commit/aa49485d23c978015a874cf4b5b6a06efb89d5b6) on `main` (parent of PERS-D1b). Optional git tag for traceability: `discovery-persistence-v1.1.0-alpha` (last Discovery release that published `cafe-discovery-persistence` via RC/release workflows). |
| **Last known good version pin** | `DISCOVERY_VERSION` aligned with the last RC/release that built `oleglod/cafe-discovery-persistence` (e.g. `v1.1.0-alpha` or the `sha-<short>` tag from commit `aa49485`). Prefer a **pinned digest** from Docker Hub if available. Current stack target: `PERSISTENCE_VERSION` on `oleglod/cafe-persistence` (see `cafe-deploy/env/*.env.template`). |
| **Retention policy** | Keep `oleglod/cafe-discovery-persistence` images on the registry for **90 days** or **3 minor Discovery releases** after PERS-D1b merge — whichever is longer. Do **not** delete legacy tags before that window without ops sign-off. |
| **Rebuild from source** | After D1b: **not supported in this repo**. Rebuild only from commit `aa49485d` (or earlier) in a throwaway branch, or use a published `oleglod/cafe-discovery-persistence:<tag>` image. Dockerfile path on that commit: `Dockerfile-discovery-persistence`. |
| **Rollback procedure** | [cafe-deploy README — rollback persistence image](https://github.com/create2-labs/cafe-deploy/blob/main/README.md) and [RUNBOOK_SCAN_HISTORY.md](https://github.com/create2-labs/cafe-deploy/blob/main/docs/RUNBOOK_SCAN_HISTORY.md) § Rollback. Summary: set `cafe-persistence.image` to `oleglod/cafe-discovery-persistence:${DISCOVERY_VERSION}`, redeploy persistence only, re-run scan smokes. |

## Registry purge ticket

Create an ops ticket to **purge `oleglod/cafe-discovery-persistence` tags** after the retention window (see table above). Suggested title: `Purge legacy cafe-discovery-persistence images post-PERS-D1b`.

## Tests moved with persistence

IMM persistence unit/integration tests (`internal/persistence/*`, `internal/planquota/integration_test.go`) live in **cafe-persistence**. Discovery retains handler/service/repository quota tests (`internal/handler/planquota_*`, `internal/service/plan_quota_*`, ledger repository tests).
