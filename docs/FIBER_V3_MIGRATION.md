# Fiber v3 — plan de migration (`cafe-discovery`)

**Statut :** backlog (pas bloquant releases)  
**Cible :** `github.com/gofiber/fiber/v3` ≥ **v3.4.0**  
**Source backlog :** [cafe-deploy/TODO.md — Fiber v3](https://github.com/create2-labs/cafe-deploy/blob/main/TODO.md)  
**Prérequis :** déjà sur `fiber/v2` **v2.52.14**, version qui **corrige** [CVE-2026-45045](https://nvd.nist.gov/vuln/detail/CVE-2026-45045) (spoofing `X-Real-IP` dans `proxy.BalancerForward` — non utiliséé ici). La migration v3 n’est **pas** requise pour cette CVE. Go module déjà en **1.26.4** (Fiber v3 exige Go ≥ 1.25).

**Contrainte review :** chaque **commit / slice** ≤ **~400 lignes** (`git show --stat` / diff du commit). La PR cutover peut dépasser 400 **au total**.  
**Contrainte CI :** aucune PR mergeable non compilable — required checks verts sur le tip avant merge.  
**Contrainte produit :** aucun changement de contrat HTTP public (routes, status, JSON).

---

## 1. Inventaire actuel (v2)

| Métrique | Valeur |
| --- | --- |
| Fichiers important Fiber | **27** (dont `cmd/cli/tls-scan`) |
| Signatures `*fiber.Ctx` | **~95** |
| `c.BodyParser` | **6** (`auth.go`×3, `cafe_wallet.go`×2, `discovery.go`×1) |
| `c.QueryInt` | **2** (`helpers.go`) |
| `c.Context()` (fasthttp user ctx) | **~31** (handlers scan/auth/authz) |
| `DisableStartupMessage` | **~36** (surtout tests) |
| `app.Test(...)` | **~71** appels (souvent `app.Test(req, -1)`) |
| Middleware upstream | `cors` ; `adaptor` **seulement** pour `/metrics` (pas de `proxy.BalancerForward`) |
| JWT | middleware **maison** (`internal/middleware/jwt.go`) — pas `contrib/jwt` |

**Modules Go dans le repo :**

| Module | Path | Fiber ? | Action |
| --- | --- | --- | --- |
| `cafe-discovery` | `/go.mod` (racine) | **oui** — `fiber/v2` v2.52.14 | migrer (D1–D7c) |
| `cafe/pq-scan` | `cmd/cli/tls-scan/go.mod` | **oui** — blank import dans `tools.go` | migrer (D8) |
| `walletscan` | `cmd/cli/wallet-scan/go.mod` | **non** — vérifié : aucun `gofiber` dans `go.mod` / sources | **hors scope** ; re-vérifier à D8 |

Ne pas se contenter d’un `rg` à la racine pour l’acceptance : chaque `**/go.mod` imbriqué doit être trié (migré ou explicitement « pas concerné »).

---

## 2. Breaking changes applicables

| v2 | v3 | Où chez nous |
| --- | --- | --- |
| `github.com/gofiber/fiber/v2` | `.../fiber/v3` | tous les fichiers listés |
| `func(c *fiber.Ctx)` | `func(c fiber.Ctx)` | handlers, middleware, tests |
| `c.BodyParser(&x)` | `c.Bind().Body(&x)` | 6 sites |
| `c.QueryInt("k", def)` | `fiber.Query[int](c, "k", def)` | `helpers.go` |
| `c.Context()` (v2 user ctx) | **à auditer** — voir §2.1 (pas un remplacement systématique par `c`) | ~31 appels handlers → persistence/read/authz |
| `fiber.Config{DisableStartupMessage: true}` | retirer du `Config` ; si `Listen`, utiliser `fiber.ListenConfig{DisableStartupMessage: true}` | tests + évent. `Listen` |
| `app.Test(req, -1)` | `app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})` | tous les `*_test.go` Fiber |
| `cors.Config` strings CSV | slices `[]string` (`AllowOrigins`, `AllowMethods`, `AllowHeaders`, `ExposeHeaders`) | `internal/app/container.go` |
| `middleware/adaptor`, `middleware/cors` | `cors` sous `fiber/v3/middleware/...` ; **adaptor optionnel** — voir §2.2 `/metrics` | `container.go`, `metrics_test.go` |

**Hors scope (follow-up optionnel) :** typage `fiber.Locals[uuid.UUID](c, "user_id")` — `c.Locals` reste valide.

### 2.1 Audit — `c.Context()` et cancellation (obligatoire avant cutover)

En Fiber v3, `fiber.Ctx` est une **interface** qui satisfait aussi `context.Context`. Les docs upstream soulignent des changements sur le contexte requête / cancellation : **ne pas** traiter `c.Context() → c` comme un remplacement mécanique « équivalent ».

Aujourd’hui Discovery passe `c.Context()` à de nombreux appels sync (ex. `scanRead.ListWalletScans(c.Context(), …)` dans `discovery_v1_scans.go`, idem CBOM / delete / authz / pending).

**Avant chaque site `c.Context()` :**

1. Le `context` est-il **uniquement** utilisé pendant le handler (appel bloquant, pas de goroutine, pas de retention après `return`) ?
2. Un service / client / repo **conserve-t-il** ce ctx (champ struct, cache, goroutine fire-and-forget, queue) après le retour du handler ?
3. Y a-t-il déjà un timeout métier explicite (`context.WithTimeout(c.Context(), …)` — ex. auth) à préserver ?

**Choix par site (après audit) :**

| Cas | Préférer |
| --- | --- |
| Appel sync borné à la durée du handler, aucun escape du ctx | `c` **ou** garder `c.Context()` si l’API v3 l’expose encore — documenter le choix dans le commit D3/D4 |
| Ctx potentiellement retenu / async / hors durée de vie requête | **ne pas** passer `c` nu ; dériver `context.WithTimeout` / `WithCancel` (parent = ctx requête ou `context.Background()` selon le besoin) et s’assurer que le travail n’est pas le ctx Fiber annulé trop tôt / trop tard |
| Timeout métier déjà présent | conserver le `WithTimeout` ; ne changer que le parent si l’audit le justifie |

**Review D3/D4 :** checklist explicite « audit ctx » sur tous les `c.Context()` touchés ; pas de search-replace global sans triage.

### 2.2 `/metrics` — adaptor vs handler `net/http` natif

Aujourd’hui :

```go
app.Get("/metrics", adaptor.HTTPHandler(metrics.Handler()))
```

Fiber v3 enregistre directement un `http.Handler` / `http.HandlerFunc` sur les routes (adaptation à l’enregistrement ; le wrapping manuel `adaptor` n’est plus requis pour ce cas — [docs What’s new / Direct net/http handlers](https://docs.gofiber.io/whats_new/)).

**Préférence D5 :**

```go
app.Get("/metrics", metrics.Handler()) // prometheus http.Handler
```

| | |
| --- | --- |
| **Faire** | Tenter le register direct ; mettre à jour `metrics_test.go` de la même façon ; supprimer l’import `middleware/adaptor` s’il n’a plus d’usage |
| **Repli** | Si comportement scrape / status / body diverge → garder `adaptor.HTTPHandler(...)` sous `fiber/v3/middleware/adaptor` et noter pourquoi dans le commit |
| **Contrainte** | Contrat scrape inchangé (Prometheus text exposition) ; sémantique `net/http` (pas de `fiber.Ctx` dans le handler metrics) — acceptable pour `/metrics` |

Ne pas réécrire `metrics.Handler()` en handler Fiber natif dans cette migration (hors scope perf).

**Outil :** `go install github.com/gofiber/cli/fiber@latest` puis `fiber migrate --to v3` sur une branche jetable pour générer le diff de référence ; **réorganiser** en commits thématiques (slices ci-dessous), ne pas pousser le dump monolithique tel quel.

---

## 3. Stratégie CI / PR (décision)

Fiber v2 et v3 ne peuvent pas coexister dans le même module compilable. Empiler des PRs « does-not-compile » est **incompatible** avec des required checks par PR.

**Règle :**

| Livrable | Forme | CI |
| --- | --- | --- |
| **PR D0** | Docs only | N/A / docs |
| **PR D-CUTOVER** | **Une** PR mergeable, **multi-commits** (slices D1→D6, D7a/b/c, D8) | tip vert : `go test ./...` **+** modules CLI imbriqués (§5) + lint/vuln |
| Scanners | PRs séparées (plans TLS/wallet) | après Discovery sur `main` |

**Workflow auteur :**

1. Branche `chore/fiber-v3` depuis `main`.
2. Cutover local jusqu’à tip vert (`go test ./...` **et** modules imbriqués §5), **puis** `git reset` / recommits en slices D1…D6, D7a/b/c, D8 (chaque commit ≤ ~400 lignes).
3. Ouvrir **une** PR vers `main` ; description = checklist de review par slice (modèle §4).
4. Reviewer coche slice par slice (`git show <sha>` / fichiers listés) ; merge squash **interdit** si on veut garder l’historique des slices — préférer **merge commit** ou rebase+merge **sans squash**, selon la convention du repo. Si squash obligatoire : la checklist reste le guide de review, l’historique slices est perdu après merge (acceptable).
5. Enchaîner immédiatement `cafe-scanner-tls` + `cafe-scanner-wallet`.

**Mesure par slice :**

```bash
git show --stat <commit>
# insertions + deletions ≤ ~400
```

---

## 4. Slices (commits dans PR D-CUTOVER)

Chaque slice = **un commit** nommé `fiber-v3: D<n> — …`.  
Hors PR cutover : **PR D0** (ce plan) peut merger seule avant.

### Commit D1 — Dépendances Fiber v3

| | |
| --- | --- |
| **Scope** | `go.mod` / `go.sum` racine : `fiber/v2` → `fiber/v3@v3.4.0` (ou patch ≥ 3.3.0) ; `go mod tidy` |
| **Est. lignes** | ~80–350 (surtout `go.sum`) |
| **Review** | pin explicite ; plus de `fiber/v2` dans le module racine |

Si `go.sum` seul > 400 : deux commits D1a/D1b (require puis tidy).

---

### Commit D2 — Middleware / metrics / helpers

| | |
| --- | --- |
| **Fichiers** | `internal/middleware/jwt.go`, `internal/middleware/internal_service_auth.go`, `internal/handler/helpers.go`, `internal/metrics/http.go` |
| **Changements** | imports `v3` ; `*fiber.Ctx` → `fiber.Ctx` ; `QueryInt` → `fiber.Query[int]` |
| **Est. lignes** | ~80–150 |
| **Review** | signatures Ctx ; pagination helpers |

---

### Commit D3 — Handlers « petits » + bind body

| | |
| --- | --- |
| **Fichiers** | `auth.go`, `cafe_wallet.go`, `plan.go`, `scan_authz.go`, `scan_read_errors.go`, `discovery_v1_cbom.go`, `discovery_v1_scan_result.go` |
| **Changements** | Ctx ; `BodyParser` → `Bind().Body` ; **audit** `c.Context()` (§2.1) — pas de remplacement systématique |
| **Est. lignes** | ~200–320 |
| **Review** | 0× `BodyParser` ; erreurs bind → toujours 400 ; chaque ancien `c.Context()` justifié (sync OK / timeout / pas d’escape) |

---

### Commit D4 — Handlers Discovery / TLS scan

| | |
| --- | --- |
| **Fichiers** | `internal/handler/discovery.go`, `internal/handler/discovery_v1_scans.go` |
| **Changements** | Ctx ; `Bind().Body` (PostScan) ; **audit** `c.Context()` sur scan read/pending/delete (§2.1) |
| **Est. lignes** | ~150–280 |
| **Review** | pas de logique métier (guards, quota, pending, delete) ; audit ctx (ex. `ListWalletScans` / `Get*` / `Delete*`) |

Si > 400 : commits D4a (`discovery.go`) / D4b (`discovery_v1_scans.go`).

---

### Commit D5 — App bootstrap (CORS, adaptor, Listen)

| | |
| --- | --- |
| **Fichiers** | `internal/app/container.go`, `internal/app/discovery_v1_routes.go`, **`internal/app/cors_csv_test.go`** (nouveau) |
| **Changements** | imports `v3` + middleware ; helper `splitCSV` + CORS `[]string` ; **`/metrics`** via `metrics.Handler()` direct (§2.2) ou repli adaptor ; `Listen` / `ListenConfig` si besoin ; buffers PQC inchangés |
| **Est. lignes** | ~150–280 (container + test helper) |
| **Review** | `splitCSV` + table de cases dans `cors_csv_test.go` (exécution CI : D6+) ; `/metrics` scrapable **sans** `adaptor` si possible ; env documentées inchangées |

**CORS — pattern obligatoire :**

Viper reste en **CSV string** (`CORS_ALLOW_ORIGINS`, etc.) ; Fiber v3 exige des **slices** pour `AllowOrigins`, `AllowMethods`, `AllowHeaders`, `ExposeHeaders`. Convertir uniquement au câblage — ne pas changer le format env.

```go
origins := splitCSV(corsOrigins)   // TrimSpace, drop empty segments
methods := splitCSV(corsMethods)
app.Use(cors.New(cors.Config{
    AllowOrigins:     origins,
    AllowMethods:     methods,
    AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
    AllowCredentials: true,
    ExposeHeaders:    []string{"Content-Length"},
    MaxAge:           60,
}))
```

**Test unitaire `splitCSV` obligatoire** — ajouter `cors_csv_test.go` **dans le commit D5** (à côté du helper), cases :

| Entrée | Attendu |
| --- | --- |
| `"https://a.example, https://b.example"` | `["https://a.example", "https://b.example"]` (espaces trimés) |
| `""` / `"   "` | `nil` ou `[]string{}` (documenter le choix ; Fiber doit recevoir une slice vide, pas `[""]`) |
| `"*"` | `["*"]` (wildcard conservé tel quel) |
| `"a,,b,"` | `["a", "b"]` (segments vides droppés) |
| `" GET , POST "` | `["GET", "POST"]` |

**Validation CI du test :** pas à l’étape D5 seule. `cors_csv_test.go` vit dans `internal/app`, où d’autres `*_test.go` Fiber restent non migrés jusqu’à D6 — `go test ./internal/app/...` **échoue encore** après D5. Ne pas créer un sous-package isolé juste pour ça.

→ Le test `splitCSV` est **écrit en D5**, **exécuté / vert à partir de D6** (package `app` compilable) et reconfirmé au **tip D-CUTOVER**. Review D5 : lire le fichier de test + la table ci-dessus ; ne pas exiger un `go test` vert du package à ce commit.

Sans ce test au tip, une régression silencieuse (espace collé à l’origine, `""` dans la slice, perte de `*`) casse le preflight en staging seulement.

Après D5 seul, le tip **ne compile pas encore** les tests Fiber restants — enchaîner D6 → D7a/b/c → D8 **dans la même PR** avant merge.

---

### Commit D6 — Tests infra (app / middleware / metrics)

| | |
| --- | --- |
| **Fichiers** | `version_test.go`, `utility_v1_routes_test.go`, `discovery_v1_routes_test.go`, `internal_service_auth_test.go`, `metrics_test.go`, `discovery_v1_scans_test.go`, `planquota_imm6b8_test.go` |
| **Changements** | imports ; Ctx ; drop `DisableStartupMessage` ; `app.Test` → `TestConfig` ; `/metrics` aligné D5 (handler direct ou adaptor) |
| **Est. lignes** | ~250–380 |
| **Review** | `Timeout: 0, FailOnTimeout: false` partout où on avait `-1` ; test Fiber `/metrics` sans `adaptor` si D5 l’a retiré ; **`TestSplitCSV` (ou équivalent) vert** dans ce package |

---

### Commits D7a / D7b / D7c — Tests handlers + contract (**pré-scindés**)

Ne pas regrouper en un seul D7 : `wallet_scans_v1_test.go` seul concentre beaucoup de `fiber.New`, `app.Use(func(c *fiber.Ctx)…)`, `app.Test(req, -1)` et `DisableStartupMessage` (~14 setups) — un commit unique dépasse facilement le budget review.

**Changements communs :** imports `v3` ; `fiber.Ctx` ; drop `DisableStartupMessage` ; `app.Test` → `TestConfig{Timeout: 0, FailOnTimeout: false}`.

#### Commit D7a — authz + CBOM tests

| | |
| --- | --- |
| **Fichiers** | `internal/handler/scan_authz_test.go`, `internal/handler/discovery_v1_cbom_test.go` |
| **Est. lignes** | ~150–280 |
| **Review** | helpers `newAuthzTestApp` / Locals ; CBOM `app.Test` |

#### Commit D7b — discovery scan v1 handler tests

| | |
| --- | --- |
| **Fichiers** | `internal/handler/discovery_scan_v1_test.go` |
| **Est. lignes** | ~200–350 (fichier ~713 lignes, ~13 `fiber.New` / `app.Test`) |
| **Review** | PostScan / delete / Locals ; pas de drift métier dans les stubs |

#### Commit D7c — contract wallet scans

| | |
| --- | --- |
| **Fichiers** | `internal/contract/wallet_scans_v1_test.go` |
| **Est. lignes** | ~250–400 (fichier ~819 lignes, densément Fiber) — **slice dédiée obligatoire** |
| **Review** | chaque `app.Use` + `Locals("user_id")` + `TestConfig` ; contrats HTTP inchangés |

---

### Commit D8 — CLI tools module + clôture doc

| | |
| --- | --- |
| **Fichiers** | `cmd/cli/tls-scan/go.mod`, `go.sum`, `tools.go` ; note done dans `TODO.md` / ce doc |
| **Changements** | blank import → `fiber/v3` ; tidy module `cafe/pq-scan` |
| **Est. lignes** | ~50–200 |
| **Review** | `tls-scan` sans `fiber/v2` ; **`cmd/cli/wallet-scan` re-confirmé hors Fiber** (`rg gofiber` vide dans ce module) ; tous les `**/go.mod` du repo couverts |

---

### Modèle de description PR D-CUTOVER

```markdown
## Summary
Migrate cafe-discovery from fiber/v2 (v2.52.14) to fiber/v3.
No public HTTP contract changes. CVE-2026-45045 already fixed on v2.52.14; this PR is upstream alignment.

## Review checklist (one commit = one slice)
- [ ] D1 — go.mod / go.sum pin fiber/v3
- [ ] D2 — middleware, helpers, metrics (`fiber.Ctx`, Query generics)
- [ ] D3 — small handlers + `Bind().Body` + audit `c.Context()`
- [ ] D4 — discovery.go + discovery_v1_scans.go (no business logic drift; ctx lifetime audit)
- [ ] D5 — container CORS + `cors_csv_test.go` (écrit ; exécution vert à D6+) ; `/metrics` §2.2
- [ ] D6 — infra tests (`TestConfig`) ; **`splitCSV` tests verts**
- [ ] D7a — `scan_authz_test` + `discovery_v1_cbom_test`
- [ ] D7b — `discovery_scan_v1_test`
- [ ] D7c — `contract/wallet_scans_v1_test` (slice dédiée)
- [ ] D8 — `cmd/cli/tls-scan` → fiber/v3 ; `cmd/cli/wallet-scan` confirmé hors Fiber

## Test plan
- [ ] CI green: root + nested modules (§5), golangci-lint, govulncheck
- [ ] GET /health, GET /version
- [ ] signup/signin + one wallet scan + one TLS scan
- [ ] `splitCSV` tests green (from D6 / tip)
- [ ] CORS preflight from configured frontend origin
- [ ] GET /metrics scrapable (direct http.Handler preferred)
```

---

## 5. Checklist de validation (tip avant merge `main`)

`go test ./...` depuis la racine **ne traverse pas** les modules Go imbriqués (`cmd/cli/tls-scan`, `cmd/cli/wallet-scan`). Tip vert = les trois :

```bash
go test ./...
(cd cmd/cli/tls-scan && go test ./...)
(cd cmd/cli/wallet-scan && go test ./...)
```

Même si `wallet-scan` est hors Fiber : cohérent avec « tous les `**/go.mod` couverts ».

- [ ] `rg 'gofiber/fiber/v2' -g '*.go' -g 'go.mod'` → vide (racine **et** modules imbriqués)
- [ ] `cmd/cli/wallet-scan` : toujours sans `gofiber` (hors scope, re-vérifié)
- [ ] `go test ./...` (module racine)
- [ ] `(cd cmd/cli/tls-scan && go test ./...)`
- [ ] `(cd cmd/cli/wallet-scan && go test ./...)`
- [ ] `golangci-lint run ./...` (même version CI ; racine — lint CLI modules si le job CI les couvre déjà)
- [ ] `govulncheck ./...` (idem : lancer aussi dans chaque module imbriqué si pas couvert par le job racine)
- [ ] `splitCSV` : tests verts (depuis D6 / tip D-CUTOVER — pas attendu vert au commit D5 seul)
- [ ] Smoke manuel ou stack : `GET /health`, `GET /version`, signup/signin, `POST` scan wallet/TLS (contrat inchangé)
- [ ] CORS preflight depuis l’origine frontend configurée
- [ ] `/metrics` scrapable (préférer register `http.Handler` direct ; adaptor seulement en repli documenté)
- [ ] Audit §2.1 : aucun service ne retient un ctx Fiber au-delà du handler (ou mitigation timeout/Background documentée)

---

## 6. Risques & non-régression

| Risque | Mitigation |
| --- | --- |
| CORS `[]string` mal splité | `cors_csv_test.go` **écrit en D5**, **vert dès D6 / tip** (package `app` encore cassé après D5 seul) ; preflight smoke en tip |
| Modules CLI oubliés | `go test ./...` racine **insuffisant** — ajouter `cd cmd/cli/{tls-scan,wallet-scan} && go test ./...` (§5) |
| `Bind().Body` messages d’erreur différents | garder `StatusBadRequest` ; comparer 1–2 tests auth/wallet |
| `app.Test` timeout default ≠ `-1` | toujours `Timeout: 0, FailOnTimeout: false` là où on avait `-1` |
| `c.Context()` / cancellation v3 | **pas d’équivalence assumée** — audit §2.1 site par site ; éviter de passer `c` si le ctx escape le handler ; préférer `WithTimeout` / parent explicite si besoin |
| Diff `go.sum` explosif | slice D1 isolée ; mesurer avant commit |
| `/metrics` via adaptor | Préférer `app.Get("/metrics", metrics.Handler())` (§2.2) ; repli adaptor si scrape régresse |
| Review d’une PR large | checklist par slice + `git show` ; pas de squash si on veut garder les commits |

---

## 7. Ordre coordonné multi-repos

1. **cafe-discovery** PR D0 (docs) puis PR D-CUTOVER → `main` (ce plan)
2. **cafe-scanner-tls** puis **cafe-scanner-wallet** (plans locaux) — health-only, PRs courtes
3. Mettre à jour la ligne Fiber dans `cafe-deploy/TODO.md` (done)

Ne pas laisser Discovery en v3 et les scanners en v2 longtemps (drift ops / Dependabot).
