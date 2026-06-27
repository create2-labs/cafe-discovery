# Mini-plans PR — persistance CAFE (PERS-D*)

> **Parent / index normatif :** [ADR_20260622_persistence.md](./ADR_20260622_persistence.md) §14 (ordre + table **Prérequis**)  
> **Rôle :** **source d’exécution** — checklists détaillées par jalon. Les deux documents doivent rester alignés.

---

## Modèle de checklist (toute PR)

- [ ] Prérequis merges listés ci-dessous satisfaits
- [ ] API publiques `/api/discovery/v1` et `/api/cpm/v1` inchangées (§4.4 ADR)
- [ ] Smokes / tests listés verts en CI
- [ ] Rollback documenté (si applicable)
- [ ] README / runbook mis à jour dans le dépôt lead

---

## Suivi

Table normative : [ADR §14.2](./ADR_20260622_persistence.md#142-ordre-de-merge-recommandé-révisé) *(liens GitHub, statut, date de merge)*.

---

## PERS-D0 — Acter l’ADR

| Champ | Valeur |
|-------|--------|
| **Lead** | `cafe-discovery` |
| **Prérequis** | ADR v1.4.4+ revue / signée |
| **Scope** | Docs uniquement |

**Livrables**

- Merge `docs/ADR/ADR_20260622_persistence.md` + ce fichier `ADR_20260622_persistence_PR_PLAN.md`
- Lien depuis `cafe-discovery/README.md` (section Architecture Decisions)
- Règle explicite « zero CP in Discovery » dans `cafe-discovery/TODO.md`
- Lien depuis `cafe-crypto-policy-mgt/workplans/WORKPLAN_API.md` et `cafe-crypto-policy-mgt/TODO.md` (jalons CPM D3b–D5c)

**Non-objectifs** : aucun code, aucun changement compose.

---

## PERS-D1 — Extraction mécanique scan *(détail)*

| Champ | Valeur |
|-------|--------|
| **Lead** | `cafe-persistence` *(nouveau repo)* |
| **Prérequis** | PERS-D0 |
| **Ne touche pas** | `cafe-discovery` (code intact), `cafe-deploy` (image inchangée) |

### Objectif

Créer le repo et l’image **`oleglod/cafe-persistence`** avec le **même comportement** que `cmd/persistence` Discovery aujourd’hui.

### Fichiers / livrables attendus

| Élément | Action |
|---------|--------|
| Repo `create2-labs/cafe-persistence` | Initialiser (CI, `go.mod`, README) |
| `cmd/persistence/main.go` | Copier depuis `cafe-discovery` (historique git `git log --follow` si utile) |
| `internal/persistence/**` | Idem |
| Domaines scan partagés | Extraire ou dupliquer temporairement ce que le binaire importe aujourd’hui |
| `Dockerfile-persistence` | Basé sur `Dockerfile-discovery-persistence` |
| CI | `go test ./...` ; build image ; push registry |
| Test DDL §14.5 ADR | Intégration : `pg_indexes` / golden file index scan (IMM) |

### Critères merge

- [ ] Image `oleglod/cafe-persistence:<tag>` publishable (CI verte)
- [ ] `go test` vert sur le nouveau repo
- [ ] Boot sur DB vide → tables + index scan conformes baseline (§14.5)
- [ ] Consommation NATS `scan.*` identique (smoke manuel ou test d’intégration)
- [ ] **`cafe-discovery` inchangé** — `cmd/persistence` toujours présent (rollback §14.3)

### Rollback

N/A — cette PR n’active rien en stack. Rollback = ne pas merger D2.

### Test plan (PR description)

1. `docker build` image cafe-persistence locale
2. Stack locale : postgres + nats + redis + **nouveau** binaire (à côté de l’ancien, pas en remplacement deploy)
3. Publier un scan test → ligne `scan_results` + événement ledger
4. Assert index (script ou test Go) vs golden file

---

## PERS-D2 — Prouver stack cafe-persistence *(détail)*

| Champ | Valeur |
|-------|--------|
| **Lead** | `cafe-deploy` |
| **Prérequis** | PERS-D1 (image cafe-persistence disponible) |
| **Coordination** | Tag image aligné avec équipe (voir ci-dessous) |

### Objectif

Bascule **compose / smokes** vers `cafe-persistence` **sans** supprimer le code Discovery (D1b vient après).

### Fichiers touchés (indicatif)

| Fichier | Changement |
|---------|------------|
| `compose/20-discovery.yml` | Service persistence → image `oleglod/cafe-persistence:${PERSISTENCE_VERSION}` *(ou alias documenté)* |
| `README.md`, `docs/RUNBOOK_SCAN_HISTORY.md` | Nom service, variables, procédure rollback |
| Scripts smokes `scripts/test-discovery-*.sh` | `PERSISTENCE_CONTAINER` / nom service si renommé |
| `.env.example` | `PERSISTENCE_VERSION` ou doc `DISCOVERY_VERSION` split |

### Stratégie image (à trancher dans la PR)

| Option | Avantages | Inconvénients |
|--------|-----------|---------------|
| **A** — Renommer service `cafe-persistence`, nouvelle var `PERSISTENCE_VERSION` | Clair | Deux vars à gérer |
| **B** — Garder nom compose `cafe-discovery-persistence`, image `oleglod/cafe-persistence` | Moins de churn scripts | Nom service ≠ nom image |

**Décision PERS-D2 (mergé) :** option **B**. L’option **A** (rename service → `cafe-persistence`) est reportée à **PERS-D6d** (après D6c) — voir [§ PERS-D6d](#pers-d6d--rename-service-compose).

**Recommandation PR :** documenter le choix + commande rollback explicite.

### Critères merge

- [ ] Smokes scan v1 existants **verts** (`test-discovery-imm*`, scan history, etc.)
- [ ] Backend Discovery **inchangé** côté migrations scan (encore AutoMigrate scan tables — D2b pas encore)
- [ ] Image legacy `oleglod/cafe-discovery-persistence:${DISCOVERY_VERSION}` **toujours buildable** depuis `cafe-discovery`
- [ ] Section **Rollback** dans la PR (voir ci-dessous)

### Rollback *(à coller dans la PR)*

```bash
# 1. Re-pointer compose vers l'image legacy (exemple — adapter au choix option A/B)
# image: oleglod/cafe-discovery-persistence:${DISCOVERY_VERSION}

# 2. Redéployer persistence seul
docker compose -f compose/20-discovery.yml --env-file .env up -d cafe-discovery-persistence

# 3. Re-run smokes scan
./scripts/test-discovery-imm6b4-commit-on-success.sh
```

**Condition :** D1b **non** mergé → rebuild Discovery persistence depuis `cafe-discovery` possible.

### Test plan

1. CI smokes sur staging/dev
2. Vérifier logs persistence : migrations IMM, abonnement NATS
3. GET scan list API inchangé (régression OpenAPI paths vide)

### Non-objectifs

- Pas de suppression code `cafe-discovery`
- Pas de retrait migrations scan backend (D2b)
- Pas de module CP

---

## PERS-D1b — Cleanup Discovery *(détail)*

| Champ | Valeur |
|-------|--------|
| **Lead** | `cafe-discovery` |
| **Prérequis** | **PERS-D2 mergé et smokes verts** (obligatoire) |
| **Bloquant si** | D2 pas validé en staging |

### Objectif

Retirer le code persistence extrait ; **documenter** le rollback image legacy.

### Fichiers touchés

- Supprimer : `cmd/persistence/`, `internal/persistence/` (ou équivalent migré)
- Supprimer ou archiver : `Dockerfile-discovery-persistence` *(ou marquer deprecated une release)*
- Ajouter : `docs/PERSISTENCE_EXTRACTION.md` (ou section README) avec **table rollback**

### Table rollback *(obligatoire dans la PR)*

| Question | Réponse *(remplir dans la PR)* |
|----------|-------------------------------|
| **Où vit le tag legacy ?** | ex. Docker Hub `oleglod/cafe-discovery-persistence` ; git tag `discovery-persistence-vX.Y.Z` sur commit `abc123` |
| **Dernier tag connu bon** | ex. `DISCOVERY_VERSION=1.2.3` |
| **Durée de rétention** | ex. 90 jours / 3 releases mineures — **ne pas supprimer** l’image registry avant échéance |
| **Rebuild from source** | Commit hash si rebuild nécessaire après suppression Dockerfile |
| **Procédure** | Lien vers section rollback PERS-D2 |

### Critères merge

- [ ] `go test` Discovery vert (backend seul)
- [ ] Aucune référence cassée dans `cafe-deploy` (deploy utilise déjà cafe-persistence)
- [ ] Doc rollback complète (table ci-dessus remplie)
- [ ] Issue ou ticket « purge image legacy après \<date\> » créé si politique registry

### Rollback

Si problème **après** D1b : **pas** de rebuild in-repo — utiliser **tag registry documenté** + revert compose (PERS-D2 rollback). D’où l’importance de la rétention.

---

## PERS-D2b — DDL scan unique + boot guard *(détail)*

| Champ | Valeur |
|-------|--------|
| **Lead** | `cafe-discovery` + `cafe-persistence` (selon readiness) |
| **Prérequis** | PERS-D2 vert ; **PERS-D1b recommandé** (évite double migration) mais pas strict si ordre compose garanti |
| **Coordination** | **Merge coordonné** ou persistence readiness **avant** merge backend |

### Objectif

- Un seul owner DDL tables scan : **cafe-persistence**
- Backend ne `AutoMigrate` plus `ScanResultEntity`, `TLSScanResultEntity`, `ScanUsageEventEntity`
- Garde-fous boot : pas de trafic scan si persistence pas ready

### Fichiers touchés

| Dépôt | Fichier | Changement |
|-------|---------|------------|
| `cafe-discovery` | `internal/app/container.go` | Retirer entités scan de `runMigrations` |
| `cafe-persistence` | health / readiness | Endpoint ou log « migrations scan OK » exploitable |
| `cafe-deploy` | `compose/20-discovery.yml` | `depends_on` backend → persistence `service_healthy` ; activer healthcheck persistence |
| `cafe-deploy` | runbook ops | **Rollout gate prod** (§14.4 ADR) |

### Boot order cible

```
postgres healthy → nats started → redis healthy
  → cafe-persistence (migrations scan + NATS) → readiness OK
  → cafe-discovery-backend (migrations identity only)
  → scanners
```

### Compose (dev/staging)

- [ ] Healthcheck persistence : HTTP `/health` ou `/ready` *(à implémenter en D2b si absent)*
- [ ] `cafe-discovery-backend.depends_on.cafe-persistence.condition: service_healthy`
- [ ] Scanners : `depends_on` persistence healthy *(pas seulement `service_started`)*

### Orchestrateur prod *(section PR obligatoire)*

> `depends_on` Docker **≠** prod. Documenter dans la PR :

| Mécanisme | Exemple |
|-----------|---------|
| K8s | `readinessProbe` sur cafe-persistence ; backend `readinessProbe` échoue si persistence unreachable |
| Autre | Job pre-deploy « wait-for-persistence-ready » ; load balancer drain jusqu’à OK |
| Critère | **Aucun** trafic scan API tant que migrations scan + NATS OK côté persistence |

### Critères merge

- [ ] Backend boot sur DB existante (scan tables déjà là) — pas de régression
- [ ] Fresh install : persistence crée schème scan ; backend démarre après readiness
- [ ] Smokes scan v1 verts
- [ ] Test négatif : backend start **sans** persistence ready → échec contrôlé ou pas de bind trafic
- [ ] Runbook mis à jour (compose **et** note orchestrateur)

### Rollback

1. Revert D2b (backend remigre scan tables — safe si persistence aussi rollback ou double-migrate idempotent)
2. Si incident prod : rollback image compose (D2) **avant** de toucher aux données

**Risque documenté :** fenêtre où backend sans migrations scan et persistence down → GET/DELETE cassés ; d’où rollout gate.

---

## PERS-D3a-spec — Contrat interne scan (spec)

| Lead | `cafe-persistence` |
| Prérequis | PERS-D2b |
| Livrable | OpenAPI `internal/scan/v1` : pending, get/list, delete, ledger read, auth S2S |
| Merge | Revue Discovery ; **pas** d’impl ni de route edge |

---

## PERS-D3a-impl — HTTP interne scan

| Lead | `cafe-persistence` |
| Prérequis | PERS-D3a-spec |
| Livrable | Handlers + tests contract ; port interne non exposé NGINX |
| Test | Client HTTP mock ; pas de régression NATS async |

---

## PERS-D3b-spec — Contrat interne CP (spec)

| Lead | `cafe-persistence` + `cafe-crypto-policy-mgt` |
| Prérequis | PERS-D2b (**parallèle** D3a — pas besoin de D3a-impl) |
| Livrable | OpenAPI `internal/cp/v1` ; revue CPM §8.2 ; idempotence `draft_id` |
| Merge | CPM approuve sémantique ; Discovery non bloquante |

---

## PERS-D4 — Module CP Postgres (stockage)

| Lead | `cafe-persistence` |
| Prérequis | PERS-D3b-spec |
| Livrable | Migrations `crypto_policy_*` ; repositories / writers ; tests intégration **Postgres direct** |
| Non-objectifs | Handlers HTTP `internal/cp/v1` (→ D4b), Redis CP, routes publiques, imports module scan |

**Critères merge**

- [ ] Tables + index conformes schéma CPM (revue D3b-spec)
- [ ] Writers testés sans couche HTTP (repo layer)
- [ ] Pas d’endpoint consommable par CPM encore

---

## PERS-D4b — HTTP interne CP *(miroir D3a-impl)*

| Lead | `cafe-persistence` |
| Prérequis | PERS-D4 |
| Livrable | Handlers `internal/cp/v1` implémentant D3b-spec ; auth S2S ; tests contract |
| Non-objectifs | Exposition edge NGINX ; module scan |

**Critères merge**

- [ ] Toutes les opérations D3b-spec exposées (draft, policy, persist, existence W1/W3)
- [ ] Tests contract OpenAPI / golden responses
- [ ] Port interne documenté (`PERSISTENCE_INTERNAL_HTTP` ou équivalent)
- [ ] **Prérequis explicite de PERS-D5a** — client CPM ne merge pas sans D4b

**Test plan**

1. `curl` interne (réseau Docker) : `UpsertDraft` → `PersistDraft` → `GetPolicy`
2. Idempotence `draft_id` au retry persist
3. `CountPoliciesByScan` pour existence (prépare D6b)

---

## PERS-D5a — Client persistence CPM

| Lead | `cafe-crypto-policy-mgt` |
| Prérequis | **PERS-D4b** (API HTTP CP disponible) |
| Livrable | HTTP client ; `CPM_PERSISTENCE_URL` ; **`CPM_STORE=memory` défaut** ; mapping §5.5 |
| Test | Intégration contre cafe-persistence D4b ; prod inchangée (`memory`) |

---

## PERS-D5b — Bascule `CPM_STORE=persistence`

| Lead | `cafe-crypto-policy-mgt` + `cafe-deploy` |
| Prérequis | PERS-D5a |
| Livrable | `CPM_STORE=persistence` staging → prod ; smokes restart CP-PERSIST |
| **OwnerScopedStore** | **Conservé dans le binaire** — chemin prod inactif quand `CPM_STORE=persistence` |
| Rollback | Repasser `CPM_STORE=memory` (redéploiement env, pas de revert code) |
| Non-objectifs | Suppression du code memory (→ D5c) |

**Critères merge**

- [ ] Staging prod-like : persist → restart → GET policy OK
- [ ] Rollback documenté et testé une fois sur staging
- [ ] Fenêtre de stabilité avant D5c notée dans la PR (ex. 7–14 jours)

---

## PERS-D5c — Retrait memory prod

| Lead | `cafe-crypto-policy-mgt` |
| Prérequis | PERS-D5b stable (fenêtre écoulée, smokes verts) |
| Livrable | Suppression `OwnerScopedStore` du chemin prod (code retiré ou `//go:build dev` uniquement) |
| Rollback | **Plus** de rollback `CPM_STORE=memory` en prod — incident persistence = 503 §5.5 |
| Test | CI sans régression ; dev local peut garder memory via build tag si besoin |

---

## PERS-D6a-read — Scan read/list via D3a

| Lead | `cafe-discovery` + `cafe-deploy` |
| Prérequis | PERS-D3a-impl |
| Livrable | GET/list v1 (+ CBOM read) → client HTTP `internal/scan/v1` ; retrait lectures `scan_result_repository` / `tls_scan_result_repository` sur les handlers v1 read |
| **Hors scope** | DELETE v1, pending Redis, W8 in-flight, scan-authz interne, quotas plan, legacy `Create` wallet → **D6a-delete** / **D6a-pending** |
| **Deploy** | `compose/20-discovery.yml` : `DISCOVERY_PERSISTENCE_URL`, `DISCOVERY_PERSISTENCE_TIMEOUT_SEC`, `CAFE_PERSISTENCE_SERVICE_TOKEN` sur **cafe-discovery-backend** ; `env/*.env.template` : `DISCOVERY_PERSISTENCE_URL=http://cafe-discovery-persistence:8082` ; `scripts/contract-checks.sh` |
| **Config Discovery** | `DISCOVERY_PERSISTENCE_URL`, `CAFE_PERSISTENCE_SERVICE_TOKEN` (obligatoires au boot) ; `DISCOVERY_PERSISTENCE_TIMEOUT_SEC` (défaut 15s) |
| Test | Parité réponses API publique ; `go test ./...` ; persistence down → 503 `service_unavailable` sur read v1 |

**Critères merge**

- [x] `internal/persistence/scanread` + `scanhttp` ; handlers v1 wallet/TLS list/detail/defaults/CBOM
- [x] Boot fail-closed si URL/token absents
- [x] `cafe-deploy` : env + compose backend + contract-checks
- [ ] PR discovery + deploy mergées ; stack dev : list/get scan v1 OK après recreate `cafe-discovery-backend`
- [ ] Smoke : persistence arrêtée → GET scan v1 = 503

---

## PERS-D6a-delete — Scan delete via D3a

| Lead | `cafe-discovery` |
| Prérequis | PERS-D6a-read |
| Livrable | DELETE v1 → D3a ; invalidation cache selon spec |
| Test | W3 / immutabilité inchangés côté contrat public |

---

## PERS-D6a-pending — Scan pending via D3a

| Lead | `cafe-discovery` |
| Prérequis | PERS-D6a-delete |
| Livrable | POST accept / réservation → D3a ; **dernier** (concurrence W8) |
| Test | Tests concurrence / double accept ; smokes pending existants |

---

## PERS-D6b — W1/W3 via persistence

| Lead | `cafe-discovery` + `cafe-crypto-policy-mgt` |
| Prérequis | **PERS-D4b** (existence API), **PERS-D5c** (prod durable sans memory) |
| Livrable | Existence only §9.3 ; retrait `/internal/policies/references/*` ou proxy temporaire documenté |
| Test | Mocks `{ exists: true }` — pas de payload policy dans Discovery |

---

## PERS-D6c — E2E stack

| Lead | `cafe-deploy` |
| Prérequis | PERS-D6a-pending, PERS-D6b, PERS-D5c |
| Livrable | Smokes scan + CP + restart ; checklist §11 ADR complète |
| Merge | Sign-off ops sur rollout gates documentés |

---

## PERS-D6d — Rename service compose

| Champ | Valeur |
|-------|--------|
| **Lead** | `cafe-deploy` |
| **Prérequis** | **PERS-D6c** mergé (migration persistence fonctionnellement close) |
| **Scope** | Alignement DNS Docker / nommage ops — **option A** reportée depuis PERS-D2 |
| **Ne touche pas** | Image `oleglod/cafe-persistence` ; contrats `internal/scan/v1` et `internal/cp/v1` ; API publiques ; purge registry `oleglod/cafe-discovery-persistence` |

### Contexte

PERS-D2 a basculé l’**image** vers `oleglod/cafe-persistence` en conservant le **nom de service** `cafe-discovery-persistence` (option B). PERS-D6d aligne le nom compose/DNS sur le repo plateforme.

| Avant (option B) | Après (PERS-D6d) |
|------------------|------------------|
| Service compose `cafe-discovery-persistence` | `cafe-persistence` |
| `container_name: cafe-discovery-persistence-${ENV}` | `cafe-persistence-${ENV}` |
| `http://cafe-discovery-persistence:8082` | `http://cafe-persistence:8082` |
| Image | **Inchangée** — `oleglod/cafe-persistence:${PERSISTENCE_VERSION}` |

### Fichiers touchés (indicatif)

| Fichier | Changement |
|---------|------------|
| `compose/20-discovery.yml` | Clé service, `container_name`, `depends_on`, commentaires token |
| `compose/25-cpm.yml` | `depends_on`, commentaires token |
| `env/dev.env.template`, `env/staging.env.template`, `env/prod.env.template` | `CPM_PERSISTENCE_URL`, `DISCOVERY_PERSISTENCE_URL` |
| `env/dev.local.env` | idem (si présent dans le repo) |
| `scripts/contract-checks.sh` | Assertions URL + `depends_on` |
| `scripts/test-cpm-cp-persist-d5-restart-survival.sh` | `PERSISTENCE_CONTAINER`, `compose_restart` |
| `scripts/test-discovery-imm6b1-ledger-schema.sh` | défaut `PERSISTENCE_CONTAINER` |
| `scripts/test-discovery-imm6b4-commit-on-success.sh` | idem |
| `README.md`, `CHANGELOG.md`, `TODO.md` | références service |
| `docs/RUNBOOK_SCAN_HISTORY.md`, `docs/RUNBOOK_CP_PERSISTENCE.md` | commandes `docker compose` / `docker exec` |
| `cafe-crypto-policy-mgt/README.md`, `docs/PERS_D5B_ROLLOUT.md`, `docs/PERS_D5C_REMOVE_MEMORY.md` | exemples d’URL |
| `cafe-discovery/README.md`, `docs/PERSISTENCE_EXTRACTION.md` | tableau « compose service name » |

### Critères merge

- [ ] Service compose et `container_name` = `cafe-persistence` / `cafe-persistence-${ENV}`
- [ ] Tous les `*_PERSISTENCE_URL` internes pointent `http://cafe-persistence:8082`
- [ ] `contract-checks.sh` vert
- [ ] Smokes scan + CP + restart **verts** (`test-discovery-imm*`, `test-cpm-cp-persist-d5-restart-survival.sh`)
- [ ] Redéploiement stack dev/staging : `up -d` sur persistence, backend, cpm (recréation conteneurs requise — hostname DNS change)
- [ ] Doc rollback **image** legacy inchangée (`oleglod/cafe-discovery-persistence` sur registry — hors scope)

### Test plan

1. `scripts/contract-checks.sh`
2. `./scripts/test-discovery-imm6b4-commit-on-success.sh` (ou suite IMM pertinente)
3. `./scripts/test-cpm-cp-persist-d5-restart-survival.sh` (CPM `CPM_STORE=persistence`)
4. Vérifier résolution DNS depuis `cafe-cpm` et `cafe-discovery-backend` : `curl -fsS http://cafe-persistence:8082/ready` *(adapter port si health interne)*

### Rollback

Revert la PR compose/env/scripts ; recréer les conteneurs avec l’ancien hostname `cafe-discovery-persistence`. Aucun changement de données Postgres/Redis.

### Non-objectifs

- Renommer l’image Docker Hub ou purger `oleglod/cafe-discovery-persistence`
- Modifier le code applicatif Discovery / CPM / cafe-persistence (sauf commentaires doc)
- Exposer cafe-persistence sur l’edge public

---

## Ordre rappel (jalons sensibles)

```
D0 → D1 → D2 → D1b ─┐
              D2 → D2b ─┘

Chaîne scan : D3a-spec → D3a-impl → D6a-read → D6a-delete → D6a-pending
Chaîne CP   : D3b-spec → D4 → D4b → D5a → D5b → D5c → D6b
Ops         : D6c (E2E) → D6d (rename compose service)
```

**Règle d’or :** tant que D2 n’est pas vert, **ne pas merger D1b**.  
**Symétrie :** `D3a-impl` / `D4b` = couche HTTP interne ; `D4` / writers scan-side = stockage sans HTTP.
