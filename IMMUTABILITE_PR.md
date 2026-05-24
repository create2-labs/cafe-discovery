# Scan immutability & history — PR plan (Discovery)

**Propriétaire runtime :** `cafe-discovery` (API, persistence-service, Postgres, Redis cache scan).

**Source de vérité (contrat produit) :** [`cafe-crypto-policy-mgt/workplans/WORKPLAN_API.md`](https://github.com/create2-labs/cafe-crypto-policy-mgt/blob/main/workplans/WORKPLAN_API.md) — **§2.2** (invariants, couplage **W1–W8**), **§4.2.1**, **§8.4–§8.6**. **Ce fichier** = découpage PR / écarts **implémentation** vs ce workplan.

**Contrat machine-readable :** `openapi/discovery-v1.yaml` — à aligner sur **W1**, **W6**, persistance multi-lignes.

**Contexte CPM :** policies par **`scan_id`** (**PR5/PR6/PR7**). **PR6** (**DELETE** + **409**) reste le modèle cible (**§2.2 W3**) : l’utilisateur supprime d’abord la CPM, puis le scan.

**Règles d’exécution (propriétaire humain) :** l’agent / les contributeurs ne font **pas** de commit, push, merge ni tags ; revue, git et publication restent manuelles. Chaque PR : branche locale, changements ciblés, tests, puis proposition de titre/message de commit et de PR (sections **Proposed** en anglais).

**Statut du document :** plan de découpe — **IMM-1** documenté sur branche `docs/scan-immutability-gap-and-migration` ([`docs/SCAN_IMMUTABILITY_MIGRATION.md`](docs/SCAN_IMMUTABILITY_MIGRATION.md)) ; **IMM-2+** non mergés.

---

## Règles produit wallet ↔ CPM (référence [WORKPLAN](https://github.com/create2-labs/cafe-crypto-policy-mgt/blob/main/workplans/WORKPLAN_API.md) §2.2 W1–W8)

**Périmètre :** scans **wallet / EVM** et CPM **`binding=discovery`**. **TLS** : historique + CBOM par **`scan_id`** ; pas de **W1–W4** / **W7–W8** (assessment wallet-only).

| WORKPLAN | Règle | Discovery | CPM | Frontend |
|----------|--------|-----------|-----|----------|
| **W1** | Pas de **nouveau scan** si policy **ou** draft sur la cible (après **W8**) | `POST …/scan` → **409** `CPM_EXISTS_FOR_WALLET_TARGET` | Lookup policies **+** drafts | Désactiver « Scanner » si policy/draft |
| **W2** | CPM **uniquement** sur le dernier scan **`completed`** | **`GET …?address=&latest=true`** (≤1 item **`completed`**) | **400** si `scan_id` ≠ latest **`completed`** | Référence policy via **`latest=true`** |
| **W3** | **DELETE scan** : l’utilisateur **supprime d’abord la CPM**, puis **`DELETE …/wallets/scans/{scan_id}`** | **409** `SCAN_REFERENCED_BY_POLICY` tant que policy liée ; **204** après | `GET ?scan_id=` puis `DELETE ?id=` | Guide 409 → supprimer policy → retry DELETE |
| **W4** | **DELETE CPM** sans effet sur les scans | Inchangé | `DELETE …/policies?id=` | Bouton supprimer policy |
| **W5** | **Historique** par adresse | `GET …/wallets/scans?address=` | Lecture | Liste chronologique |
| **W6** | **CBOM** par scan | `GET …/wallets/scans/{scan_id}/cbom` à la demande | Hors scope | Lien CBOM par ligne |
| **W7** | **CPM uniquement** : pas d’explore/persist tant que la **newest row** (`created_at` max) n’est pas **`completed`** (`failed` ou en cours inclus) | — | explore/persist → **400** `LATEST_SCAN_NOT_COMPLETED` | **CPM** off si newest ≠ **`completed`** |
| **W8** | **Rescan** (`POST …/scan`) : **indépendant de W7** — refus seulement si scan **en cours** ; si newest = **`failed`**, nouveau scan **OK** (sous **W1**) | **409** `SCAN_IN_PROGRESS` si en cours ; **accepté** si **`failed`** (+ **W1**) | — | **Scan** off si en cours ; **on** si **`failed`** + **W1** OK (CPM peut rester bloquée) |

> **W7 vs W8 (WORKPLAN §2.2) :** **W7** = blocage **CPM** si newest ≠ **`completed`**. **W8** = garde **rescan** sur **`POST …/scan`** (souvent **IMM-4**) — **409** seulement si en cours. Ex. : **`completed` A** + **`failed` B** → CPM **400** (**W7**), rescan **OK** (**W8** + **W1**).

---

## Écarts implémentation vs [`WORKPLAN_API.md`](https://github.com/create2-labs/cafe-crypto-policy-mgt/blob/main/workplans/WORKPLAN_API.md) (ce plan les comble)

| Sujet | `main` aujourd’hui | Workplan (cible) | PR |
|-------|-------------------|------------------|-----|
| Persistance | 1 ligne / `(user_id, address)` upsert | Multi-lignes, `scan_id` stable | IMM-2, IMM-3 |
| **W1** — contexte CPM actif | Pas de garde CPM / draft | **409** si policy ou draft (**IMM-9** + **IMM-9b**) | IMM-9 |
| **W2** — dernier `completed` | Pas de query **`latest=true`** | **`GET …/wallets/scans?address=&latest=true`** | IMM-4 |
| **W3** — DELETE scan | **409** si policy (**PR6**) — déjà proche | Parcours utilisateur documenté | — (doc/UX) |
| **W4** — DELETE policy | Ne supprime pas scan | **W4** inchangé | — |
| **W5** — historique | Filtre `chain_id` → 1 ligne | Liste multi-lignes par adresse | IMM-4 |
| **W6** — CBOM | Pas de route v1 | **`GET …/wallets/scans/{scan_id}/cbom`** | IMM-12 |
| **W7** — garde CPM lifecycle | Pas de blocage | Explore/persist si newest = **`completed`** | IMM-10 (CPM) |
| **W8** — rescan | Pas de garde en cours | **409** si en cours (**IMM-4**) ; **indépendant** de **W7** | IMM-4, IMM-9 |
| CPM persist | Tout `scan_id` | **W7** + **W2** | IMM-10 (CPM) |

**Implémentation actuelle (`main`) :** non conforme sur **W1, W2, W5, W6, W7, W8** et persistance ; **W3/W4** proches du workplan si **PR6** est actif.

---

## Executive summary

| Domaine | État actuel (`main`) | Cible [`WORKPLAN_API.md`](https://github.com/create2-labs/cafe-crypto-policy-mgt/blob/main/workplans/WORKPLAN_API.md) |
|--------|----------------------|-------------------------|
| **Identité scan** | Chaque `POST /discovery/v1/scan` alloue un **nouveau** `scan_id` (UUID). | Identique. |
| **Persistance wallet** | **Une ligne** par `(user_id, address)` — index unique `idx_scan_results_user_address` ; `WalletWriter` **upsert** et **remplace** `id` + résultat. | **Une ligne par exécution** ; **`scan_id` stable** pour la vie de la ligne ; re-scan = **nouvelle ligne**. |
| **Persistance TLS** | **Une ligne** par `(user_id, url)` — `idx_tls_scan_results_user_url` ; même pattern upsert. | Même règle que wallet (famille TLS). |
| **`result` terminal** | Écrasé sur re-scan (même adresse / même URL). | **Immutable** après terminal pour **ce** `scan_id`. |
| **Liste v1** | `GET …/wallets/scans` sans filtre : `ListOwnerWalletScansDiscoveryV1` (OK multi-lignes **si** DB le permet). Filtre `?address=&chain_id=` : **au plus 1** item via `FindByUserIDAndAddress`. | `?address=` → **toutes** exécutions pour l’adresse ; `?address=&chain_id=` → sous-ensemble **mono-chaîne**, **pas** limité à une seule ligne arbitraire. |
| **POST scan wallet** | Accepté si quota OK. | **W8** : **409** si en cours ; **OK** si newest **`failed`**. Puis **W1**. Indépendant de **W7**. |
| **CPM explore/persist** | Sans garde lifecycle. | **W7** (newest = **`completed`**) + **W2** → **400** si mauvais état ou `scan_id`. |
| **DELETE wallet** | **PR6** : `DELETE …/{scan_id}` + **409** si policy. | **W3** inchangé : action user — d’abord `DELETE` policy, puis scan. |
| **CPM persist** | Tout `scan_id` owner valide. | **W7** + **`scan_id` = latest `completed`** (**W2**). |
| **DELETE CPM** | Ne supprime pas le scan (déjà). | Inchangé (**règle 4**). |
| **Quotas plan** | `CountByUserID` ≈ adresses uniques. | Compter exécutions ; re-scan rare (bloqué si CPM). |
| **Redis** | Une clé par adresse. | Latest + historique Postgres ; **IMM-5**. |
| **CBOM** | Route `/discovery/cbom/*` retirée. | **`GET …/wallets/scans/{scan_id}/cbom`** à la demande (**règle 6**). |

**Décision d’architecture :** l’**historique** est porté par **Postgres** (`scan_results`, `tls_scan_results`) avec **`id` = `scan_id`** ; Redis reste un **accélérateur** optionnel, pas la source de vérité v1.

**`latest=true` (W2) :** dernier scan **`completed`** pour l’adresse — peut coexister avec un **`failed`** plus récent dans l’historique (`total: 1` possible). **`total: 0`** seulement s’il n’existe **aucun** **`completed`**.

**W7 (CPM) :** explore/persist refusés si newest row ≠ **`completed`** → **400** `LATEST_SCAN_NOT_COMPLETED`.

**W8 (rescan) :** **`POST …/scan`** refusé seulement si scan **en cours** → **409** `SCAN_IN_PROGRESS` ; si newest = **`failed`**, rescan **autorisé** (sous **W1**) même si **W7** bloque encore la CPM.

---

## Table de suivi des PR

| PR | GitHub issue (draft) | Branche (proposée) | Dépôt (issue) | PR Git | Dépend de | Objectif en une ligne |
|----|----------------------|-------------------|---------------|--------|-----------|------------------------|
| **IMM-1** | [§ IMM-1](#github-issue--imm-1) | `docs/scan-immutability-gap-and-migration` | `cafe-discovery` | — | — | Doc : écart vs WORKPLAN, stratégie migration. |
| **IMM-2** | [§ IMM-2](#github-issue--imm-2) | `discovery/scan-history-db-migration` | `cafe-discovery` | — | **IMM-1** | DB : retirer unicité par cible. |
| **IMM-3** | [§ IMM-3](#github-issue--imm-3) | `discovery/scan-history-persistence-writers` | `cafe-discovery` | — | **IMM-2** | Writers : une ligne par `scan_id`. |
| **IMM-4** | [§ IMM-4](#github-issue--imm-4) | `discovery/scan-history-api-list-filters` | `cafe-discovery` | — | **IMM-3** | Liste + **`latest=true`** + garde **W8** (`SCAN_IN_PROGRESS`). |
| **IMM-5** | [§ IMM-5](#github-issue--imm-5) | `discovery/scan-history-redis-legacy-readpaths` | `cafe-discovery` | — | **IMM-3** | Redis + chemins legacy alignés. |
| **IMM-6** | [§ IMM-6](#github-issue--imm-6) | `discovery/scan-history-plan-quota-semantics` | `cafe-discovery` | — | **IMM-3** | Quotas = exécutions scan. |
| **IMM-7** | [§ IMM-7](#github-issue--imm-7) | `discovery/scan-history-tests-contract` | `cafe-discovery` | — | **IMM-3**, **IMM-4** | Tests + contract v1. |
| **IMM-8** | [§ IMM-8](#github-issue--imm-8) | `deploy/scan-history-migration-runbook` | `cafe-deploy` | — | **IMM-2** | Runbook déploiement. |
| **IMM-9** | [§ IMM-9](#github-issue--imm-9) | `discovery/block-scan-when-cpm-exists` | `cafe-discovery` (+ CPM interne) | — | **IMM-4**, **IMM-9b** | **W8** (**IMM-4**) + **W1** (policy/draft). |
| **IMM-10** | [§ IMM-10](#github-issue--imm-10) | `cpm/latest-scan-only-policy` | `cafe-crypto-policy-mgt` | — | **IMM-3**, **IMM-4** | **W7** + **W2** : explore/persist latest **`completed`** only. |
| **IMM-12** | [§ IMM-12](#github-issue--imm-12) | `discovery/v1-cbom-by-scan-id` | `cafe-discovery` | — | **IMM-3** | **W6** : `GET …/cbom` à la demande. |

**Colonnes PR Git / Issue :** **—** = pas encore créée. Ouvrir l’issue sur le dépôt indiqué (**IMM-8** → `cafe-deploy`, les autres → `cafe-discovery`).

---

## Création des issues GitHub

Travail d’**alignement contrat / persistance** (écart [`WORKPLAN_API.md`](https://github.com/create2-labs/cafe-crypto-policy-mgt/blob/main/workplans/WORKPLAN_API.md) §2.2), pas un rapport de bug utilisateur.

1. Copier le **title** et le **body** de la section **IMM-*n*** ci-dessous.
2. Créer l’issue sur le dépôt **Dépôt (issue)** du tableau (sauf **IMM-8** → `create2-labs/cafe-deploy`).
3. Labels suggérés (adapter au repo) : voir chaque section.
4. Lier l’issue à la PR : `Closes #NNN` dans la PR Git.
5. Coller l’URL de l’issue dans le tableau ci-dessus et dans le bloc **Tracking** en bas de chaque issue.

**Ordre suggéré :** IMM-1 → IMM-2 → IMM-3 → IMM-4 → IMM-12 → IMM-9 (+ CPM **IMM-9b**) → IMM-10 (CPM) → IMM-7 ; IMM-8 en parallèle de IMM-2.

**Prérequis livrés :** list/detail v1 (**PR4**), **PR5/PR6/PR7** (référence policy, **DELETE** + **409**, policies). **PR6** = comportement cible **W3** — **à conserver**.

**Documents liés :** [`cafe-crypto-policy-mgt/workplans/IMMUTABILITE_PR.md`](https://github.com/create2-labs/cafe-crypto-policy-mgt/blob/main/workplans/IMMUTABILITE_PR.md) (CPM), [`cafe-frontend/IMMUTABILITE.md`](https://github.com/create2-labs/cafe-frontend/IMMUTABILITE.md) (UX).

---

## Invariants cibles (Discovery + règles wallet/CPM)

1. **`scan_id`** alloué à l’acceptation (`requested`).
2. **Une exécution** = **une ligne** Postgres (`id` = `scan_id`).
3. **`result`** **gelé** après état terminal ; **CBOM** dérivé à la demande (**IMM-12**), jamais stocké en blob.
4. **Historique** : plusieurs lignes par `target_address` ; listage **`GET …/wallets/scans?address=`** (**W5**).
5. **W1** : pas de nouveau scan si policy ou draft sur la cible (après **W8**).
6. **W2** : CPM explore/persist sur le dernier scan **`completed`** (`latest=true`).
7. **W3** : effacement scan — d’abord supprimer la CPM, puis **`DELETE …/wallets/scans/{scan_id}`**.
8. **W4** : effacement CPM — **`DELETE …/policies?id=`** sans toucher aux scans.
9. **W6** : **`GET …/wallets/scans/{scan_id}/cbom`** à la demande.
10. **W7** : CPM bloquée si newest ≠ **`completed`**.
11. **W8** : rescan — **`POST …/scan`** bloqué seulement si en cours ; retry si **`failed`** (+ **W1**).

---

## État actuel (fichiers clés)

| Fichier | Comportement aujourd’hui |
|---------|-------------------------|
| `cmd/persistence/main.go` | Crée `idx_scan_results_user_address`, `idx_tls_scan_results_user_url`. |
| `internal/persistence/storage/postgres.go` | `OnConflict` sur `(user_id, address)` / `(user_id, url)` ; commentaire *same address overwrites*. |
| `internal/handler/discovery_v1_scans.go` | Branche `address` + `chain_id` → `FindByUserIDAndAddress` (1 item max). |
| `internal/service/discovery.go` | `getExistingScan` : retourne scan existant → **pas** de nouveau scan synchrone legacy. |
| `internal/repository/base_repository.go` | `findByUserIDAndField` : `ORDER BY created_at DESC` + `First` (dernier gagnant). |

---

## IMM-1 — Gap analysis & stratégie migration (doc)

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

- **Branch:** `discovery/scan-history-db-migration`
- **Repository:** `cafe-discovery`
- **Objective:** Autoriser **plusieurs lignes** par `(user_id, address)` et `(user_id, url)`.
- **Scope:**
  - Migration SQL (GORM ou script) : `DROP INDEX IF EXISTS idx_scan_results_user_address` ; `DROP INDEX IF EXISTS idx_tls_scan_results_user_url`.
  - Index non unique pour perf : ex. `(user_id, address, created_at DESC)` / `(user_id, url, created_at DESC)` (noms à figer en implémentation).
  - Mettre à jour `cmd/persistence/main.go` : ne plus recréer les index **uniques** ; créer les index de liste.
  - Note : `id` (UUID) reste **PK** — pas de changement.
- **Out of scope:** Logique `OnStarted` / `OnCompleted`.
- **Dependencies:** **IMM-1**
- **Implementation notes:**
  - Environnements avec **une** ligne par adresse : aucune action données requise avant IMM-3.
  - PostgreSQL **15+** : conserver `NULLS NOT DISTINCT` sur TLS `user_id` si applicable.
- **Tests:** Test migration idempotente (CI ou test intégration DB).
- **Validation commands:** `cd cafe-discovery && go test ./cmd/persistence/... ./internal/persistence/...` (selon arborescence tests).
- **Proposed commit title:** `db: allow multiple scan rows per user target`
- **Proposed PR title:** `Discovery: DB migration for per-execution scan history`
- **Risks:** Déploiement persistence **avant** IMM-3 si writers encore en upsert → doublons ou erreurs jusqu’à IMM-3 ; coordonner **IMM-8**.
- **Completion criteria:** Index unique absents ; index liste présents ; migration rejouable.

---

## IMM-3 — Persistence writers (insert par scan_id)

- **Branch:** `discovery/scan-history-persistence-writers`
- **Repository:** `cafe-discovery`
- **Objective:** Aligner `WalletWriter` / `TLSWriter` sur **une ligne par `scan_id`**.
- **Scope:**
  - **`OnStarted`:** `INSERT` ligne `id = scan_id`, status `RUNNING` ; **ne plus** no-op si une autre ligne terminal existe pour la même cible.
  - **`OnCompleted` / `OnFailed`:** `UPDATE … WHERE id = ?` ; si ligne absente, `INSERT` (replay event) — **sans** `ON CONFLICT (user_id, address)`.
  - Transitions invalides : conserver garde `ValidTransition` par **`scan_id`** (logs + ignore duplicate completion).
  - `internal/persistence/handlers/scan_events.go` : ajuster messages / tests.
- **Out of scope:** Handlers HTTP v1 (IMM-4).
- **Dependencies:** **IMM-2** (sinon INSERT 2ᵉ ligne même adresse → violation unique)
- **Tests:** Unit tests `postgres.go` : deux `scan_id` différents, même `address` → deux lignes ; terminal A inchangé quand B complète.
- **Proposed commit title:** `persistence: one scan row per scan_id (no target upsert)`
- **Proposed PR title:** `Discovery persistence: per-execution scan rows`
- **Risks:** Double `scan.started` même `scan_id` → idempotence PK ; événements hors ordre documentés.
- **Completion criteria:** Re-scan NATS même adresse laisse l’ancienne ligne intacte (status + `result`).

---

## IMM-4 — API v1 list filters + `latest=true`

- **Branch:** `discovery/scan-history-api-list-filters`
- **Repository:** `cafe-discovery`
- **Objective:** Listes conformes **§4.2.1** ; query **`latest=true`** pour **W2** (sans route `/latest` dédiée).
- **Scope:**
  - `ListDiscoveryV1WalletScans` : branche `normalizedAddr != "" && chainID != nil` → `ListOwnerWalletScansDiscoveryV1` + filtre `walletEntityMatchesChainID`, **pas** `FindByUserIDAndAddress` seul.
  - Query **`latest=true`** : exige **`address`** ; retourne ≤1 **`ScanListItem`** en statut **`completed`** uniquement (**W2**).
  - Garde **`POST …/scan`** : si le plus récent est **`requested`** / **`started`** → **409** `SCAN_IN_PROGRESS` (pas de blocage si dernier **`failed`**).
  - Optionnel : **`latest=true&chain_id=N`** — latest parmi les scans matchant la chaîne.
  - OpenAPI `discovery-v1.yaml` : paramètre **`latest`** (boolean) + code **`SCAN_IN_PROGRESS`**.
  - Tests contract : `latest=true` → dernier **`completed`** ; dernier **`failed`** seul → `total: 0` ; **A `completed` + B `failed`** → `latest=true` = A, POST scan **OK**, (CPM testé en IMM-10).
- **Out of scope:** Route **`/wallets/scans/latest`** (délibérément **query** `latest=true` only).
- **Dependencies:** **IMM-3**
- **Proposed commit title:** `api: wallet scan list filters and latest=true query`
- **Proposed PR title:** `Discovery v1: fix wallet scan list when multiple rows per address`
- **Completion criteria:** listes + `latest=true` conformes ; **`POST …/scan`** refusé seulement si scan **en cours** (`SCAN_IN_PROGRESS`).

---

## IMM-5 — Redis & chemins legacy lecture

- **Branch:** `discovery/scan-history-redis-legacy-readpaths`
- **Repository:** `cafe-discovery`
- **Objective:** Ne pas contredire l’historique Postgres sur les chemins encore actifs.
- **Scope:**
  - `DiscoveryService.getExistingScan` / `ScanWallet` : **ne pas** court-circuiter un nouveau scan API legacy si produit exige historique (ou documenter dépréciation explicite du chemin synchrone).
  - `UserScanCacheService` : au warm / read-through, si plusieurs entités même adresse → Redis garde **le plus récent** (`created_at`) **ou** skip write-through v1 (décision IMM-1).
  - `DeleteDiscoveryV1WalletScan` : `redisWalletRepo.DeleteByUserIDAndAddress` — ne supprimer que si plus aucune ligne Postgres pour cette adresse (**IMM-3**).
- **Out of scope:** Refonte complète Redis par `scan_id` (option future).
- **Dependencies:** **IMM-3**
- **Proposed commit title:** `fix: align redis and legacy scan paths with per-scan_id history`
- **Completion criteria:** Pas de suppression Redis qui masque un scan historique encore en Postgres.

---

## IMM-6 — Quotas plan (sémantique exécutions)

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

---

## IMM-7 — Tests & contract v1

- **Branch:** `discovery/scan-history-tests-contract`
- **Repository:** `cafe-discovery`
- **Objective:** Verrouiller le comportement pour CPM Option A et scripts smoke v1.
- **Scope:**
  - Test intégration persistence : enchaînement `started` → `completed` × 2 `scan_id`, même address.
  - Contract `wallet_scans_v1` : liste `total >= 2`, détail ancien `scan_id` toujours **200** + `result` inchangé.
  - Mettre à jour `docs/CPM_OPTION_A_DISCOVERY_V1_CONTRACT.md` si une phrase impliquait implicitement 1 scan / adresse.
- **Dependencies:** **IMM-3**, **IMM-4**
- **Proposed commit title:** `test: scan immutability and multi-row history per address`
- **Completion criteria:** `go test ./internal/contract/... ./internal/persistence/...` vert en CI.

---

## IMM-8 — Deploy runbook (cafe-deploy)

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

- **Dépôts :** `cafe-discovery` ; appel CPM (interne ou API owner) pour **W1**.
- **Ordre :** garde **en cours** (**IMM-4**, `SCAN_IN_PROGRESS` si `requested` / `started`) ; puis **W1** — CPM existante → **409** `CPM_EXISTS_FOR_WALLET_TARGET`. **Pas** de blocage si dernier scan = **`failed`** (retry autorisé).
- **Dépend de :** IMM-3, **IMM-4**.

---

## IMM-10 — CPM explore/persist (**W7** + **W2**)

- **Dépôt principal :** `cafe-crypto-policy-mgt` — [`workplans/IMMUTABILITE_PR.md`](../cafe-crypto-policy-mgt/workplans/IMMUTABILITE_PR.md).
- **W7 (CPM)** : refuser explore/persist si le plus récent (`created_at`) n’est pas **`completed`** → **400** `LATEST_SCAN_NOT_COMPLETED` (y compris si dernier = **`failed`**).
- **W2** : refuser si `scan_id` ≠ celui de **`GET …?address=&latest=true`**.

---

## IMM-12 — CBOM par scan_id (W6)

- **API :** `GET /discovery/v1/wallets/scans/{scan_id}/cbom` (`ToCBOM()` à la demande).
- **Dépend de :** IMM-3.

---

## Séquence recommandée

```text
IMM-1 (doc)
   ↓
IMM-2 (DB) ──→ IMM-8 (runbook)
   ↓
IMM-3 (writers) ──→ IMM-5, IMM-6
   ↓
IMM-4 (list) ──→ IMM-12 (CBOM)
   ↓
IMM-9 (block POST scan si CPM)     IMM-10 (CPM latest-only, repo CPM)
   ↓
IMM-7 (tests E2E W1–W8)
```

**Merge train minimal :** IMM-1 → IMM-2 → IMM-3 → IMM-4 → IMM-12 → IMM-9 → IMM-10 → IMM-7. **PR6** (DELETE + 409) : pas de PR IMM dédiée.

---

## Critères d’acceptation produit (fin de chaîne)

**Historique & immutabilité (5–6)**

- [ ] Sans CPM sur la cible : deux scans successifs → deux `scan_id` dans `GET …/wallets/scans?address=…`.
- [ ] `GET …/wallets/scans/{scan_id}` : `result` stable après terminal ; `GET …/wallets/scans/{scan_id}/cbom` renvoie le CBOM de **ce** scan uniquement.

**CPM & scan (W7, W8, W1, W2)**

- [ ] Dernier scan `started` : `POST …/scan` → **409** `SCAN_IN_PROGRESS` ; CPM explore → **400**.
- [ ] Dernier scan `failed` : `POST …/scan` → **OK** (nouvelle ligne) ; CPM explore → **400**.
- [ ] **A `completed` + B `failed`** (B plus récent) : `latest=true` → A ; `POST …/scan` **OK** ; CPM → **400**.
- [ ] Dernier scan `completed` : `latest=true` renvoie ce `scan_id` ; CPM explore/persist OK (**W2**).
- [ ] Avec CPM persistée : `POST …/scan` → **409** (**W1**).
- [ ] CPM sur `scan_id` historique → **400** (**W2**).

**Suppression (W3–W4)**

- [ ] `DELETE …/policies?id=` : scan(s) toujours listables.
- [ ] `DELETE …/wallets/scans/{scan_id}` avec policy encore liée → **409** `SCAN_REFERENCED_BY_POLICY`.
- [ ] Après `DELETE` policy : `DELETE` scan → **204** ; scan retiré de la liste.

**Doc**

- [ ] `WORKPLAN_API.md` §2.2 W1–W8 et §4.2.1 à jour (source de vérité).

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
| Issue | — |
| PR | — |

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
**Depends on:** IMM-1 (migration strategy approved)

### Summary

Remove unique indexes that enforce **one scan row per `(user_id, address)`** and **per `(user_id, url)`**, so IMM-3 can insert a new row for each `scan_id` without conflicting. Add non-unique indexes to keep list/filter queries efficient.

### Acceptance criteria

- [ ] `DROP` (or equivalent) `idx_scan_results_user_address` and `idx_tls_scan_results_user_url`.
- [ ] Non-unique indexes for listing by user + address/url (names defined in PR).
- [ ] `cmd/persistence/main.go` no longer recreates **unique** indexes on startup; creates list indexes instead.
- [ ] Migration idempotent (safe re-run on deploy).
- [ ] Existing single row per target remains valid (no data rewrite required).

### Out of scope

- Changing `WalletWriter` / `TLSWriter` logic (IMM-3).
- HTTP API behavior (IMM-4).

### Risks

- Deploying IMM-2 **without** IMM-3 while writers still upsert by target may cause duplicate-key attempts or inconsistent state — coordinate via IMM-8 runbook.

### Implementation hints

- `cmd/persistence/main.go` (index creation)
- GORM `AutoMigrate` unchanged for entity shape; PK remains `id` UUID

**Suggested branch:** `discovery/scan-history-db-migration`  
**Suggested PR title:** `Discovery: DB migration for per-execution scan history`

### Test plan

- [ ] `go test` for migration/persistence package if present
- [ ] Manual: second insert same `(user_id, address)` with different `id` succeeds after migration

### Tracking

| Field | Value |
|-------|--------|
| Issue | — |
| PR | — |

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

- [ ] `OnStarted`: inserts row with `id = scan_id`, status `RUNNING`; does **not** no-op when another terminal row exists for the same address/URL.
- [ ] `OnCompleted` / `OnFailed`: update `WHERE id = scan_id`; optional insert-if-missing for event replay without target-level upsert.
- [ ] Re-scan same wallet address: **two** rows with distinct `scan_id`; first row’s `result` unchanged when second completes.
- [ ] Same behavior for TLS scans (per `url`).
- [ ] Invalid transition handling remains per `scan_id` (duplicate completion ignored with log).
- [ ] Unit tests in `internal/persistence/storage` cover two `scan_id`s, one address.

### Out of scope

- `GET /discovery/v1/wallets/scans` filter fix (IMM-4).
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

## GitHub issue — IMM-4

### Title (copy as-is)

```
[Discovery][IMM-4] v1 wallet scan list: history filters and latest=true query
```

### Labels (suggested)

`discovery` · `scan-history` · `api` · `v1`

### Repository

`create2-labs/cafe-discovery`

### Body (copy below the line)

---

**Type:** Technical task (API bugfix vs WORKPLAN).

**Tracking ID:** IMM-4  
**Plan:** [IMMUTABILITE_PR.md — IMM-4](https://github.com/create2-labs/cafe-discovery/blob/main/IMMUTABILITE_PR.md#github-issue--imm-4)  
**Depends on:** IMM-3 (multiple rows per address possible)

### Summary

Implement **WORKPLAN_API.md §4.2.1** list filters and **`?latest=true`** on **`GET …/wallets/scans`** (no dedicated `/latest` route).

### Acceptance criteria

- [ ] `?address=` only: all executions for address, paginated.
- [ ] `?address=&chain_id=`: list query + `walletEntityMatchesChainID`; **not** single-row lookup only.
- [ ] `?address=&latest=true`: **≤1** item, last **`completed`** only (**W2**); `total` 0 if no **`completed`** exists; **400** if `latest=true` without `address`.
- [ ] `POST …/scan`: **409** `SCAN_IN_PROGRESS` only if newest is `requested` / `started`; **allowed** if newest is **`failed`**.
- [ ] `?chain_id=` without `address` → **400** (unchanged).
- [ ] Default sort: `created_at` desc, `scan_id` desc.
- [ ] OpenAPI: query param **`latest`** documented.
- [ ] Contract tests: `latest=true` picks last **`completed`**; POST blocked in-flight only; **`completed` + newer `failed`** → POST OK.

### Out of scope

- OpenAPI change (already describes immutability).
- TLS list (no address filter).

### Implementation hints

- `internal/handler/discovery_v1_scans.go` — `ListDiscoveryV1WalletScans`
- `internal/contract/wallet_scans_v1_test.go`

**Suggested branch:** `discovery/scan-history-api-list-filters`  
**Suggested PR title:** `Discovery v1: fix wallet scan list when multiple rows per address`

### Test plan

- [ ] `go test ./internal/handler/... ./internal/contract/...`

### Tracking

| Field | Value |
|-------|--------|
| Issue | — |
| PR | — |

---

## GitHub issue — IMM-5

### Title (copy as-is)

```
[Discovery][IMM-5] Align Redis and legacy scan read paths with per-scan_id history
```

### Labels (suggested)

`discovery` · `scan-history` · `redis` · `legacy`

### Repository

`create2-labs/cafe-discovery`

### Body (copy below the line)

---

**Type:** Technical task (consistency).

**Tracking ID:** IMM-5  
**Plan:** [IMMUTABILITE_PR.md — IMM-5](https://github.com/create2-labs/cafe-discovery/blob/main/IMMUTABILITE_PR.md#github-issue--imm-5)  
**Depends on:** IMM-3

### Summary

Postgres holds multiple scans per address; Redis and legacy services still assume **one result per `(user_id, address)`**. Align behavior so v1 remains Postgres-backed and Redis does not hide or delete history incorrectly.

### Acceptance criteria

- [ ] `DiscoveryService.getExistingScan` / synchronous `ScanWallet` documented or updated so re-scan policy matches product (no silent return of stale single row blocking new execution, unless path is explicitly deprecated).
- [ ] `UserScanCacheService` warm/read-through: when multiple wallet rows share an address, Redis stores **latest by `created_at`** OR skips address-key cache for v1 (per IMM-1 decision); behavior documented in code comment.
- [ ] `DeleteDiscoveryV1WalletScan`: Redis `DeleteByUserIDAndAddress` only when **no** remaining Postgres rows for that address.
- [ ] No regression for v1 `GET …/wallets/scans/{scan_id}` (Postgres path).

### Out of scope

- Full Redis schema migration to `scan_id` keys.

### Implementation hints

| File | Role |
|------|------|
| `internal/service/discovery.go` | Legacy scan short-circuit |
| `internal/service/user_scan_cache.go` | Warm / read-through |
| `internal/handler/discovery_v1_scans.go` | DELETE + Redis |

**Suggested branch:** `discovery/scan-history-redis-legacy-readpaths`  
**Suggested PR title:** `Discovery: align Redis and legacy paths with scan history model`

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

After IMM-3, each completed scan is a **row**. Plan limits (`WalletScanLimit`, `EndpointScanLimit`) should count **executions** (rows), not implicit unique addresses/URLs. Confirm `CountByUserID` and pre-scan checks in `POST /discovery/v1/scan` match this.

### Acceptance criteria

- [ ] `CountByUserID` on wallet/TLS repos counts all owner rows (respecting soft-delete if applicable).
- [ ] Second `POST /discovery/v1/scan` for same address increments usage when limit is finite.
- [ ] Limit reached → **403** with existing error shape.
- [ ] Tests for plan service / handler limit check updated or added.

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
**Depends on:** IMM-3, IMM-4

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

- May merge with IMM-4 PR if small; otherwise dedicated branch after IMM-3+4 on `main`.

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
- [ ] States: deploy persistence migration **before** or **with** writers that insert per `scan_id`; avoid IMM-2-only window with old upsert writers in production.
- [ ] Release note: re-scan creates new `scan_id`; old policies keep valid `scan_id` until DELETE.
- [ ] Optional: link to `cafe-crypto-policy-mgt` smoke script `test-discovery-v1-wallet-scans-to-cpm.sh` for post-deploy check.

### Out of scope

- Implementing IMM-2/3 code (cafe-discovery issues).
- Compose version bumps unless required for doc accuracy.

### Implementation hints

- `cafe-deploy/README.md`, compose service order for discovery + persistence

**Suggested branch:** `deploy/scan-history-migration-runbook`  
**Suggested PR title:** `Deploy: runbook for Discovery scan history migration`

### Test plan

- [ ] Dry-run review with operator checklist
- [ ] Post-deploy: list + detail v1 + optional CPM smoke on staging

### Tracking

| Field | Value |
|-------|--------|
| Issue | — |
| PR | — |

---

## GitHub issue — IMM-9

### Title (copy as-is)

```
[Discovery][IMM-9] Block wallet POST /scan when a persisted CPM exists for target address
```

### Labels (suggested)

`discovery` · `scan-history` · `cpm` · `api`

### Repository

`create2-labs/cafe-discovery` (+ coordination `cafe-crypto-policy-mgt`)

### Body (copy below the line)

---

**Type:** Technical task (**WORKPLAN §2.2 W1** + garde en cours, **IMM-4**).

**Tracking ID:** IMM-9  
**Plan:** [IMMUTABILITE_PR.md — W8, W1](https://github.com/create2-labs/cafe-discovery/blob/main/IMMUTABILITE_PR.md#règles-produit-wallet--cpm-référence-workplan-22-w1w8)

### Summary

Before `POST /discovery/v1/scan` with `{ "address": "0x…" }`:

1. **Scan en cours** (**IMM-4**) — newest row `requested` / `started` → **409** `SCAN_IN_PROGRESS`.
2. **W1** — persisted CPM for **target_address** → **409** `CPM_EXISTS_FOR_WALLET_TARGET`.

Newest row **`failed`** → **allow** new scan (retry until **`completed`**).

### Acceptance criteria

- [ ] In-flight guard **before** **W1** (reuse IMM-4 helper).
- [ ] Newest **`failed`** → new scan **accepted** (if quota + **W1** OK).
- [ ] **W1**: **409** `CPM_EXISTS_FOR_WALLET_TARGET` when policy exists.
- [ ] OpenAPI documents `SCAN_IN_PROGRESS` + `CPM_EXISTS_FOR_WALLET_TARGET`.
- [ ] Integration tests: in-flight blocked, failed retry OK, CPM exists.

### Dependencies

**IMM-3**, **IMM-4** (in-flight guard on POST).

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
[CPM][IMM-10] Policy explore/persist only when latest scan is completed (W7 + W2)
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

`POST …/policies/decisions/explore` and `POST …/policies` for wallet targets:

1. **W7 (CPM)** — reject if the **newest** scan row is not **`completed`** (**400** `LATEST_SCAN_NOT_COMPLETED`) — includes **`failed`** and in-flight (even when an older **`completed`** exists).
2. **W2** — reject if `scan_id` ≠ latest **`completed`** (**400** `SCAN_ID_NOT_LATEST_FOR_TARGET`). **Canonical latest:** Discovery **`GET …/wallets/scans?address=&latest=true`** (**IMM-4**).

### Acceptance criteria

- [ ] **W7** before **W2** on every explore/persist.
- [ ] **400** when newest is **`failed`** or in-flight; **`completed` A + `failed` B** → **400** even for `scan_id` A.
- [ ] **400** `SCAN_ID_NOT_LATEST_FOR_TARGET` when `scan_id` is historical.
- [ ] Latest resolved via **`?latest=true`** — not `limit=1` alone.
- [ ] Contract tests: in-flight, failed, historical `scan_id`.

### Dependencies

Discovery **IMM-4** (`latest=true`); IMM-3 multi-row history.

### Tracking

| Field | Value |
|-------|--------|
| Issue | — |
| PR | — |

---

## GitHub issue — IMM-12

### Title (copy as-is)

```
[Discovery][IMM-12] GET /discovery/v1/wallets/scans/{scan_id}/cbom on demand
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

Expose CycloneDX CBOM per wallet scan via v1 API. CBOM is **generated on read** from persisted scan fields (`ToCBOM()`), not stored. Restores user-facing CBOM per historical `scan_id` after PR13c removed `/discovery/cbom/*`.

### Acceptance criteria

- [ ] `GET /discovery/v1/wallets/scans/{scan_id}/cbom` → **200** JSON CBOM when scan exists and terminal.
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

## Références

| Document | Rôle |
|----------|------|
| [`WORKPLAN_API.md`](../cafe-crypto-policy-mgt/workplans/WORKPLAN_API.md) §2.2, §4.2, §8.4 | Contrat cible |
| [`WORKPLAN_API_PR.md`](../cafe-crypto-policy-mgt/workplans/WORKPLAN_API_PR.md) | PR4 list/detail, PR6 DELETE — prérequis livrés |
| [`docs/CPM_OPTION_A_DISCOVERY_V1_CONTRACT.md`](./docs/CPM_OPTION_A_DISCOVERY_V1_CONTRACT.md) | Mapping CPM ↔ v1 |
| [`openapi/discovery-v1.yaml`](./openapi/discovery-v1.yaml) | Spec immutabilité `result` |
| [`internal/persistence/storage/postgres.go`](./internal/persistence/storage/postgres.go) | Writers à modifier (**IMM-3**) |
