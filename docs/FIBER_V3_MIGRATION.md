# Fiber v3 — plan de migration (`cafe-discovery`)

**Statut :** D-CUTOVER **done** (`fiber/v3` **v3.4.0**)  
**Cible :** `github.com/gofiber/fiber/v3` ≥ **v3.4.0**  
**Source backlog :** [cafe-deploy/TODO.md — Fiber v3](https://github.com/create2-labs/cafe-deploy/blob/main/TODO.md)  
**État actuel :** module racine sur `fiber/v3` **v3.4.0**. CVE-2026-45045 était déjà corrigée en v2.52.14 ; cette migration est un alignement upstream. Go module **1.26.6** (Fiber v3 exige Go ≥ 1.25). Les CLIs dev (`wallet-scan`, `publickey`, `tls-scan`) ont migré vers `cafe-scanner-wallet` (PR4) et `cafe-scanner-tls` (PR5).

**Livraison :** **un seul commit** sur `chore/fiber-v3` 
**Contrainte CI :** tip vert avant merge.  
**Contrainte produit :** aucun changement de contrat HTTP public (routes, status, JSON). Note : Fiber v3 émet `Content-Type: application/json; charset=utf-8` (charset ajouté) — body JSON inchangé.

---

## 1. Inventaire (post cutover)

| Métrique | Valeur |
| --- | --- |
| Pin Fiber | `fiber/v3` **v3.4.0** (module racine uniquement) |
| Handlers | `func(c fiber.Ctx)` |
| Bind body | `c.Bind().Body(...)` |
| Query int | `fiber.Query(c, ...)` (type inféré) |
| Ctx sync handlers | `c.RequestCtx()` (équivalent v2 `c.Context()` → `*fasthttp.RequestCtx` ; appels sync bornés au handler, pas d’escape) |
| CORS | CSV Viper → `splitCSV` → `[]string` |
| `/metrics` | `app.Get("/metrics", metrics.Handler())` — **sans** `adaptor` |
| `app.Test` | `fiber.TestConfig{Timeout: 0, FailOnTimeout: false}` |
| `cmd/cli/wallet-scan` | **migré** vers `cafe-scanner-wallet` (PR4 ; hors Fiber) |
| `cmd/cli/tls-scan` | **migré** vers `cafe-scanner-tls` (PR5 ; module tools-only hors Fiber) |

---

## 2. Breaking changes appliqués

| v2 | v3 | Chez nous |
| --- | --- | --- |
| `github.com/gofiber/fiber/v2` | `.../fiber/v3` | tous les fichiers Fiber |
| `func(c *fiber.Ctx)` | `func(c fiber.Ctx)` | handlers, middleware, tests |
| `c.BodyParser(&x)` | `c.Bind().Body(&x)` | auth, cafe_wallet, discovery |
| `c.QueryInt("k", def)` | `fiber.Query(c, "k", def)` | `helpers.go` |
| `c.Context()` (fasthttp) | `c.RequestCtx()` | handlers sync → persistence/authz (audit §2.1) |
| `DisableStartupMessage` dans `Config` | `fiber.ListenConfig{DisableStartupMessage: true}` | `Container.Start` |
| `app.Test(req, -1)` | `app.Test(req, fiber.TestConfig{...})` | tous les `*_test.go` Fiber |
| CORS CSV strings | `splitCSV` → slices | `container.go` + `cors_csv_test.go` |
| `adaptor.HTTPHandler(metrics.Handler())` | `metrics.Handler()` direct | `container.go`, `metrics_test.go` |

### 2.1 Audit `c.Context()` / `RequestCtx` (fait)

Tous les anciens `c.Context()` handlers sont des appels **sync** (scan read/pending/delete, policyRef, auth timeout parent, authz). Aucun service ne retient le ctx après le return du handler. Choix : **`c.RequestCtx()`** (migration mécanique fidèle à v2 `Context()` → fasthttp RequestCtx). Timeout métier auth conservé : `context.WithTimeout(c.RequestCtx(), 5*time.Second)`.

### 2.2 `/metrics`

Register direct `http.Handler` OK — scrape Prometheus inchangé ; tests Fiber mis à jour sans adaptor. Export mort `metrics.Registry()` retiré (`Handler()` suffit).

---

## 3. Stratégie livrée

Branche `chore/fiber-v3` ; **un commit** cutover ; tip vert (`go test ./...` + lint/vuln + `deadcode -test` sur module racine).

Les slices D1–D8 ci-dessous restent une **carte de lecture** historique du plan ; elles ne correspondent pas à des commits séparés.

---

## 4. Slices (référence plan — livrées en un seul commit)

| Slice | Scope |
| --- | --- |
| D1 | `go.mod` / `go.sum` → `fiber/v3@v3.4.0` |
| D2 | middleware, helpers, metrics HTTP |
| D3 | petits handlers + `Bind().Body` + audit ctx |
| D4 | `discovery.go` + `discovery_v1_scans.go` |
| D5 | CORS `splitCSV` + `cors_csv_test.go` + `/metrics` direct + ListenConfig |
| D6 | tests infra app/middleware/metrics |
| D7a/b/c | tests handlers + contract |
| D8 | clôture doc ; CLIs dev hors repo (wallet → scanner-wallet PR4, tls → scanner-tls PR5) |

---

## 5. Checklist de validation (tip)

- [x] `rg 'gofiber/fiber/v2' -g '*.go' -g 'go.mod'` → vide
- [x] `cmd/cli/wallet-scan` : sans `gofiber` (migré PR4)
- [x] `cmd/cli/tls-scan` : hors repo (migré PR5)
- [x] `go test ./...` (module racine)
- [x] `golangci-lint run ./...` — 0 issues
- [x] `govulncheck ./...` — 0 vulns in call graph
- [x] `deadcode -test ./...` — 0 (inclut les tests comme racines ; `Registry()` retiré)
- [x] `splitCSV` tests verts
- [x] `/metrics` via `http.Handler` direct (sans adaptor)
- [x] Audit §2.1 `RequestCtx` — sync only, pas d’escape
- [ ] Smoke manuel / stack : `GET /health`, `GET /version`, signup/signin, scan wallet/TLS, CORS preflight (staging)

Note : `deadcode ./...` **sans** `-test` signale encore du code hors chemin `main` mais encore exercé par les tests (helpers contract, stubs, chemins legacy `ScanWallet`, etc.) — hors purge Fiber.

---

## 6. Risques & non-régression

| Risque | Mitigation |
| --- | --- |
| CORS mal splité | `cors_csv_test.go` + table cases |
| `Content-Type` + charset | test version accepte préfixe `application/json` |
| `Bind().Body` messages | status 400 conservé ; tests auth/wallet verts |
| `app.Test` timeout | `Timeout: 0, FailOnTimeout: false` |
| Ctx cancellation v3 | `RequestCtx` sync only (§2.1) |

---

## 7. Coordination multi-repos

1. ~~**cafe-discovery** D-CUTOVER~~ **done** (`fiber/v3` v3.4.0) — **1 commit**
2. ~~**cafe-scanner-tls** T1~~ **done**
3. ~~**cafe-scanner-wallet** W1~~ **done**
4. Clôturer l’item Fiber v3 dans `cafe-deploy/TODO.md`
