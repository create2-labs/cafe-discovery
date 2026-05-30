# Scan immutability & history — PR plan (Discovery)

> **English product summary:** [CAFE functional specifications](https://github.com/create2-labs/cafe-documentation/blob/main/functional-specifications.md) and [technical specifications](https://github.com/create2-labs/cafe-documentation/blob/main/technical-specifications.md). This file remains the **implementation PR tracker** (mixed FR/EN).

**Propriétaire runtime :** `cafe-discovery` (API, persistence-service, Postgres, Redis cache scan).

**Source de vérité (contrat produit) :** [`cafe-crypto-policy-mgt/workplans/WORKPLAN_API.md`](../cafe-crypto-policy-mgt/workplans/WORKPLAN_API.md) — **§0**, **§2.2** (invariants, couplage **W1–W7**), **§4.2.1**, **§5.4.6**, **§8.4–§8.8**. **Ce fichier** = découpage PR / écarts **implémentation** vs ce workplan.

**All API paths in this document refer to the canonical public prefixes defined in WORKPLAN_API.md: `/api/discovery/v1` and `/api/cpm/v1`.**

**Contrat machine-readable :** `openapi/discovery-v1.yaml` — à aligner sur **W1** (policy **+** draft), **W6**, persistance multi-lignes.

**Contexte CPM :** policies par **`scan_id`** (**PR5/PR6/PR7**). **PR6** (**DELETE** + **409**) reste le modèle cible (**§2.2 W3**) : l’utilisateur supprime d’abord la CPM, puis le scan.

**Règles d’exécution (propriétaire humain) :** l’agent / les contributeurs ne font **pas** de commit, push, merge ni tags ; revue, git et publication restent manuelles. Chaque PR : branche locale, changements ciblés, tests, puis proposition de titre/message de commit et de PR (sections **Proposed** en anglais).

**Statut du document :** **IMM-1**–**IMM-12** largement mergés ; **IMM-6b-1** ([#83](https://github.com/create2-labs/cafe-discovery/pull/83)) livré ; **IMM-6b-2…8** (quota P1/G1–G4) + **IMM-W1-1…3** (CPM DELETE drafts) + **FE-IMM-*** **à faire**.

---

## Règles produit wallet ↔ CPM (référence WORKPLAN §2.2 W1–W7)

**Périmètre wallet ↔ CPM :** scans **wallet / EVM** et CPM **`binding=discovery`** — règles **W1–W7** (assessment/remediation **wallet-only**).

**TLS scope (Discovery-only for current CPM product flow) :**

- **oui** : `scan_id` stable, historique, résultat terminal immutable, list/detail/delete, CBOM optionnel ;
- **oui** : inventaire de risque / observation / historique Discovery ;
- **non** (flux CPM produit actuel) : cible d’assessment ou de remediation CPM ; pas de migration/remediation TLS dans CPM ;
- **défensif** : si un `scan_id` TLS est référencé par une policy persistée, **`DELETE /api/discovery/v1/tls/scans/{scan_id}`** → **`409`** **`SCAN_REFERENCED_BY_POLICY`**.

| WORKPLAN | Règle | Discovery | CPM | Frontend |
|----------|--------|-----------|-----|----------|
| **W7** | **CPM** bloquée si le **dernier scan** n’est pas **`completed`**. **`POST …/scan`** : refus si **en cours** ; retry si **`failed`** **et** **W1** OK | `SCAN_IN_PROGRESS` si en cours ; CPM → **400** | explore/persist → **400** | Scan : off si en cours ou **W1** ; CPM off si dernier ≠ **`completed`** |
| **W1** | **Un seul contexte CPM actif / adresse** : pas de scan si **policy** ou **draft plateforme** | `POST …/scan` → **409** | Lookup policies **+** drafts (**IMM-9b**) | Finaliser / supprimer draft plateforme ; ou export local → delete draft → rescan (UX frontend) |
| **W2** | CPM **uniquement** sur le dernier scan **`completed`** | **`GET …?address=&latest=true`** (≤1 item **`completed`**) | **400** si `scan_id` ≠ latest **`completed`** | Référence policy via **`latest=true`** |
| **W3** | **DELETE scan** : l’utilisateur **supprime d’abord la CPM**, puis **`DELETE …/wallets/scans/{scan_id}`** | **409** `SCAN_REFERENCED_BY_POLICY` tant que policy liée ; **204** après | `GET ?scan_id=` puis `DELETE ?id=` | Guide 409 → supprimer policy → retry DELETE |
| **W4** | **DELETE CPM** sans effet sur les scans | Inchangé | `DELETE …/policies?id=` | Bouton supprimer policy |
| **W5** | **Historique** par adresse | `GET …/wallets/scans?address=` | Lecture | Liste chronologique |
| **W6** | **CBOM** par scan | `GET …/wallets/scans/{scan_id}/cbom` à la demande | Hors scope | Lien CBOM par ligne |

---

## Écarts implémentation vs `WORKPLAN_API.md` (ce plan les comble)

| Sujet | `main` aujourd’hui | Workplan (cible) | PR |
|-------|-------------------|------------------|-----|
| Persistance | 1 ligne / `(user_id, address)` upsert | Multi-lignes, `scan_id` stable | IMM-2, IMM-3 |
| Garde readiness | Pas de blocage | **W7** (CPM : dernier `completed` ; POST : en cours seulement) | IMM-4c, IMM-9, IMM-10 |
| POST scan | Pas de garde CPM / en cours | **W8** (en cours) + **W1** → **409** | IMM-4c, IMM-9 (+ IMM-9b) |
| CPM persist | Tout `scan_id` | **W7** + **W2** | IMM-10 (CPM) |
| DELETE scan | **409** si policy (**PR6**) — **déjà conforme W3** | Conserver ; documenter parcours utilisateur | — (doc/UX) |
| DELETE policy | Ne supprime pas scan | **W4** | — |
| Liste / historique | Filtre `chain_id` → 1 ligne | **W5** | IMM-4a |
| Dernier scan (W2) | Pas de query **`latest=true`** | **`GET …/wallets/scans?address=&latest=true`** | IMM-4b |
| CBOM | Pas de route v1 | **W6** | IMM-12 |

**Implémentation actuelle (`main`) :** non conforme sur **W1, W2, W5, W6, W7** et persistance ; **W3/W4** proches du workplan si **PR6** est actif.

---

## Executive summary

| Domaine | État actuel (`main`) | Cible `WORKPLAN_API.md` |
|--------|----------------------|-------------------------|
| **Identité scan** | Chaque `POST /api/discovery/v1/scan` alloue un **nouveau** `scan_id` (UUID). | Identique. |
| **Persistance wallet** | **Une ligne** par `(user_id, address)` — index unique `idx_scan_results_user_address` ; `WalletWriter` **upsert** et **remplace** `id` + résultat. | **Une ligne par exécution** ; **`scan_id` stable** pour la vie de la ligne ; re-scan = **nouvelle ligne**. |
| **Persistance TLS** | **Une ligne** par `(user_id, url)` — `idx_tls_scan_results_user_url` ; même pattern upsert. | Même règle que wallet (famille TLS). |
| **`result` terminal** | Écrasé sur re-scan (même adresse / même URL). | **Immutable** après terminal pour **ce** `scan_id`. |
| **Liste v1** | `GET …/wallets/scans` sans filtre : `ListOwnerWalletScansDiscoveryV1` (OK multi-lignes **si** DB le permet). Filtre `?address=&chain_id=` : **au plus 1** item via `FindByUserIDAndAddress`. | `?address=` → **toutes** exécutions pour l’adresse ; `?address=&chain_id=` → sous-ensemble **mono-chaîne**, **pas** limité à une seule ligne arbitraire. |
| **POST scan wallet** | Accepté si quota OK. | **W7** : **409** si en cours ; retry si **`failed`** **seulement** si **W1** OK (pas de policy/draft). |
| **CPM explore/persist** | Sans garde lifecycle. | **W7** + **W2** → **400** si dernier ≠ **`completed`** ou mauvais `scan_id`. |
| **DELETE wallet** | **PR6** : `DELETE …/{scan_id}` + **409** si policy. | **W3** inchangé : action user — d’abord `DELETE` policy, puis scan. |
| **CPM persist** | Tout `scan_id` owner valide. | **W7** + **`scan_id` = latest `completed`** (**W2**). |
| **DELETE CPM** | Ne supprime pas le scan (déjà). | Inchangé (**règle 4**). |
| **Quotas plan** | **IMM-6** : row count ; DELETE diminue l’usage. | **IMM-6b** : ledger **success-only** ; garde POST **`successful + in_flight < limit`** + cap **`min(limit, 3)`** ; commit atomique à la complétion ; **P1** monotonic + visibilité effacés. |
| **Redis** | Une clé par adresse. | Latest + historique Postgres ; **IMM-5**. |
| **CBOM** | Route CBOM historique retirée. | **`GET …/wallets/scans/{scan_id}/cbom`** à la demande (**règle 6**). |

**Décision d’architecture :** l’**historique** est porté par **Postgres** (`scan_results`, `tls_scan_results`) avec **`id` = `scan_id`** ; Redis reste un **accélérateur** optionnel, pas la source de vérité v1.

**`latest=true` (W2) :** dernier scan **`completed`** pour l’adresse — peut coexister avec un **`failed`** plus récent dans l’historique (`total: 1` possible). **`total: 0`** seulement s’il n’existe **aucun** **`completed`**.

**Garde readiness (W7) :** **CPM** — refuser explore/persist tant que la ligne la plus récente n’est pas **`completed`**. **`POST …/scan`** — refuser si en cours (`SCAN_IN_PROGRESS`) ; si dernier **`failed`**, retry seulement si **W1** OK (pas de policy **ni** draft).

**W1 (contexte CPM actif) :** au plus **une** policy **ou** draft par adresse — **409** `CPM_EXISTS_FOR_WALLET_TARGET` ; parcours plateforme : finaliser ou supprimer le draft avant rescan. **Parcours client optionnel** (export local → delete draft plateforme → rescan → reload si même adresse + même `wallet_type`) : [`cafe-frontend/IMMUTABILITE.md`](../cafe-frontend/IMMUTABILITE.md) — hors scope API Discovery.

---

## Table de suivi des PR

| PR | Branche (proposée) | Dépôt | PR Git | Dépend de | Objectif |
| --- | --- | --- | --- | --- | --- |
| **IMM-1** | `docs/scan-immutability-gap-and-migration` | `cafe-discovery` | [#64](https://github.com/create2-labs/cafe-discovery/pull/64) | — | Doc : écart vs WORKPLAN, stratégie migration. |
| **IMM-2** | `discovery/scan-history-db-migration` | `cafe-discovery` | [#65](https://github.com/create2-labs/cafe-discovery/pull/65) | **IMM-1** (#64) | DB : retirer unicité par cible. |
| **IMM-3** | `discovery/scan-history-persistence-writers` | `cafe-discovery` | [#66](https://github.com/create2-labs/cafe-discovery/pull/66), [#67](https://github.com/create2-labs/cafe-discovery/pull/67) | **IMM-2** (#65) | Writers : une ligne par `scan_id`. |
| **IMM-4a** | `discovery/scan-history-list-filters` | `cafe-discovery` | [#69](https://github.com/create2-labs/cafe-discovery/pull/69) | **IMM-3** | Liste wallet v1 multi-lignes + retrait lectures wallet mono-ligne par adresse. |
| **IMM-4b** | `discovery/scan-history-latest-completed` | `cafe-discovery` | [#70](https://github.com/create2-labs/cafe-discovery/pull/70) | **IMM-4a** | Query `latest=true` (W2) + OpenAPI. |
| **IMM-4c** | `discovery/block-in-flight-wallet-scan` | `cafe-discovery` | [#71](https://github.com/create2-labs/cafe-discovery/pull/71) | **IMM-4a** | W8 : `POST /scan` → 409 `SCAN_IN_PROGRESS` si scan wallet en cours, y compris `requested`. |
| **IMM-5** | `discovery/scan-history-redis-cleanup` | `cafe-discovery` | [#72](https://github.com/create2-labs/cafe-discovery/pull/72) | **IMM-4a** | Nettoyage Redis résiduel après retrait des lectures wallet mono-ligne. |
| **IMM-6** | `discovery/scan-history-plan-quota-semantics` | `cafe-discovery` | [#73](https://github.com/create2-labs/cafe-discovery/pull/73) | **IMM-3** | Quotas = exécutions scan (row count — **supersédé par IMM-6b** pour l’affichage limite). |
| **IMM-6b-1** | `discovery/plan-usage-ledger-schema` | `cafe-discovery` | [#83](https://github.com/create2-labs/cafe-discovery/pull/83) | **IMM-6** | DDL **`scan_usage_events`** (ledger append-only). |
| **IMM-6b-2** | `discovery/plan-usage-ledger-repo` | `cafe-discovery` | — | **IMM-6b-1** | Repo ledger + comptage **`in_flight`** / **`successful`**. |
| **IMM-6b-3** | `discovery/plan-post-scan-guards` | `cafe-discovery` | — | **IMM-6b-2** | POST : **`successful + in_flight < limit`** + cap **`min(limit, 3)`**. |
| **IMM-6b-4** | `discovery/plan-commit-on-success` | `cafe-discovery` | — | **IMM-6b-2** | Persistence : slot atomique au **`completed` success** ; sinon **`failed`** sans résultat. |
| **IMM-6b-5** | `discovery/plan-usage-api-breakdown` | `cafe-discovery` | — | **IMM-6b-4** | **`GET /plans/usage`** : `used` / `visible` / `deleted_by_user`. |
| **IMM-6b-6** | `discovery/plan-usage-backfill` | `cafe-discovery` | — | **IMM-6b-2** | Backfill ledger depuis lignes **`completed` success** uniquement. |
| **IMM-6b-7** | `discovery/cbom-success-only` | `cafe-discovery` | — | **IMM-12** | CBOM **404** si scan ≠ **`completed` success** (W6 / P1). |
| **IMM-6b-8** | `discovery/plan-quota-integration-tests` | `cafe-discovery` | — | **IMM-6b-3…7** | Tests race, monotonic DELETE, CBOM, stub limit. |
| **IMM-7** | `discovery/scan-history-tests-contract` | `cafe-discovery` | [#75](https://github.com/create2-labs/cafe-discovery/pull/75) | **IMM-3**, **IMM-4a–4c** | Tests + contract v1. |
| **IMM-8** | `deploy/scan-history-migration-runbook` | `cafe-deploy` | [#20](https://github.com/create2-labs/cafe-deploy/pull/20) | **IMM-2** | Runbook déploiement. |
| **IMM-9** | `discovery/block-scan-when-cpm-exists` | `cafe-discovery` (+ CPM interne) | [#76](https://github.com/create2-labs/cafe-discovery/pull/76) | **IMM-4c**, **IMM-9b** | W8 déjà en place + W1 (policy ou draft). |
| **IMM-10** | `cpm/latest-scan-only-policy` | `cafe-crypto-policy-mgt` | [#40](https://github.com/create2-labs/cafe-crypto-policy-mgt/pull/40) | **IMM-4b** | W7 (newest row) + W2 (`latest=true`), wallet-only. |
| **IMM-12** | `discovery/v1-cbom-by-scan-id` | `cafe-discovery` | [#80](https://github.com/create2-labs/cafe-discovery/pull/80) | **IMM-3** | W6 : `GET /wallets/scans/{scan_id}/cbom` à la demande. |
| **IMM-11** | `discovery/remove-obsolete-routes` | `cafe-discovery` (+ edge/deploy) | [#78](https://github.com/create2-labs/cafe-discovery/pull/78) | **IMM-4a–4c**, routes cibles | Retrait routes hors contrat §0, OpenAPI, edge, scripts. |

**Colonnes PR Git / Issue :** **—** = pas encore créée. Ouvrir l’issue sur le dépôt indiqué (**IMM-8** → `cafe-deploy`, les autres → `cafe-discovery`).

---

## Création des issues GitHub

Travail d’**alignement contrat / persistance** (écart `WORKPLAN_API.md` §2.2), pas un rapport de bug utilisateur.

1. Copier le **title** et le **body** de la section **IMM-*n*** ci-dessous.
2. Créer l’issue sur le dépôt **Dépôt (issue)** du tableau (sauf **IMM-8** → `create2-labs/cafe-deploy`).
3. Labels suggérés (adapter au repo) : voir chaque section.
4. Lier l’issue à la PR : `Closes #NNN` dans la PR Git.
5. Coller l’URL de l’issue dans le tableau ci-dessus et dans le bloc **Tracking** en bas de chaque issue.

**Ordre suggéré :**

0. Patch **`WORKPLAN_API.md`** seulement si nécessaire (draft **DELETE**, TLS wallet-only, chemins publics).
1. **IMM-1** — doc migration, décision Redis, contraintes release.
2. **IMM-2** + **IMM-3** — PRs séparées possibles ; **release atomique obligatoire** (voir [§ Release IMM-2 + IMM-3](#release-imm-2--imm-3-unité-atomique)).
3. **IMM-4a** — list filters multi-lignes (`address`, `chain_id`), tri stable, lifecycle enum conforme ; suppression du chemin wallet mono-ligne par adresse.
4. **IMM-4b** — `latest=true` (**W2**) + OpenAPI.
5. **IMM-4c** — garde **W8** sur `POST …/scan` (`SCAN_IN_PROGRESS`, y compris `requested`).
6. **IMM-9b** (CPM) — lookup `target_address` normalisée + policy + draft.
7. **IMM-9** — W1 après W8 ; fail-closed si lookup CPM indisponible.
8. **IMM-10** (CPM) — W7 via newest row ; W2 via `latest=true`.
9. **IMM-12** — CBOM by `scan_id`.
10. **IMM-11** — cleanup routes / edge / OpenAPI / scripts.
12. **IMM-6b-1…8** — ledger success-only + garde POST + commit atomique + CBOM (**P1**) ; **avant** frontend **FE-IMM-0**.
13. **IMM-W1-1…3** (CPM) — **`DELETE /api/cpm/v1/drafts?id=…`**.
14. Frontend **FE-IMM-0…5** — après **IMM-6b-5** + **IMM-W1-3** minimum.
15. **IMM-7** — tests contract E2E (peut chevaucher **IMM-11**).

**IMM-8** (runbook deploy) : en parallèle de **IMM-2**, mis à jour pour release atomique **IMM-2+IMM-3**.

**Prérequis livrés :** list/detail v1 (**PR4**), **PR5/PR6/PR7** (référence policy, **DELETE** + **409**, policies). **PR6** = comportement cible **W3** — **à conserver**.

**Documents liés :** [`cafe-crypto-policy-mgt/workplans/IMMUTABILITE_PR.md`](../cafe-crypto-policy-mgt/workplans/IMMUTABILITE_PR.md) (CPM), [`cafe-frontend/IMMUTABILITE.md`](../cafe-frontend/IMMUTABILITE.md) (UX).

---

## Invariants cibles (Discovery + règles wallet/CPM)

1. **`scan_id`** alloué à l’acceptation (`requested`).
2. **Une exécution** = **une ligne** Postgres (`id` = `scan_id`).
3. **`result`** **gelé** après état terminal **`completed` success** ; **CBOM** (**W6**) **uniquement** si **`completed` success** — **IMM-6b-7**.
4. **Historique** : plusieurs lignes par `target_address` ; listage **`GET …/wallets/scans?address=`** (règle **5**).
5. **W7** : CPM bloquée si dernier scan ≠ **`completed`** ; **`POST …/scan`** bloqué si en cours, ou si **W1** (policy/draft), sinon retry possible si **`failed`**.
6. **W1** : au plus un contexte CPM actif — pas de scan si policy **ou** draft sur la cible.
7. **CPM** : **W7** sur newest row (`limit=1`, tri défaut) ; **W2** sur dernier **`completed`** (`latest=true`).
8. **Effacement scan** : `DELETE …/wallets/scans/{scan_id}` ; **409** si policy référencée ; parcours **W3** (d’abord supprimer CPM).
9. **Effacement CPM** : `DELETE …/policies?id=` sans toucher aux scans (**W4**).
10. **CBOM** : `GET …/wallets/scans/{scan_id}/cbom` — **404** sauf scan **`completed` success** (**IMM-6b-7**).
11. **Quota plan (P1)** : **`wallet_scans_used`** = succès comptabilisés (ledger) ; **ne diminue jamais** (DELETE scan/policy/draft) ; exposer **`visible`** / **`deleted_by_user`** (**IMM-6b-5**).
12. **Anti-bypass quota** : slot atomique à la persistance du success ; si quota dépassé → **`failed`** + **`PLAN_LIMIT_EXCEEDED`**, **adresse conservée**, **résultat jeté** (**IMM-6b-4**).
13. **Garde POST quota** : autoriser démarrage seulement si **`successful + in_flight < limit`** et **`in_flight < min(limit, 3)`** (**IMM-6b-3**).

---

## État actuel (fichiers clés)

| Fichier | Comportement aujourd’hui |
|---------|-------------------------|
| `cmd/persistence/main.go` | **IMM-2** : DDL index au démarrage (drop unique + index liste) ; **IMM-6b-1** : `AutoMigrate` **`scan_usage_events`** + index `(user_id, scan_kind)`. |
| `internal/domain/scan_usage_event.go` | Entity ledger append-only **IMM-6b-1** ; écriture quota en **IMM-6b-4**. |
| `internal/persistence/storage/postgres.go` | ~~`OnConflict` sur `(user_id, address)` / `(user_id, url)`~~ → **IMM-3** : insert/update par `scan_id` (`id` PK). |
| `internal/handler/discovery_v1_scans.go` | Branche `address` + `chain_id` → `FindByUserIDAndAddress` (1 item max) ; à remplacer par liste owner/address. |
| `internal/service/discovery.go` | `getExistingScan` : retourne scan existant → **à supprimer** ; pas de compat production à préserver. |
| `internal/repository/base_repository.go` | `findByUserIDAndField` : `ORDER BY created_at DESC` + `First` (dernier gagnant) ; supprimer si inutilisé après retrait wallet mono-ligne. |

---

## IMM-1 — Gap analysis & stratégie migration (doc)

- **Status:** mergé — PR [#64](https://github.com/create2-labs/cafe-discovery/pull/64)
- **Branch:** `docs/scan-immutability-gap-and-migration`
- **Repository:** `cafe-discovery`
- **Objective:** Figurer la trajectoire **IMM-2…IMM-8** et la politique données pour l’existant (1 row / cible).
- **Livrable doc :** [`docs/SCAN_IMMUTABILITY_MIGRATION.md`](docs/SCAN_IMMUTABILITY_MIGRATION.md) (gap, lifecycle, NATS→Postgres, **no backfill**, Redis, checklist OpenAPI/README).
- **Scope:**
  - Section dans ce fichier ou `docs/SCAN_IMMUTABILITY_MIGRATION.md` : état → cible, diagramme lifecycle.
  - Décision explicite : **pas de rétro-historique** pour les re-scans passés (impossible sans backup) ; première exécution post-migration crée la 2ᵉ ligne.
  - Checklist OpenAPI / README Discovery (lien `WORKPLAN_API.md` §2.2).
- **Out of scope:** Changement SQL ou writers.
- **Dependencies:** Aucune.
- **Tests:** Revue doc uniquement.
- **Proposed commit title:** `docs: scan immutability gap and migration strategy`
- **Proposed PR title:** `Discovery: document scan immutability gap vs WORKPLAN_API`
- **Completion criteria:** Stratégie migration validée en revue ; IMM-2 peut démarrer.

---

## IMM-2 — Migration DB (retirer unicité par cible)

- **Status:** mergé — PR [#65](https://github.com/create2-labs/cafe-discovery/pull/65) (pas d’issue GitHub ; ce plan suffit)
- **Branch:** `discovery/scan-history-db-migration`
- **Repository:** `cafe-discovery`
- **Objective:** Autoriser **plusieurs lignes** par `(user_id, address)` et `(user_id, url)`.
- **Scope:**
  - DDL au démarrage `persistence` dans `cmd/persistence/main.go` : `DROP INDEX IF EXISTS` sur les index uniques legacy ; `CREATE INDEX` liste non uniques (`idx_scan_results_user_address_created_at`, `idx_tls_scan_results_user_url_created_at`).
  - **Pas** de package / script de migration incrémentale — dev : reset volume Postgres si le schéma change brutalement.
  - Note : `id` (UUID) reste **PK** — pas de changement.
- **Out of scope:** Logique `OnStarted` / `OnCompleted`.
- **Dependencies:** **IMM-1**
- **Implementation notes:**
  - Environnements avec **une** ligne par adresse : aucune action données requise avant IMM-3.
  - PostgreSQL **15+** : conserver `NULLS NOT DISTINCT` sur TLS `user_id` si applicable.
- **Tests:** `go build ./cmd/persistence/...` ; pas de tests DDL dédiés (schéma = état cible au boot).
- **Validation commands:** `go build ./cmd/persistence/...` ; vérif manuelle `pg_indexes` après redémarrage persistence.
- **Proposed commit title:** `db: allow multiple scan rows per user target`
- **Proposed PR title:** `Discovery: DB migration for per-execution scan history`
- **Risks:** Voir [§ Release IMM-2 + IMM-3](#release-imm-2--imm-3-unité-atomique) — **pas** de fenêtre prod/staging avec schéma IMM-2 et anciens writers, ni avec writers IMM-3 et anciens index uniques.
- **Completion criteria:** Index unique absents ; index liste présents après boot persistence.

---

## Release IMM-2 + IMM-3 — unité atomique

**IMM-2** et **IMM-3** peuvent être **revues en PRs séparées**, mais doivent être **déployées comme une seule unité** (même fenêtre de release, même train de déploiement).

| Fenêtre interdite | Risque |
|-------------------|--------|
| Schéma **IMM-2** (index uniques retirés) + writers **anciens** (upsert par cible) | État incohérent, doublons ou erreurs PK |
| Writers **IMM-3** (insert par `scan_id`) + index **uniques** encore actifs | Second scan même cible → violation contrainte |

**Règles :**

- Pas de staging/prod avec **IMM-2** seul sans **IMM-3** prêt à déployer immédiatement après.
- Pas de staging/prod avec **IMM-3** sans **IMM-2** déjà appliqué sur la même base.
- **IMM-8** runbook documente l’ordre : migration DB → image persistence **IMM-3** → backend API (**IMM-4+**).

---

## IMM-3 — Persistence writers (insert par scan_id)

- **Status:** mergé 
  — PR [#66](https://github.com/create2-labs/cafe-discovery/pull/66) 
  - PR [#67](https://github.com/create2-labs/cafe-discovery/pull/67) corrige un bug sur la gestion du champ created_at 
- **Branch:** `discovery/scan-history-persistence-writers`
- **Repository:** `cafe-discovery`
- **Objective:** Aligner `WalletWriter` / `TLSWriter` sur **une ligne par `scan_id`**.
- **Scope:**
  - **`OnStarted`:** `INSERT` ligne `id = scan_id`, **`status = started`** (lifecycle API **§5.4.1** — **pas** `RUNNING` exposé API) ; **ne plus** no-op si une autre ligne terminal existe pour la même cible.
  - **`OnCompleted` / `OnFailed`:** `UPDATE … WHERE id = ?` ; si ligne absente, `INSERT` (replay event) — **sans** `ON CONFLICT (user_id, address)`.
  - Transitions invalides : conserver garde `ValidTransition` par **`scan_id`** (logs + ignore duplicate completion).
  - `internal/persistence/handlers/scan_events.go` : ajuster messages / tests.
- **Out of scope:** Handlers HTTP v1 (IMM-4a–4c).
- **Dependencies:** **IMM-2** (sinon INSERT 2ᵉ ligne même adresse → violation unique)
- **Tests:** Unit tests `postgres.go` : deux `scan_id` différents, même `address` → deux lignes ; terminal A inchangé quand B complète.
- **Proposed commit title:** `persistence: one scan row per scan_id (no target upsert)`
- **Proposed PR title:** `Discovery persistence: per-execution scan rows`
- **Risks:** Double `scan.started` même `scan_id` → idempotence PK ; événements hors ordre documentés.
- **Completion criteria:** Re-scan NATS même adresse laisse l’ancienne ligne intacte (status + `result`) ; **aucune** réponse API n’expose `RUNNING` / `running`.

---

## IMM-4a — API v1 wallet list filters (multi-lignes)

- **Status:** mergé — PR [#69](https://github.com/create2-labs/cafe-discovery/pull/69)
- **Branch:** `discovery/scan-history-list-filters`
- **Repository:** `cafe-discovery`
- **Objective:** Corriger les listes wallet v1 pour l'historique multi-lignes (**WORKPLAN §4.2.1**) et retirer les lectures wallet mono-ligne par adresse qui contredisent le modèle immutable, sans introduire encore `latest=true` ni garde POST.
- **Scope:**
  - `ListDiscoveryV1WalletScans` : branche `normalizedAddr != "" && chainID != nil` → lister toutes les lignes owner/address puis filtrer avec `walletEntityMatchesChainID`, **pas** `FindByUserIDAndAddress` seul.
  - `GET …/wallets/scans?address=` : toutes les exécutions pour l'adresse, paginées.
  - Liste par défaut : tri **`created_at`** desc, **`scan_id`** desc — newest row pour **W7** (`limit=1`).
  - Lifecycle API : `requested` \| `started` \| `completed` \| `failed` uniquement (**§5.4.1**), aucune réponse API avec `RUNNING` / `running`.
  - Supprimer le chemin wallet “scan existant par adresse” : `DiscoveryService.getExistingScan` / `ScanWallet` ne doivent plus retourner silencieusement une ligne existante au lieu de créer une nouvelle exécution.
  - Supprimer `ScanResultRepository.FindByUserIDAndAddress` côté wallet Postgres, et le helper `findByUserIDAndField` si aucun autre repository suivi ne l'utilise.
  - Retirer ou neutraliser les appels wallet cache/read-through qui reposent sur une clé unique `(user_id, address)` pour servir une lecture métier hors liste historique v1.
  - OpenAPI : vérifier / ajuster la description du tri par défaut et du filtre `chain_id` si nécessaire.
  - Tests contract : `?address=` multi-lignes ; `?address=&chain_id=` multi-lignes ; `?chain_id=` sans `address` → **400** ; re-scan wallet ne réutilise pas une ancienne ligne par adresse.
- **Out of scope:** `latest=true` (**IMM-4b**) ; garde `POST …/scan` (**IMM-4c**) ; TLS list ; migration Redis complète par `scan_id`.
- **Dependencies:** **IMM-3**
- **Proposed commit title:** `api: fix wallet scan history list filters`
- **Proposed PR title:** `Discovery v1: fix wallet scan list filters for multi-row history`
- **Completion criteria:** listes wallet conformes, tri stable, lifecycle public conforme, aucune lecture wallet mono-ligne par adresse ne court-circuite l'historique immutable.

---

## IMM-4b — API v1 `latest=true` (W2)

- **Status:** mergé — PR [#70](https://github.com/create2-labs/cafe-discovery/pull/70) 
- **Branch:** `discovery/scan-history-latest-completed`
- **Repository:** `cafe-discovery`
- **Objective:** Ajouter **`GET …/wallets/scans?address=&latest=true`** pour **W2** (dernier `completed` uniquement), sans route `/latest`.
- **Scope:**
  - Query **`latest=true`** : exige **`address`** ; retourne ≤1 **`ScanListItem`** en statut **`completed`** uniquement (**W2**).
  - `total` = **0** si aucun scan `completed` n'existe pour l'adresse, même si des lignes `failed` / `requested` / `started` existent.
  - `latest=true&chain_id=N` : latest parmi les scans `completed` matchant la chaîne (retenu par WORKPLAN).
  - **Ne pas** utiliser `latest=true` pour **W7** ; W7 reste `limit=1` + tri par défaut.
  - OpenAPI : paramètre query **`latest`** documenté sur `GET /wallets/scans` avec `address` obligatoire.
  - Tests contract : latest choisit le dernier `completed` ; cas `completed A + failed B plus récent` → latest retourne A ; `latest=true` sans `address` → **400**.
- **Out of scope:** garde `POST …/scan` (**IMM-4c**) ; route `/wallets/scans/latest` (interdite).
- **Dependencies:** **IMM-4a**
- **Proposed commit title:** `api: add latest completed wallet scan query`
- **Proposed PR title:** `Discovery v1: add wallet scan latest=true query`
- **Completion criteria:** `latest=true` conforme et OpenAPI alignée.

---

## IMM-4c — POST scan in-flight guard (W8)

- **Status:** mergé — PR [#71](https://github.com/create2-labs/cafe-discovery/pull/71) 
- **Branch:** `discovery/block-in-flight-wallet-scan`
- **Repository:** `cafe-discovery`
- **Objective:** Refuser un nouveau **`POST /api/discovery/v1/scan`** wallet si un scan wallet est déjà en cours pour la cible (**W8**), y compris l'état initial `requested`.
- **Scope:**
  - `POST …/scan` wallet : **409** `SCAN_IN_PROGRESS` si newest row est `requested` / `started`.
  - Couvrir aussi l'état **`requested`** avant insertion Postgres `started` : le pending v1 doit être consultable par owner + adresse, ou une solution équivalente doit être introduite.
  - Newest `failed` : **pas** de blocage W8 ; le scan est accepté sous réserve de **W1** plus tard (**IMM-9**).
  - Garde W8 avant W1 : IMM-9 réutilise ce helper et ajoute le lookup CPM policy/draft.
  - OpenAPI : réponse **409** documentée sur `POST /scan` avec code **`SCAN_IN_PROGRESS`**.
  - Tests : pending `requested` → 409 ; DB `RUNNING` / API `started` → 409 ; newest `FAILED` → accepté ; TLS hors scope W8.
- **Out of scope:** W1 policy/draft (**IMM-9**) ; CPM W7/W2 (**IMM-10**).
- **Dependencies:** **IMM-4a**
- **Proposed commit title:** `api: block in-flight wallet rescans`
- **Proposed PR title:** `Discovery v1: block wallet scan while latest scan is in progress`
- **Completion criteria:** `POST …/scan` refuse seulement les scans wallet en cours (`requested` / `started`) avec **409** `SCAN_IN_PROGRESS`.

---

## IMM-5 — Redis cleanup post-retrait legacy wallet

- **Status:** mergé dans [#72](https://github.com/create2-labs/cafe-discovery/pull/72)
- **Branch:** `discovery/scan-history-redis-cleanup`
- **Repository:** `cafe-discovery`
- **Objective:** Nettoyer les restes Redis/cache après suppression des lectures wallet mono-ligne par adresse dans **IMM-4a**.
- **Scope:**
  - Supprimer les méthodes wallet cache/read-through devenues inutilisées après **IMM-4a**.
  - Vérifier que Redis n'est plus utilisé comme source métier pour une lecture wallet par `(user_id, address)` hors liste historique v1.
  - `DeleteDiscoveryV1WalletScan` : `redisWalletRepo.DeleteByUserIDAndAddress` — ne supprimer que si plus aucune ligne Postgres pour cette adresse (**IMM-3**).
  - `DELETE …/wallets/scans/{scan_id}` (et TLS) : purger aussi la corrélation pending v1 Redis (`discovery:v1:pending_scan:*`, réservation wallet) pour que **GET** → **404** après effacement du `scan_id` (pas de fantôme `requested`).
- **Out of scope:** Refonte complète Redis par `scan_id` (option future) ; TLS cache ; effacement global par adresse wallet (**pas** de `DELETE` par `WALLET_ADDR`).
- **Dependencies:** **IMM-4a**
- **Proposed commit title:** `fix: clean up wallet redis read paths after scan history migration`
- **Completion criteria:** Pas de suppression Redis qui masque un scan historique encore en Postgres ; après **DELETE** d’un `scan_id`, **GET** détail → **404** (pending v1 nettoyé).

---

## IMM-6 — Quotas plan (sémantique exécutions)

- **Status:** mergé dans [#73](https://github.com/create2-labs/cafe-discovery/pull/73)
- **Branch:** `discovery/scan-history-plan-quota-semantics`
- **Repository:** `cafe-discovery`
- **Objective:** Les limites **wallet** / **TLS** reflètent le nombre d’**exécutions** persistées.
- **Scope:**
  - Confirmer `CountByUserID` / compteurs Redis alignés post IMM-3.
  - `prepareWalletScanQueue` / `prepareTLSScanQueue` : politique « re-scan compte comme nouvelle exécution » — OK si limite atteinte → **403** attendu.
  - Tests `plan` / handler limit.
- **Dependencies:** **IMM-3**
- **Out of scope:** Changement grilles tarifaires produit.
- **Proposed commit title:** `plan: count scan executions for quota after history model`
- **Completion criteria:** Deux scans réussis même adresse → `WalletScansUsed` +2 (si limite non illimitée).

> **Note post-merge IMM-6 :** le comptage par **`CountByUserID`** (lignes Postgres actives) fait **baisser** l’usage affiché après **`DELETE …/scans/{scan_id}`** (soft delete). La règle produit **P1** (usage monotonic) est implémentée par la série **IMM-6b** ci-dessous (**ledger option B**).

---

## IMM-6b — Quotas plan : ledger success-only, garde POST, commit atomique (option B)

**Décisions produit (P1 + garde-fous) :**

| # | Règle | Implémentation |
|---|--------|----------------|
| **P1-a** | **`wallet_scans_used`** / **`endpoint_scans_used`** = scans **`completed` success** comptabilisés (ledger) | Incrément ledger **uniquement** dans la transaction persistence **success** (**IMM-6b-4**) |
| **P1-b** | Usage **ne diminue jamais** (DELETE scan / policy / draft) | Ledger append-only ; soft delete masque l’historique seulement |
| **P1-c** | UX : **`used`**, **`visible`**, **`deleted_by_user`** | **`GET /plans/usage`** (**IMM-6b-5**) : `deleted = used - visible` (success non soft-deleted) |
| **G1** | POST : **`successful + in_flight < limit`** | **`prepareWalletScanQueue`** / TLS (**IMM-6b-3**) |
| **G2** | Cap parallélisme : **`in_flight < min(limit, 3)`** (illimité → cap **3**) | Même PR **IMM-6b-3** |
| **G3** | Quota dépassé **à la complétion** : **`failed`**, code **`PLAN_LIMIT_EXCEEDED`**, **adresse conservée**, **résultat jeté** | **IMM-6b-4** — pas de payload crypto exploitable |
| **G4** | **Pas de CBOM** si scan ≠ **`completed` success** | **IMM-6b-7** — **404** (incl. `failed`, `plan_limit_exceeded`, in-flight) |

**Définitions (par `scan_kind` wallet | endpoint) :**

- **`successful`** = `COUNT(scan_usage_events)` ou lignes terminal **`completed` success` comptées.
- **`in_flight`** = lignes owner avec lifecycle **`requested` \| `started`** (API ; RUNNING interne) **non** terminal.
- **`visible`** = lignes **`completed` success** actives (non soft-deleted) en Postgres.

**Écart IMM-6 :** comptage row-based + incrément implicite à l’acceptation — **remplacé** par ce modèle. **Ne pas** utiliser **`CountByUserID`** pour les limites après **IMM-6b-5**.

**Flux cible :**

```text
POST /scan
  → G1: successful + in_flight < limit ?
  → G2: in_flight < min(limit, 3) ?
  → W8/W1… → publish scan.started

scan.completed (persistence)
  → BEGIN
  → try_acquire_success_slot(user, kind)   -- atomic: used < limit
  → IF slot:
        write SUCCESS + full result + ledger event (scan_id)
    ELSE:
        write FAILED + PLAN_LIMIT_EXCEEDED + address kept + NO result payload
        NO ledger event
  → COMMIT

DELETE scan (success row)
  → soft delete ; ledger unchanged ; used unchanged ; visible decreases
```

### IMM-6b-1 — Schéma DB ledger

- **Status:** livré — PR [#83](https://github.com/create2-labs/cafe-discovery/pull/83)
- **Branch:** `discovery/plan-usage-ledger-schema`
- **Repository:** `cafe-discovery`
- **Objective:** Table **`scan_usage_events`** : `(id, user_id, scan_id UUID UNIQUE, scan_kind, consumed_at)`.
- **Scope:** DDL boot persistence ; index `(user_id, scan_kind)` ; entity GORM si partagée backend.
- **Out of scope:** Logique POST/persistence.
- **Dependencies:** **IMM-6** (mergé)
- **Proposed commit title:** `db: add scan_usage_events ledger for success-only plan quota`
- **Completion criteria:** Table + indexes ; build persistence OK.

### IMM-6b-2 — Repository ledger + compteurs plan

- **Status:** à faire
- **Branch:** `discovery/plan-usage-ledger-repo`
- **Repository:** `cafe-discovery`
- **Objective:** API repository :
  - **`RecordSuccessUsage(userID, scanID, kind)`** — idempotent (`ON CONFLICT scan_id DO NOTHING`)
  - **`CountSuccessUsage(userID, kind)`**
  - **`CountInFlightScans(userID, kind)`** — lifecycle non terminal
  - **`CountVisibleSuccessScans(userID, kind)`** — success actifs (non deleted)
  - **`TryAcquireSuccessSlot(userID, kind, limit)`** — atomique (transaction) ; retourne bool
- **Scope:** `internal/repository/` ; tests unitaires idempotence + in_flight.
- **Dependencies:** **IMM-6b-1**
- **Proposed commit title:** `plan: scan usage ledger and plan counter queries`
- **Completion criteria:** Tests verts ; **`TryAcquireSuccessSlot`** empêche dépassement sous concurrence simulée.

### IMM-6b-3 — Garde POST (`successful + in_flight` + cap parallèle)

- **Status:** à faire
- **Branch:** `discovery/plan-post-scan-guards`
- **Repository:** `cafe-discovery`
- **Objective:** Remplacer **`CheckScanLimit`** row-count par :
  1. **`successful + in_flight < limit`** (limit illimité → pas de check G1)
  2. **`in_flight < min(limit, 3)`** (limit 0/unlimited → **`in_flight < 3`**)
- **Scope:** `prepareWalletScanQueue`, `prepareTLSScanQueue`, `checkScanLimits` ; **403** message existant ou enrichi ; OpenAPI erreur plan si documenté.
- **Out of scope:** Persistence completed ; CBOM.
- **Dependencies:** **IMM-6b-2**
- **Proposed commit title:** `plan: post scan guards on successful plus in-flight and parallel cap`
- **Completion criteria:** À limit=5, used=4, in_flight=1 → POST **403** ; in_flight=0 → POST OK ; ≥3 in_flight avec cap → **403** même si quota restant.

### IMM-6b-4 — Commit atomique au success (persistence)

- **Status:** à faire
- **Branch:** `discovery/plan-commit-on-success`
- **Repository:** `cafe-discovery` (persistence-service + éventuellement handler)
- **Objective:** Dans **`handleWalletCompleted`** / **`handleTLSCompleted`** :
  - Transaction : **`TryAcquireSuccessSlot`** → si OK : **`OnCompleted`** + **`RecordSuccessUsage`**
  - Sinon : **`OnFailed`** (ou update) avec **`PLAN_LIMIT_EXCEEDED`**, **conserver `address`/`url`**, **aucun champ `result` exploitable** (networks vides, pas d’exposition clé, pas de posture)
- **Scope:** `internal/persistence/handlers/scan_events.go`, `storage/postgres.go` ; pas d’écriture Redis riche si rejet quota.
- **Out of scope:** **`GET /plans/usage`** breakdown.
- **Dependencies:** **IMM-6b-2**
- **Proposed commit title:** `plan: atomic success slot on scan completed with stripped over-limit failure`
- **Completion criteria:** 2 completions concurrentes à limit−1 → 1 success riche + 1 failed stub ; stub **GET detail** sans données bypass ; ledger +1 seulement pour le success.

### IMM-6b-5 — API `GET /plans/usage` (used / visible / deleted)

- **Status:** à faire
- **Branch:** `discovery/plan-usage-api-breakdown`
- **Repository:** `cafe-discovery`
- **Objective:** Enrichir **`PlanUsage`** :
  - **`wallet_scans_used`** / **`endpoint_scans_used`** ← ledger success count
  - **`wallet_scans_visible`** / **`endpoint_scans_visible`** ← visible success rows
  - **`wallet_scans_deleted_by_user`** / **`endpoint_scans_deleted_by_user`** ← `used - visible`
  - **`wallet_scans_in_flight`** / **`endpoint_scans_in_flight`** (optionnel, UX/debug)
- **Scope:** `internal/service/plan.go`, `internal/handler/plan.go`, README/OpenAPI plans.
- **Dependencies:** **IMM-6b-4**
- **Proposed commit title:** `plan: expose used visible and deleted scan counts in usage API`
- **Completion criteria:** DELETE success scan → `used` stable, `visible` −1, `deleted` +1 ; docs champs JSON.

### IMM-6b-6 — Backfill ledger (success historiques)

- **Status:** à faire
- **Branch:** `discovery/plan-usage-backfill`
- **Repository:** `cafe-discovery`
- **Objective:** Insérer events pour lignes **`status = success/completed`** existantes (**incl. soft-deleted**).
- **Scope:** Job one-shot idempotent au boot ou script maintenance.
- **Out of scope:** Lignes **`failed`**, **`plan_limit_exceeded`**, in-flight.
- **Dependencies:** **IMM-6b-2**
- **Proposed commit title:** `plan: backfill scan usage ledger from historical successes`
- **Completion criteria:** `used` post-backfill ≥ succès Postgres ; pas de doublons **`scan_id`**.

### IMM-6b-7 — CBOM success-only (W6 / P1)

- **Status:** à faire
- **Branch:** `discovery/cbom-success-only`
- **Repository:** `cafe-discovery`
- **Objective:** **`GET …/wallets/scans/{scan_id}/cbom`** et TLS sibling → **404** (ou **403** documenté) sauf scan **`completed` success** owner.
- **Scope:** Handler CBOM v1 ; OpenAPI ; tests contract.
- **Dependencies:** **IMM-12** (route existante)
- **Proposed commit title:** `discovery: cbom only for completed successful scans`
- **Completion criteria:** `failed`, in-flight, over-limit stub → **404** ; success → **200** CBOM.

### IMM-6b-8 — Tests intégration quota + race

- **Status:** à faire
- **Branch:** `discovery/plan-quota-integration-tests`
- **Repository:** `cafe-discovery`
- **Objective:** Couvrir **G1–G4**, **P1-b**, parallélisme, CBOM.
- **Scope:** Tests handler + persistence + plan ; script smoke optionnel.
- **Dependencies:** **IMM-6b-3** … **IMM-6b-7**
- **Proposed commit title:** `test: plan quota guards commit race and cbom success-only`
- **Completion criteria:**
  - [ ] POST blocked when `successful + in_flight >= limit`
  - [ ] POST blocked when `in_flight >= min(limit, 3)`
  - [ ] Concurrent complete at limit → one rich success, one stub, ledger +1
  - [ ] DELETE success → used unchanged, visible down
  - [ ] CBOM 404 on non-success

---

## IMM-7 — Tests & contract v1

- **Status:** mergé dans [#75](https://github.com/create2-labs/cafe-discovery/pull/75)- **Branch:** `discovery/scan-history-tests-contract`
- **Repository:** `cafe-discovery`
- **Objective:** Verrouiller le comportement pour CPM Option A et scripts smoke v1.
- **Scope:**
  - Test intégration persistence : enchaînement `started` → `completed` × 2 `scan_id`, même address.
  - Contract `wallet_scans_v1` : liste `total >= 2`, détail ancien `scan_id` toujours **200** + `result` inchangé.
  - Mettre à jour `docs/CPM_OPTION_A_DISCOVERY_V1_CONTRACT.md` si une phrase impliquait implicitement 1 scan / adresse.
- **Dependencies:** **IMM-3**, **IMM-4a–4c**
- **Proposed commit title:** `test: scan immutability and multi-row history per address`
- **Completion criteria:** `go test ./internal/contract/... ./internal/persistence/...` vert en CI.

---

## IMM-8 — Deploy runbook (cafe-deploy)

- **Status:** mergé dans [#20](https://github.com/create2-labs/cafe-deploy/pull/20)
- **Branch:** `discovery/scan-history-tests-contract`
- **Branch:** `deploy/scan-history-migration-runbook`
- **Repository:** `cafe-deploy`
- **Objective:** Procédure release safe pour IMM-2 + IMM-3.
- **Scope:**
  - README ou `docs/RUNBOOK_SCAN_HISTORY.md` : ordre images `cafe-discovery-persistence` puis `cafe-discovery-backend` ; backup Postgres recommandé.
  - Note release : anciens `scan_id` référencés CPM **restent** valides ; re-scan **ne casse plus** les références existantes.
  - Option : étendre `scripts/test-discovery-v1-wallet-scans-to-cpm.sh` (CPM) avec 2ᵉ scan même adresse (si script dans deploy ou CPM — lien croisé).
- **Dependencies:** **IMM-2**
- **Out of scope:** Changement compose obligatoire si migration auto au boot persistence.
- **Proposed commit title:** `docs: runbook for Discovery scan history migration`
- **Completion criteria:** Runbook relu ; fenêtre déploiement documentée.

---

## IMM-9 — Gardes POST scan (en cours + **W1**)

- **Status:** mergé dans [#76](https://github.com/create2-labs/cafe-discovery/pull/76)
- **Dépôts :** `cafe-discovery` ; appel CPM interne (**IMM-9b**) pour **W1**.
- **Endpoint :** `POST /api/discovery/v1/scan` avec `{ "address": "0x…" }` (wallet only pour **W1**).
- **Ordre des gardes :**
  1. Newest row `requested` / `started` → **409** `SCAN_IN_PROGRESS` (**IMM-4c**).
  2. Lookup CPM par **`target_address`** normalisée (**IMM-9b**) — policy **ou** draft → **409** `CPM_EXISTS_FOR_WALLET_TARGET`.
  3. Fail-closed si lookup CPM indisponible (pas de scan silencieux).
- **Retry après `failed` :** accepté **seulement** si **W1** OK (aucune policy, aucun draft). Sinon : finaliser (`POST /api/cpm/v1/policies`) ou supprimer draft (`DELETE /api/cpm/v1/drafts?id=…`) / policy (`DELETE /api/cpm/v1/policies?id=…`).
- **Dépend de :** IMM-3, **IMM-4c**, **IMM-9b** (CPM, avant cette PR ou en parallèle coordonné).

---

## IMM-10 — CPM explore/persist (**W7** + **W2**)

- **Status:** mergé dans [#40](https://github.com/create2-labs/cafe-crypto-policy-mgt/pull/40) 
- **Dépôt principal :** `cafe-crypto-policy-mgt` — [`workplans/IMMUTABILITE_PR.md`](../cafe-crypto-policy-mgt/workplans/IMMUTABILITE_PR.md) § IMM-10.
- **Wallet-only** — pas d’assessment/remediation TLS.
- **Step 1 — W7 :** `GET /api/discovery/v1/wallets/scans?address=…&limit=1` (newest row) ; si `status != completed` → **400** `LATEST_SCAN_NOT_COMPLETED`.
- **Step 2 — W2 :** `GET /api/discovery/v1/wallets/scans?address=…&latest=true` ; si `scan_id` demandé ≠ latest **`completed`** → **400** `SCAN_ID_NOT_LATEST_FOR_TARGET`.
- **Ne pas** utiliser `latest=true` pour **W7** ; **ne pas** utiliser `limit=1` seul pour **W2**.

---

## IMM-11 — Remove obsolete routes, OpenAPI, edge mappings and scripts

- **Livré dans cette branche (complément PR54/57)** : régression **`TestDiscoveryLegacyRoutesRemoved_IMM11`** + constantes **`internal/discoveryroutes/legacy_removed.go`** ; smoke deploy **`discovery-v1-http-smoke.sh`** (GET listes/context legacy → **404**) ; script bench TLS sur **`GET /discovery/v1/tls/scans`**.

- **Branch:** `discovery/remove-obsolete-routes`
- **Repositories:** `cafe-discovery`, `cafe-deploy` (edge/nginx), scripts/tests frontend si applicable.
- **Objective:** Retirer les surfaces obsolètes **après** disponibilité des routes cibles (**IMM-4a–4c**, **IMM-12**, CPM **§0.2**) — **avant** de considérer la chaîne IMM terminée.
- **Scope:**
  - Retirer ou confirmer le retrait de l’ancien **la liste wallet historique** (liste par adresse).
  - Retirer ou confirmer le retrait de l’ancien **la liste TLS historique** (liste par URL).
  - Retirer les routes de détail ambiguës par adresse ou URL en path.
  - Retirer **CBOM historique** si encore référencé.
  - Décider explicitement du sort de **la façade policy-context historique** (supprimer si redondant avec `wallets/scans` + `GET /api/cpm/v1/policies?scan_id=`).
  - Nettoyer OpenAPI, README, edge/nginx, scripts smoke, tests frontend/backend.
- **Dependencies:** IMM-4a–4c, IMM-12 ; CPM routes stables ; **IMM-9**/**IMM-10** pour erreurs documentées.
- **Out of scope:** Nouvelles fonctionnalités API.
- **Proposed commit title:** `chore: remove obsolete discovery routes and align edge/OpenAPI`
- **Proposed PR title:** `Discovery IMM-11: remove obsolete routes, edge mappings and scripts`
- **Completion criteria:** Routes hors contrat §0 non routées ; OpenAPI et edge alignés **§0** ; tests QA mis à jour.

---

## IMM-12 — CBOM par scan_id (W6)

- **API :** `GET /api/discovery/v1/wallets/scans/{scan_id}/cbom` (`ToCBOM()` à la demande).
- **Dépend de :** IMM-3.

---

## Critères d’acceptation produit (fin de chaîne)

|  | Domaine | Critère | Validation (test/script) |
|---|---|---|---|
| ✅ | Historique & immutabilité (W5–W6) | Sans CPM sur la cible : deux scans successifs → deux `scan_id` dans `GET /api/discovery/v1/wallets/scans?address=…`. | `internal/contract/wallet_scans_v1_test. go::TestDiscoveryV1WalletScans_twoExecutionsSameAddress_listContainsBoth` |
| ✅ | Historique & immutabilité (W5–W6) | `GET /api/discovery/v1/wallets/scans/{scan_id}` : `result` stable après terminal ; `GET …/cbom` renvoie le CBOM de ce scan uniquement. | `internal/contract/wallet_scans_v1_test.go::TestDiscoveryV1WalletScans_historicalScanIDDetailIsStable`; `internal/handler/discovery_v1_cbom_test.go`; `cafe-deploy/scripts/test-discovery-imm12-wallet-scan-cbom.sh` |
| ✅ | Historique & immutabilité (W5–W6) | Aucune réponse API n’expose `RUNNING`/`running` (lifecycle: `started`). | `internal/contract/wallet_scans_v1_test.go::TestDiscoveryV1WalletScans_addressFilterReturnsAllExecutions` |
| ✅ | CPM & scan (W7, W1, W2) | `POST /api/discovery/v1/scan` : newest `requested` \| `started` → `409 SCAN_IN_PROGRESS`. | `internal/handler/discovery_scan_v1_test.go::TestPostDiscoveryScanV1_WalletPendingRequested409`; `...WalletRunningRow409` |
| ✅ | CPM & scan (W7, W1, W2) | Newest `failed` + no policy/draft → scan accepté ; CPM explore/persist → `400 LATEST_SCAN_NOT_COMPLETED`. | `internal/handler/discovery_scan_v1_test.go::TestPostDiscoveryScanV1_WalletFailedNewestAccepted`; `cafe-deploy/scripts/test-cpm-imm10-wallet-scan-w7-w2-guards.sh` |
| ⏳ | CPM & scan (W7, W1, W2) | Newest `failed` + draft/policy → `409 CPM_EXISTS_FOR_WALLET_TARGET` jusqu’à `DELETE /api/cpm/v1/drafts?id=…` / finalize policy. | partiel: `internal/handler/discovery_scan_v1_test.go::TestPostDiscoveryScanV1_WalletFailedNewestBlockedByCPMDraft`; `cafe-deploy/scripts/test-discovery-imm9-wallet-scan-w1-cpm-block.sh` |
| ⏳ | CPM & scan (W7, W1, W2) | `completed` A + `failed` B (B newer): CPM `400 LATEST_SCAN_NOT_COMPLETED` même pour `scan_id` A ; `POST …/scan` OK seulement si W1 OK. | `cafe-deploy/scripts/test-cpm-imm10-wallet-scan-w7-w2-guards.sh` |
| ✅ | CPM & scan (W7, W1, W2) | W7 CPM : newest row via `limit=1` (pas `latest=true`). | `cafe-crypto-policy-mgt/internal/app/authz_scan_test.go::TestWithAuthentication_IMM10_W7RejectsWhenNewestIsFailed`; `cafe-deploy/scripts/test-cpm-imm10-wallet-scan-w7-w2-guards.sh` |
| ⏳ | CPM & scan (W7, W1, W2) | W2 CPM : `GET …/wallets/scans?address=&latest=true` (pas `limit=1` seul). | `cafe-crypto-policy-mgt/internal/app/authz_scan_test.go::TestWithAuthentication_IMM10_W2RejectsHistoricalScanID`; `cafe-deploy/scripts/test-cpm-imm10-wallet-scan-w7-w2-guards.sh` |
| ✅ | CPM & scan (W7, W1, W2) | Historical `scan_id` → `400 SCAN_ID_NOT_LATEST_FOR_TARGET`. | `cafe-crypto-policy-mgt/internal/app/authz_scan_test.go::TestWithAuthentication_IMM10_W2RejectsHistoricalScanID`; `cafe-deploy/scripts/test-cpm-imm10-wallet-scan-w7-w2-guards.sh` |
| ⏳ | CPM & scan (W7, W1, W2) | `DELETE /api/cpm/v1/drafts?id=…` contractualisé ; débloque rescan après suppression draft. | `cafe-deploy/scripts/test-discovery-imm9-wallet-scan-w1-cpm-block.sh` |
| ⏳ | CPM & scan (W7, W1, W2) | UX : export local → delete draft → scan → reload. | N/A backend (validation frontend/manuelle) |
| ⏳ | TLS | TLS : historique Discovery + CBOM optionnel ; pas de cible CPM assessment/remediation produit actuel. | `cafe-crypto-policy-mgt/internal/app/assessment_request_test.go::TestPoliciesAssessmentRequest_tlsScanNotFound`; `cafe-crypto-policy-mgt/internal/api/wallet_scan_detail_test.go::TestObservationPayloadFromDiscoveryWalletScanDetail_tlsRejected`; `cafe-crypto-policy-mgt/internal/app/authz_scan_test.go::TestWithAuthentication_IMM10_ExploreRejectsTLSScanIDAsNotFound`; `...PersistRejectsTLSScanIDAsNotFound` |
| ✅ | Suppression (W3–W4) | `DELETE /api/cpm/v1/policies?id=` : scan(s) toujours listables. | `cafe-deploy/scripts/test-discovery-w3-w4-scan-policy-delete.sh` |
| ✅ | Suppression (W3–W4) | `DELETE /api/discovery/v1/wallets/scans/{scan_id}` avec policy liée → `409 SCAN_REFERENCED_BY_POLICY`. | `cafe-deploy/scripts/test-discovery-w3-w4-scan-policy-delete.sh` |
| ✅ | Suppression (W3–W4) | Après suppression policies : DELETE scan → `204`. | `cafe-deploy/scripts/test-discovery-w3-w4-scan-policy-delete.sh` |
| ✅ | Cleanup (IMM-11) | Routes hors contrat §0 retirées (code + edge + OpenAPI). | `cafe-deploy/scripts/lib/discovery-v1-http-smoke.sh` |
| ✅ | Doc | `WORKPLAN_API.md` §2.2 W1–W7, §4.2.1, §5.4.6 à jour (source de vérité). | Vérification documentaire manuelle |

---

## GitHub issue — IMM-1

### Title (copy as-is)

```
[Discovery][IMM-1] Document scan immutability gap and migration strategy
```

### Labels (suggested)

`documentation` · `discovery` · `scan-history` · `contract-alignment`

### Repository

`create2-labs/cafe-discovery`

### Body (copy below the line)

---

**Type:** Technical task (contract alignment) — *tracks gap vs product spec, not an end-user bug.*

**Tracking ID:** IMM-1  
**Plan:** [IMMUTABILITE_PR.md — IMM-1](https://github.com/create2-labs/cafe-discovery/blob/main/IMMUTABILITE_PR.md#github-issue--imm-1)  
**Product reference:** [`WORKPLAN_API.md` §2.2 / §4.2.1](https://github.com/create2-labs/cafe-crypto-policy-mgt/blob/main/workplans/WORKPLAN_API.md)

### Summary

Discovery **v1 HTTP** exposes `scan_id` and documents immutable `result` after terminal states, but **persistence** still upserts wallet/TLS rows by **`(user_id, address)`** or **`(user_id, url)`**, replacing the previous `scan_id` on re-scan. This issue delivers **documentation only**: gap analysis, target invariants, and an explicit **data migration policy** before IMM-2 changes the database.

### Current vs target

| Topic | Current (`main`) | Target (`WORKPLAN_API.md`) |
|-------|------------------|----------------------------|
| Rows per target | At most one Postgres row per user + address/URL | One row **per scan execution** (`id` = `scan_id`) |
| Re-scan same `0x…` | Overwrites row and `scan_id` | New row, new `scan_id`; old row unchanged |
| Lost history | Prior `scan_id` may 404 for CPM policies | Prior `scan_id` remains readable until DELETE |

### Acceptance criteria

- [ ] Doc added (this plan section and/or `docs/SCAN_IMMUTABILITY_MIGRATION.md`) describing gap, lifecycle, and NATS → persistence flow.
- [ ] Explicit decision: **no backfill** of historical re-scans already overwritten (only forward behavior after IMM-2+3).
- [ ] Redis role documented: cache vs Postgres source of truth for v1 list/detail.
- [ ] README or maintainer note links `WORKPLAN_API.md` §2.2 and `IMMUTABILITE_PR.md` sequence IMM-2…IMM-8.
- [ ] Review sign-off recorded (comment on issue or PR) before IMM-2 starts.

### Out of scope

- SQL migrations, writer code, API handler changes.

### Dependencies

- None.

### Implementation hints

- Key files today: `cmd/persistence/main.go`, `internal/persistence/storage/postgres.go`, `IMMUTABILITE_PR.md` executive summary.

**Suggested branch:** `docs/scan-immutability-gap-and-migration`  
**Suggested PR title:** `Discovery: document scan immutability gap vs WORKPLAN_API`

### Test plan

- Doc review only.

### Tracking

| Field | Value |
|-------|--------|
| Issue | — (non créée ; plan IMMUTABILITE_PR suffit) |
| PR | [#64](https://github.com/create2-labs/cafe-discovery/pull/64) |

---

## GitHub issue — IMM-2

### Title (copy as-is)

```
[Discovery][IMM-2] DB migration: allow multiple scan rows per user target
```

### Labels (suggested)

`discovery` · `scan-history` · `database` · `migration`

### Repository

`create2-labs/cafe-discovery`

### Body (copy below the line)

---

**Type:** Technical task (database schema).

**Tracking ID:** IMM-2  
**Plan:** [IMMUTABILITE_PR.md — IMM-2](https://github.com/create2-labs/cafe-discovery/blob/main/IMMUTABILITE_PR.md#github-issue--imm-2)  
**Depends on:** IMM-1 (migration strategy approved — PR [#64](https://github.com/create2-labs/cafe-discovery/pull/64))

### Summary

Remove unique indexes that enforce **one scan row per `(user_id, address)`** and **per `(user_id, url)`**, so IMM-3 can insert a new row for each `scan_id` without conflicting. Add non-unique indexes to keep list/filter queries efficient.

### Acceptance criteria

- [ ] `DROP` (or equivalent) `idx_scan_results_user_address` and `idx_tls_scan_results_user_url`.
- [ ] Non-unique indexes for listing by user + address/url (names defined in PR).
- [ ] `cmd/persistence/main.go` no longer recreates **unique** indexes on startup; creates list indexes instead.
- [ ] DDL index appliqué au boot persistence (pas de couche migration dédiée).
- [ ] Existing single row per target remains valid (no data rewrite required).

### Out of scope

- Changing `WalletWriter` / `TLSWriter` logic (IMM-3).
- HTTP API behavior (IMM-4a–4c).

### Risks

- Deploying IMM-2 **without** IMM-3 while writers still upsert by target may cause duplicate-key attempts or inconsistent state — coordinate via IMM-8 runbook.

### Implementation hints

- `cmd/persistence/main.go` (index creation)
- GORM `AutoMigrate` unchanged for entity shape; PK remains `id` UUID

**Suggested branch:** `discovery/scan-history-db-migration`  
**Suggested PR title:** `Discovery: DB migration for per-execution scan history`

### Test plan

- [ ] `go build ./cmd/persistence/...`
- [ ] Manual: `pg_indexes` après boot ; second insert même `(user_id, address)` + `id` différent OK (après IMM-3 pour le comportement writer)

### Tracking

| Field | Value |
|-------|--------|
| Issue | — (non créée ; plan IMMUTABILITE_PR suffit) |
| PR | — (branche `discovery/scan-history-db-migration`) |

---

## GitHub issue — IMM-3

### Title (copy as-is)

```
[Discovery][IMM-3] Persistence: one Postgres row per scan_id (no target upsert)
```

### Labels (suggested)

`discovery` · `scan-history` · `persistence` · `nats`

### Repository

`create2-labs/cafe-discovery`

### Body (copy below the line)

---

**Type:** Technical task (persistence / contract alignment).

**Tracking ID:** IMM-3  
**Plan:** [IMMUTABILITE_PR.md — IMM-3](https://github.com/create2-labs/cafe-discovery/blob/main/IMMUTABILITE_PR.md#github-issue--imm-3)  
**Depends on:** IMM-2 (unique indexes removed)

### Summary

Refactor `WalletWriter` and `TLSWriter` so each NATS scan lifecycle operates on **`scan_id` as row primary key**: insert on start, update by `id` on complete/fail. Stop using `ON CONFLICT (user_id, address|url)` that overwrites prior executions.

### Acceptance criteria

- [ ] `OnStarted`: inserts row with `id = scan_id`, **`status = started`** (lifecycle API — **not** `RUNNING` / `running` in API responses).
- [ ] `OnCompleted` / `OnFailed`: update `WHERE id = scan_id`; optional insert-if-missing for event replay without target-level upsert.
- [ ] Re-scan same wallet address: **two** rows with distinct `scan_id`; first row’s `result` unchanged when second completes.
- [ ] Same behavior for TLS scans (per `url`).
- [ ] Invalid transition handling remains per `scan_id` (duplicate completion ignored with log).
- [ ] Unit tests in `internal/persistence/storage` cover two `scan_id`s, one address.

### Out of scope

- `GET /api/discovery/v1/wallets/scans` filter fix (IMM-4a).
- Redis / legacy API paths (IMM-5).

### Implementation hints

| File | Change |
|------|--------|
| `internal/persistence/storage/postgres.go` | Remove target upsert; update by PK |
| `internal/persistence/handlers/scan_events.go` | Adjust tests/logs if needed |

**Suggested branch:** `discovery/scan-history-persistence-writers`  
**Suggested PR title:** `Discovery persistence: per-execution scan rows`

### Test plan

- [ ] `go test ./internal/persistence/...`
- [ ] Optional: wallet NATS chain smoke after local stack up

### Tracking

| Field | Value |
|-------|--------|
| Issue | — |
| PR | — |

---

## GitHub issue — IMM-4a

### Title (copy as-is)

```
[Discovery][IMM-4a] v1 wallet scan list filters for multi-row history
```

### Labels (suggested)

`discovery` · `scan-history` · `api` · `v1`

### Repository

`create2-labs/cafe-discovery`

### Body (copy below the line)

---

**Type:** Technical task (API bugfix vs WORKPLAN).

**Tracking ID:** IMM-4a  
**Plan:** [IMMUTABILITE_PR.md — IMM-4a](https://github.com/create2-labs/cafe-discovery/blob/main/IMMUTABILITE_PR.md#github-issue--imm-4a)  
**Depends on:** IMM-3 (multiple rows per address possible)

### Summary

Implement **WORKPLAN_API.md §4.2.1** list filters on **`GET …/wallets/scans`** for multi-row scan history, and remove wallet read paths that still assume a single result per `(user_id, address)`.

### Acceptance criteria

- [ ] `?address=` only: all executions for address, paginated.
- [ ] `?address=&chain_id=`: list query + `walletEntityMatchesChainID`; **not** single-row lookup only.
- [ ] No API response exposes `RUNNING` / `running` (lifecycle: `started`).
- [ ] `?chain_id=` without `address` → **400** (unchanged).
- [ ] Default sort: `created_at` desc, `scan_id` desc.
- [ ] `DiscoveryService.getExistingScan` / `ScanWallet` no longer short-circuit a new wallet execution by returning an existing address row.
- [ ] `ScanResultRepository.FindByUserIDAndAddress` is removed for wallet scan results; any generic helper used only by that path is removed too.
- [ ] Wallet cache/read-through calls that depend on a unique `(user_id, address)` result are removed or neutralized outside the v1 historical list path.
- [ ] OpenAPI: list/filter descriptions aligned if needed.
- [ ] Contract tests: address history, address+chain history, chain-only 400, wallet re-scan does not reuse an old address row.

### Out of scope

- `latest=true` (**IMM-4b**).
- `POST /scan` in-flight guard (**IMM-4c**).
- TLS list (no address filter).

### Implementation hints

- `internal/handler/discovery_v1_scans.go` — `ListDiscoveryV1WalletScans`
- `internal/service/discovery.go` — remove wallet existing-scan short-circuit
- `internal/repository/scan_result_repository.go` — replace single-row address lookup with historical list helper
- `internal/service/user_scan_cache.go` — remove/neutralize wallet address read-through if still reachable
- `internal/contract/wallet_scans_v1_test.go`

**Suggested branch:** `discovery/scan-history-list-filters`  
**Suggested PR title:** `Discovery v1: fix wallet scan list filters for multi-row history`

### Test plan

- [ ] `go test ./internal/handler/... ./internal/contract/...`

### Tracking

| Field | Value |
|-------|--------|
| Issue | — |
| PR | — |

---

## GitHub issue — IMM-4b

### Title (copy as-is)

```
[Discovery][IMM-4b] v1 wallet scans: latest=true returns latest completed scan
```

### Labels (suggested)

`discovery` · `scan-history` · `api` · `v1`

### Repository

`create2-labs/cafe-discovery`

### Body (copy below the line)

---

**Type:** Technical task (API contract / W2).

**Tracking ID:** IMM-4b  
**Plan:** [IMMUTABILITE_PR.md — IMM-4b](https://github.com/create2-labs/cafe-discovery/blob/main/IMMUTABILITE_PR.md#github-issue--imm-4b)  
**Depends on:** IMM-4a

### Summary

Add **`?latest=true`** on **`GET …/wallets/scans`** for **W2**: return the latest **`completed`** scan only. Do not add a `/latest` route.

### Acceptance criteria

- [ ] `?address=&latest=true`: **≤1** item, last **`completed`** only; `total` 0 if no **`completed`** exists.
- [ ] `latest=true` without `address` → **400**.
- [ ] `latest=true&chain_id=N`: latest completed scan matching the chain.
- [ ] `completed A + failed B` where B is newer → `latest=true` returns A.
- [ ] Newest row for W7 remains default sort + `limit=1`; never use `latest=true` for W7.
- [ ] OpenAPI: query param **`latest`** documented on `GET /wallets/scans` with `address` requirement.
- [ ] Contract tests cover latest, no completed, newer failed, and missing address.

### Out of scope

- `POST /scan` in-flight guard (**IMM-4c**).
- Route `/wallets/scans/latest`.

### Implementation hints

- `internal/handler/discovery_v1_scans.go` — `ListDiscoveryV1WalletScans`
- `openapi/discovery-v1.yaml` — query parameter `latest`
- `internal/contract/wallet_scans_v1_test.go`

**Suggested branch:** `discovery/scan-history-latest-completed`  
**Suggested PR title:** `Discovery v1: add wallet scan latest=true query`

### Test plan

- [ ] `go test ./internal/handler/... ./internal/contract/...`

### Tracking

| Field | Value |
|-------|--------|
| Issue | — |
| PR | — |

---

## GitHub issue — IMM-4c

### Title (copy as-is)

```
[Discovery][IMM-4c] POST wallet scan: block in-flight rescans with SCAN_IN_PROGRESS
```

### Labels (suggested)

`discovery` · `scan-history` · `api` · `v1`

### Repository

`create2-labs/cafe-discovery`

### Body (copy below the line)

---

**Type:** Technical task (W8 guard).

**Tracking ID:** IMM-4c  
**Plan:** [IMMUTABILITE_PR.md — IMM-4c](https://github.com/create2-labs/cafe-discovery/blob/main/IMMUTABILITE_PR.md#github-issue--imm-4c)  
**Depends on:** IMM-4a

### Summary

Before accepting **`POST /api/discovery/v1/scan`** for a wallet target, reject if a scan is already in progress for the same owner/address.

### Acceptance criteria

- [ ] Newest DB row `RUNNING` / API `started` → **409** `SCAN_IN_PROGRESS`.
- [ ] Pending v1 `requested` before persistence row exists → **409** `SCAN_IN_PROGRESS`.
- [ ] Newest `FAILED` → scan accepted (W1 policy/draft guard comes later in **IMM-9**).
- [ ] W8 helper is reusable by **IMM-9**, which adds W1 after this guard.
- [ ] OpenAPI: `POST /scan` documents **409** `SCAN_IN_PROGRESS`.
- [ ] Tests cover pending requested, DB running, failed retry, and wallet-only scope.

### Out of scope

- W1 policy/draft lookup (**IMM-9**).
- CPM W7/W2 guards (**IMM-10**).
- TLS rescans.

### Implementation hints

- `internal/handler/discovery.go` — `prepareWalletScanQueue`
- `internal/repository/pending_v1_scan_repository.go` — pending lookup by owner/address, or equivalent
- `openapi/discovery-v1.yaml` — `409` on `POST /scan`
- `internal/handler/discovery_scan_v1_test.go`

**Suggested branch:** `discovery/block-in-flight-wallet-scan`  
**Suggested PR title:** `Discovery v1: block wallet scan while latest scan is in progress`

### Test plan

- [ ] `go test ./internal/handler/... ./internal/repository/...`

### Tracking

| Field | Value |
|-------|--------|
| Issue | — |
| PR | — |

---

## GitHub issue — IMM-5

### Title (copy as-is)

```
[Discovery][IMM-5] Clean up Redis wallet read paths after scan history migration
```

### Labels (suggested)

`discovery` · `scan-history` · `redis`

### Repository

`create2-labs/cafe-discovery`

### Body (copy below the line)

---

**Type:** Technical task (consistency).

**Tracking ID:** IMM-5  
**Plan:** [IMMUTABILITE_PR.md — IMM-5](https://github.com/create2-labs/cafe-discovery/blob/main/IMMUTABILITE_PR.md#github-issue--imm-5)  
**Depends on:** IMM-4a

### Summary

After **IMM-4a**, wallet reads no longer rely on a single result per `(user_id, address)`. Clean up remaining Redis/cache behavior so it cannot hide or delete historical wallet rows incorrectly.

### Acceptance criteria

- [ ] Wallet cache/read-through methods made obsolete by **IMM-4a** are removed or unreachable.
- [ ] Redis is not used as source of truth for wallet reads by `(user_id, address)`.
- [ ] `DeleteDiscoveryV1WalletScan`: Redis `DeleteByUserIDAndAddress` only when **no** remaining Postgres rows for that address.
- [ ] No regression for v1 `GET …/wallets/scans/{scan_id}` (Postgres path).

### Out of scope

- Full Redis schema migration to `scan_id` keys.

### Implementation hints

| File | Role |
|------|------|
| `internal/service/user_scan_cache.go` | Remove obsolete wallet read-through |
| `internal/handler/discovery_v1_scans.go` | DELETE + Redis |

**Suggested branch:** `discovery/scan-history-redis-cleanup`  
**Suggested PR title:** `Discovery: clean up Redis wallet read paths after scan history migration`

### Test plan

- [ ] `go test ./internal/service/...`
- [ ] Manual: delete one of two scans same address; Redis key behavior matches spec

### Tracking

| Field | Value |
|-------|--------|
| Issue | — |
| PR | — |

---

## GitHub issue — IMM-6

### Title (copy as-is)

```
[Discovery][IMM-6] Plan quotas: count scan executions after history model
```

### Labels (suggested)

`discovery` · `scan-history` · `billing` · `plans`

### Repository

`create2-labs/cafe-discovery`

### Body (copy below the line)

---

**Type:** Technical task (quota semantics).

**Tracking ID:** IMM-6  
**Plan:** [IMMUTABILITE_PR.md — IMM-6](https://github.com/create2-labs/cafe-discovery/blob/main/IMMUTABILITE_PR.md#github-issue--imm-6)  
**Depends on:** IMM-3

### Summary

After IMM-3, each completed scan is a **row**. Plan limits (`WalletScanLimit`, `EndpointScanLimit`) should count **executions** (rows), not implicit unique addresses/URLs. Confirm `CountByUserID` and pre-scan checks in `POST /api/discovery/v1/scan` match this.

> **Superseded for limit enforcement:** **IMM-6b** (ledger **P1**) replaces row-count / soft-delete semantics for **`GET /plans/usage`** and **`CheckScanLimit`**. See [IMM-6b](#imm-6b--quotas-plan--ledger-dusage-monotonic-option-b).

### Acceptance criteria

- [x] `CountByUserID` on wallet/TLS repos counts all owner rows (respecting soft-delete if applicable). *(IMM-6 merged — **not** the long-term quota source.)*
- [x] Second `POST /api/discovery/v1/scan` for same address increments usage when limit is finite.
- [x] Limit reached → **403** with existing error shape.
- [x] Tests for plan service / handler limit check updated or added.

**Follow-up (IMM-6b):**

- [ ] Ledger on **completed success** only ; POST guards **G1/G2** ; atomic commit **G3** ; CBOM success-only **G4**.
- [ ] `GET /plans/usage` : `used`, `visible`, `deleted_by_user` ; usage never decreases on DELETE.

### Out of scope

- Product pricing table changes.

### Implementation hints

- `internal/service/plan.go`, `internal/handler/discovery.go` (`prepareWalletScanQueue`, `checkScanLimits`)
- `internal/repository/scan_result_repository.go` `CountByUserID`

**Suggested branch:** `discovery/scan-history-plan-quota-semantics`  
**Suggested PR title:** `Discovery: count scan executions for plan quota`

### Test plan

- [ ] `go test ./internal/service/...` (plan tests)

### Tracking

| Field | Value |
|-------|--------|
| Issue | — |
| PR | — |

---

## GitHub issue — IMM-7

### Title (copy as-is)

```
[Discovery][IMM-7] Tests and contract: scan immutability and multi-row history
```

### Labels (suggested)

`discovery` · `scan-history` · `testing` · `contract`

### Repository

`create2-labs/cafe-discovery`

### Body (copy below the line)

---

**Type:** Technical task (quality gate).

**Tracking ID:** IMM-7  
**Plan:** [IMMUTABILITE_PR.md — IMM-7](https://github.com/create2-labs/cafe-discovery/blob/main/IMMUTABILITE_PR.md#github-issue--imm-7)  
**Depends on:** IMM-3, IMM-4a–4c

### Summary

Add automated coverage so **CPM Option A** and smoke scripts can rely on stable `scan_id` after re-scan: list returns N items, detail by old `scan_id` still works, `result` immutable after terminal.

### Acceptance criteria

- [ ] Persistence integration test: two lifecycles, same `address`, different `scan_id` → two terminal rows.
- [ ] `internal/contract/wallet_scans_v1_test.go`: multi-item list + detail stability for first `scan_id`.
- [ ] `docs/CPM_OPTION_A_DISCOVERY_V1_CONTRACT.md` updated if it implied one scan per address.
- [ ] CI green: `go test ./internal/contract/... ./internal/persistence/...`

### Out of scope

- CPM repo changes (unless doc cross-link only).
- Frontend e2e (optional follow-up).

### Implementation hints

- May merge focused coverage into IMM-4a/4b/4c when small; otherwise dedicated branch after IMM-3 + IMM-4a–4c on `main`.

**Suggested branch:** `discovery/scan-history-tests-contract`  
**Suggested PR title:** `Discovery: tests for scan immutability and per-address history`

### Test plan

- [ ] Full package tests above
- [ ] `bash -n` on CPM smoke script unchanged or extended (optional)

### Tracking

| Field | Value |
|-------|--------|
| Issue | — |
| PR | — |

---

## GitHub issue — IMM-8

### Title (copy as-is)

```
[Deploy][IMM-8] Runbook: Discovery scan history migration (IMM-2 + IMM-3)
```

### Labels (suggested)

`documentation` · `deploy` · `discovery` · `runbook`

### Repository

`create2-labs/cafe-deploy`

### Body (copy below the line)

---

**Type:** Technical task (operations / release).

**Tracking ID:** IMM-8  
**Plan:** [cafe-discovery IMMUTABILITE_PR.md — IMM-8](https://github.com/create2-labs/cafe-discovery/blob/main/IMMUTABILITE_PR.md#github-issue--imm-8)  
**Depends on:** IMM-1 approved; documents rollout for IMM-2 + IMM-3

### Summary

Document safe deployment order for scan history schema + persistence writer changes: backup, image order (`cafe-discovery-persistence` vs `cafe-discovery-backend`), rollback notes, and release note for operators (CPM `scan_id` references no longer broken by re-scan).

### Acceptance criteria

- [ ] Runbook in `cafe-deploy` (e.g. `docs/RUNBOOK_SCAN_HISTORY.md` or README section): prerequisites, steps, verification, rollback.
- [ ] Documents **atomic release IMM-2 + IMM-3**: separate PRs OK, **single deploy window** — no staging/prod with IMM-2 schema + old upsert writers; no staging/prod with IMM-3 writers + old unique indexes.
- [ ] Order: DB migration (IMM-2) → persistence image (IMM-3) → backend API (IMM-4a–4c) in the same train.
- [ ] Release note: re-scan creates new `scan_id`; old policies keep valid `scan_id` until DELETE.
- [ ] Optional: link to `cafe-crypto-policy-mgt` smoke script `test-discovery-v1-wallet-scans-to-cpm.sh` for post-deploy check.

### Out of scope

- Implementing IMM-2/3 code (cafe-discovery issues).
- Compose version bumps unless required for doc accuracy.

### Implementation hints

- `cafe-deploy/README.md`, compose service order for discovery + persistence

**Suggested branch:** `deploy/scan-history-migration-runbook`  
**Suggested PR title:** `Deploy: runbook for Discovery scan history migration`


---

## GitHub issue — IMM-9

### Title (copy as-is)

```
[Discovery][IMM-9] Block wallet POST /scan when CPM policy or draft exists for target address (W1)
```

### Labels (suggested)

`discovery` · `scan-history` · `cpm` · `api`

### Repository

`create2-labs/cafe-discovery` (+ coordination `cafe-crypto-policy-mgt`)

### Body (copy below the line)

---

**Type:** Technical task (**WORKPLAN §2.2 W1** + garde en cours, **IMM-4c**).

**Tracking ID:** IMM-9  
**Plan:** [IMMUTABILITE_PR.md — W7 POST, W1](https://github.com/create2-labs/cafe-discovery/blob/main/IMMUTABILITE_PR.md#règles-produit-wallet--cpm-référence-workplan-22-w1w7)

### Summary

Before `POST /api/discovery/v1/scan` with `{ "address": "0x…" }` (wallet only — **W1** does not apply to TLS URL scans):

1. **Scan en cours** (**IMM-4c**) — newest row `requested` / `started` → **409** `SCAN_IN_PROGRESS`.
2. **W1** — internal CPM lookup (**IMM-9b**) by **normalized `target_address`** (no `scan_id` yet; Discovery must not resolve address via policy → Discovery detail) → **409** `CPM_EXISTS_FOR_WALLET_TARGET` when policy **or** draft exists.

Newest **`failed`** → new scan **only** if **W1** OK (no policy, no draft). User may unblock via **`DELETE /api/cpm/v1/drafts?id=…`** or finalize policy.

### Acceptance criteria

- [ ] In-flight guard **before** **W1** (reuse IMM-4c helper).
- [ ] **W1**: **409** when policy **or** draft exists (**IMM-9b** lookup by normalized address).
- [ ] Fail-closed if CPM lookup unavailable (no silent bypass).
- [ ] Newest **`failed`** + draft present → **409** (must delete/finalize draft first).
- [ ] Newest **`failed`** + no draft/policy → scan **accepted**.
- [ ] OpenAPI documents `SCAN_IN_PROGRESS` + `CPM_EXISTS_FOR_WALLET_TARGET`.
- [ ] Integration tests: in-flight, draft blocks, policy blocks, failed retry when W1 OK.

### Dependencies

**IMM-3**, **IMM-4c** (in-flight guard on POST).

### Out of scope

CPM explore/persist guards (**IMM-10**).

**Suggested branch:** `discovery/block-scan-when-cpm-exists`

### Tracking

| Field | Value |
|-------|--------|
| Issue | — |
| PR | — |

---

## GitHub issue — IMM-10

### Title (copy as-is)

```
[CPM][IMM-10] Policy explore/persist: W7 newest row then W2 latest completed (wallet-only)
```

### Labels (suggested)

`cpm` · `scan-history` · `contract-alignment`

### Repository

`create2-labs/cafe-crypto-policy-mgt`

### Body (copy below the line)

---

**Type:** Technical task (**WORKPLAN §2.2 W7**, **W2**). *Primary repo is CPM; see companion Discovery doc.*

**Tracking ID:** IMM-10  
**Plan:** [cafe-crypto-policy-mgt/workplans/IMMUTABILITE_PR.md](https://github.com/create2-labs/cafe-crypto-policy-mgt/blob/main/workplans/IMMUTABILITE_PR.md)

### Summary

`POST /api/cpm/v1/policies/decisions/explore` and `POST /api/cpm/v1/policies` for **wallet** targets (**Option A**, wallet-only — no TLS assessment/remediation):

**Step 1 — W7 (newest row, not `latest=true`):**

- `GET /api/discovery/v1/wallets/scans?address=0x…&limit=1` (default sort `created_at` desc, `scan_id` desc).
- If `newest.status != completed` → **400** `LATEST_SCAN_NOT_COMPLETED` (includes `failed`, `requested`, `started`).

**Step 2 — W2 (`latest=true`, not `limit=1`):**

- `GET /api/discovery/v1/wallets/scans?address=0x…&latest=true`.
- If requested `scan_id` ≠ latest **`completed`** → **400** `SCAN_ID_NOT_LATEST_FOR_TARGET`.

### Acceptance criteria

- [ ] **W7** before **W2** on every explore/persist.
- [ ] **W7** uses `limit=1` + default sort — **never** `latest=true`.
- [ ] **W2** uses **`latest=true`** — **never** `limit=1` alone.
- [ ] **400** when newest is **`failed`** or in-flight; **`completed` A + `failed` B** → **400** even for `scan_id` A.
- [ ] **400** `SCAN_ID_NOT_LATEST_FOR_TARGET` when `scan_id` is historical.
- [ ] Wallet-only; no TLS CPM path in this PR.
- [ ] Contract tests: in-flight, failed, `completed`+`failed` newer, historical `scan_id`.

### Dependencies

Discovery **IMM-4b** (`latest=true`) and **IMM-4a** (multi-row history).

### Tracking

| Field | Value |
|-------|--------|
| Issue | — |
| PR | — |

---

## GitHub issue — IMM-12

### Title (copy as-is)

```
[Discovery][IMM-12] GET /api/discovery/v1/wallets/scans/{scan_id}/cbom on demand
```

### Labels (suggested)

`discovery` · `scan-history` · `cbom` · `api`

### Repository

`create2-labs/cafe-discovery`

### Body (copy below the line)

---

**Type:** Technical task (**WORKPLAN §2.2 W6**).

**Tracking ID:** IMM-12  
**Plan:** [WORKPLAN_API.md §2.2 W6](https://github.com/create2-labs/cafe-crypto-policy-mgt/blob/main/workplans/WORKPLAN_API.md) · [IMMUTABILITE_PR.md](https://github.com/create2-labs/cafe-discovery/blob/main/IMMUTABILITE_PR.md)

### Summary

Expose CycloneDX CBOM per wallet scan via v1 API. CBOM is **generated on read** from persisted scan fields (`ToCBOM()`), not stored. Restores user-facing CBOM per historical `scan_id` after PR13c removed CBOM historique.

### Acceptance criteria

- [ ] `GET /api/discovery/v1/wallets/scans/{scan_id}/cbom` → **200** JSON CBOM when scan exists and terminal.
- [ ] **404** if scan missing or not owner.
- [ ] **409** or **400** if scan not terminal (document choice).
- [ ] OpenAPI path + schema; README link.
- [ ] Optional: TLS sibling route.

### Dependencies

IMM-3 (row stable per `scan_id`).

**Suggested branch:** `discovery/v1-cbom-by-scan-id`

### Tracking

| Field | Value |
|-------|--------|
| Issue | — |
| PR | — |

---

## GitHub issue — IMM-11

### Title (copy as-is)

```
[Discovery][IMM-11] Remove obsolete routes, OpenAPI entries, edge mappings and scripts
```

### Labels (suggested)

`discovery` · `contract-alignment` · `cleanup` · `deploy`

### Repository

`create2-labs/cafe-discovery` (+ `cafe-deploy` edge/nginx ; scripts/tests cross-repo)

### Body (copy below the line)

---

**Type:** Technical task (cleanup — **WORKPLAN §5.3**, **§8.7**).

**Tracking ID:** IMM-11  
**Plan:** [IMMUTABILITE_PR.md — IMM-11](https://github.com/create2-labs/cafe-discovery/blob/main/IMMUTABILITE_PR.md#imm-11--remove-obsolete-routes-openapi-edge-mappings-and-scripts)

### Summary

After target routes are available (**IMM-4a–4c**, **IMM-12**, CPM **§0.2**), remove legacy Discovery surfaces and align edge/OpenAPI/scripts. **Before** considering the IMM chain complete.

### Scope

- Remove or confirm removal of legacy **la liste wallet historique** (list by address).
- Remove or confirm removal of legacy **la liste TLS historique** (list by URL).
- Remove ambiguous detail routes by address or URL in path.
- Remove **CBOM historique** if still referenced.
- Explicit decision on **la façade policy-context historique** (remove if redundant with `GET /api/discovery/v1/wallets/scans` + `GET /api/cpm/v1/policies?scan_id=`).
- Clean OpenAPI, README, edge/nginx, smoke scripts, frontend/backend tests.

### Acceptance criteria

- [ ] Legacy routes not served on edge or backend.
- [ ] OpenAPI documents only **§0** canonical paths.
- [ ] QA/tests updated — no references to removed paths.
- [ ] Deploy/nginx config matches **WORKPLAN §5.5**.

### Dependencies

**IMM-4a–4c**, **IMM-12** ; CPM routes stable ; **IMM-9** / **IMM-10** error codes documented.

### Out of scope

New API features.

**Suggested branch:** `discovery/remove-obsolete-routes`  
**Suggested PR title:** `Discovery IMM-11: remove obsolete routes, edge mappings and scripts`

### Tracking

| Field | Value |
|-------|--------|
| Issue | — |
| PR | — |

---

## GitHub issue — IMM-6b (ledger monotonic)

### Title (copy as-is)

```
[Discovery][IMM-6b] Plan usage ledger: monotonic quota (option B)
```

### Labels (suggested)

`discovery` · `scan-history` · `billing` · `plans`

### Repository

`create2-labs/cafe-discovery`

### Summary

Replace row-count quota (**IMM-6**) with success-only ledger (**IMM-6b**). **P1:** usage counts **completed successes** only; never decreases on DELETE. **G1–G4:** POST guards (`successful + in_flight < limit`, cap `min(limit,3)`), atomic commit on completion, stripped failure if over limit (keep address), CBOM success-only.

**PR sequence:** IMM-6b-1 (schema) → IMM-6b-2 (repo) → IMM-6b-3 (POST guards) → IMM-6b-4 (persistence commit) → IMM-6b-5 (usage API) → IMM-6b-6 (backfill) → IMM-6b-7 (CBOM) → IMM-6b-8 (tests).

See [IMM-6b section](#imm-6b--quotas-plan--ledger-success-only-garde-post-commit-atomique-option-b).

### Acceptance criteria

- [ ] Ledger event **only** on persisted **completed success** (one per `scan_id`).
- [ ] POST blocked when `successful + in_flight >= limit`.
- [ ] POST blocked when `in_flight >= min(limit, 3)`.
- [ ] Concurrent completions at limit: one rich success, one `PLAN_LIMIT_EXCEEDED` stub (address kept, no result).
- [ ] `GET /plans/usage` exposes `used`, `visible`, `deleted_by_user`.
- [ ] DELETE success scan: `used` unchanged, `visible` decreases.
- [ ] CBOM **404** unless **completed success**.

---

## Références

| Document | Rôle |
|----------|------|
| [`WORKPLAN_API.md`](../cafe-crypto-policy-mgt/workplans/WORKPLAN_API.md) §2.2, §4.2, §5.4.6, §8.7 | Contrat cible |
| [`WORKPLAN_API_PR.md`](../cafe-crypto-policy-mgt/workplans/WORKPLAN_API_PR.md) | PR4 list/detail, PR6 DELETE — prérequis livrés |
| [`docs/CPM_OPTION_A_DISCOVERY_V1_CONTRACT.md`](./docs/CPM_OPTION_A_DISCOVERY_V1_CONTRACT.md) | Mapping CPM ↔ v1 |
| [`openapi/discovery-v1.yaml`](./openapi/discovery-v1.yaml) | Spec immutabilité `result` |
| [`internal/persistence/storage/postgres.go`](./internal/persistence/storage/postgres.go) | Writers à modifier (**IMM-3**) |
