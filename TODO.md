# Cafe Discovery — backlog

Items deferred; not blocking current IMM work unless noted.

---



## IMM-9 follow-up — Deduplicate CPM internal HTTP client (`cpmpolicyref`)

**Context:** IMM-9 added `ActiveWalletCPMContextForTarget` beside existing `PersistedPoliciesReferenceScan` in `internal/cpmpolicyref/http_client.go`.

**Improvement:** Extract a small private helper (e.g. `postInternalJSON(ctx, path, reqBody, respDest)`) shared by both methods.

**Acceptance:** Same behaviour and tests ; no change to paths or wire types.

**Repos:** `cafe-discovery` only.

---



## Retirer Moralis / RPC runtime de Discovery (slim orchestrator)

**Context:** Depuis l’extraction de `cafe-scanner-wallet`, Discovery est un **orchestrateur HTTP** : validation adresse, file NATS, lecture résultats. Le scan wallet réel (RPC + indexer Moralis/Etherscan) vit dans le scanner.

Discovery embarque encore ~**1 150 lignes** de code mort sur le chemin prod :


| Package                | Lignes (approx.) | Usage prod actuel |
| ---------------------- | ---------------- | ----------------- |
| `internal/walletscan/` | ~720             | ❌ Aucun           |
| `pkg/evm/`             | ~360             | ❌ Aucun           |
| `pkg/moralis/`         | ~75              | ❌ Aucun           |


En prod, Discovery n’appelle que `ValidateAndNormalizeAddress` (3 sites handlers) — validation locale, zéro appel réseau BC.

**Flux prod actuel :**

```
Frontend → Discovery (validate + NATS publish) → scanner-wallet (RPC + indexer) → persistence
```

**Incohérence déploiement :** `MORALIS_API_KEY` est requis sur Discovery dans compose/helm, mais la NetworkPolicy `discovery-egress` (cafe-expresso) n’autorise **pas** d’egress externe — Moralis/RPC ne sont de toute façon pas joignables depuis le pod Discovery en cluster.

**Référence Moralis → Etherscan :** le scanner-wallet reste le seul consommateur de l’indexer ; voir discussion backlog (remplacement Moralis sur `cafe-scanner-wallet` uniquement).

---



### Ce que Discovery utilise vraiment


| Besoin                     | Source                                  | Appels réseau ?   |
| -------------------------- | --------------------------------------- | ----------------- |
| Valider adresse wallet     | `DiscoveryService` → `WalletScanEngine` | Non               |
| Mapper réseau → `chain_id` | `ChainConfig`                           | Non               |
| Exposer RPC configurés     | `GET /discovery/v1/rpcs`                | Non (lit le YAML) |
| Scanner un wallet          | NATS → `cafe-scanner-wallet`            | Non (Discovery)   |


Handlers prod : `prepareWalletScanQueue`, filtres list/CBOM — **jamais** `ScanWallet()`.

`ScanWallet()` n’est appelé que dans `internal/service/discovery_test.go`.

---



### Nuance : « retirer RPC » ≠ « retirer RPC du config »


| Niveau          | Action                                      | Impact                            |
| --------------- | ------------------------------------------- | --------------------------------- |
| **A — Runtime** | Supprimer clients EVM + Moralis au boot     | ✅ Recommandé, sans régression     |
| **B — Config**  | Retirer `blockchains[].rpc` du YAML partagé | ⚠️ Casse `GET /discovery/v1/rpcs` |


Le frontend consomme encore `/rpcs` (`cafe-frontend/src/services/scanService.js` → `DISCOVERY_V1_RPCS`).

**Décision :** retirer les **clients** RPC ; **garder** `blockchains[].rpc` dans le config comme **métadonnée** pour `/rpcs`, sauf déplacement explicite de l’endpoint.

Config partagée (`cafe-deploy/config/discovery/config.yaml`) montée sur discovery, persistence et scanner-wallet :


| Champ YAML           | Discovery                | Persistence | scanner-wallet  |
| -------------------- | ------------------------ | ----------- | --------------- |
| `name`, `chain_id`   | ✅                        | ✅           | ✅               |
| `rpc`                | ✅ (lecture `/rpcs` only) | ❌           | ✅ (appels live) |
| `moralis_chain_name` | ❌ (dead)                 | ❌ (ignoré)  | ✅               |


---



### Périmètre du nettoyage

**Supprimer du binaire serveur Discovery :**


| Élément                                 | Fichiers                        | Notes                                  |
| --------------------------------------- | ------------------------------- | -------------------------------------- |
| `WalletScanEngine`                      | `internal/walletscan/*`         | Dupliqué dans `cafe-scanner-wallet`    |
| Client Moralis                          | `pkg/moralis/*`                 |                                        |
| Client EVM                              | `pkg/evm/*`                     | Utilisé seulement par walletscan + CLI |
| `ScanWallet()`                          | `internal/service/discovery.go` | Tests uniquement                       |
| `RecoverPublicKeyFromTransactionData()` | idem                            | CLI `cmd/cli/publickey`                |
| Init clients au boot                    | `internal/app/container.go`     | L.59–66, 110                           |
| Vars env Moralis (Discovery)            | config, compose, helm           | Scanner-wallet conserve la clé         |


**Conserver :**


| Élément                            | Raison                                     |
| ---------------------------------- | ------------------------------------------ |
| `ChainConfig` (`name`, `chain_id`) | Réponses API, filtres, export observations |
| `blockchains[].rpc` dans YAML      | Endpoint public `/rpcs`                    |
| Validation adresse                 | Extraire dans `internal/address/`          |
| `cafe-contracts`                   | Export observations wallet (CPM)           |


**Hors scope Discovery (lié, PR séparées) :**

- `cafe-persistence` : `MORALIS_*` dans `config.go` également mort.
- Split config discovery vs scanner (Option 3) — reporter après stabilisation Etherscan sur scanner.
- CLI dev : migrer `cmd/cli/wallet-scan` et `cmd/cli/tls-scan` vers les repos scanner (voir § ci‑dessous).
- CLI `cmd/cli/publickey/` — migrer avec wallet-scan vers `cafe-scanner-wallet`.

---



### Options (non retenues vs recommandation)


| Option                 | Effort  | Bénéfice                            | Verdict           |
| ---------------------- | ------- | ----------------------------------- | ----------------- |
| **1 — Config only**    | ½ j     | Retire secret Discovery du deploy   | Quick win partiel |
| **2 — Slim discovery** | 1.5–2 j | Code + deps + alignement archi NATS | **Recommandé**    |
| **3 — Split config**   | 2.5–3 j | Séparation nette discovery/scanner  | Reporter          |


---



### Risques


| Risque                        | Probabilité           | Mitigation                               |
| ----------------------------- | --------------------- | ---------------------------------------- |
| Dev local sans scanner-wallet | Moyenne               | Doc : scans wallet = scanner obligatoire |
| CLI `getpublickey` cassé      | Certaine si non migré | PR4 : migrer vers `cafe-scanner-wallet` |
| Docs/scripts CLI tls-scan     | Liens morts           | PR5 : migrer vers `cafe-scanner-tls`    |
| Tests `ScanWallet` perdus     | Faible                | Couverture dans `cafe-scanner-wallet`    |
| Config YAML partagée confuse  | Faible                | Commentaires dans config deploy          |


---



### Bénéfices attendus

- **Ops :** 1 secret en moins sur Discovery ; config plus claire.
- **Sécurité :** surface réduite ; cohérence avec NetworkPolicy deny-egress.
- **Maintenabilité :** fin de la duplication walletscan Discovery ↔ scanner-wallet.
- **Image Docker :** estimation −5 à 15 Mo (retrait `go-ethereum` du binaire serveur).

---



## Plan PR — slim Discovery (Option 2)

Deux PRs séquentielles. **Ne pas** faire le split config (Option 3) avant stabilisation du remplacement Moralis → Etherscan sur `cafe-scanner-wallet`.

### PR1 — Slim runtime Discovery (priorité)

**Objectif :** Discovery = orchestrateur pur ; Moralis/RPC runtime supprimés du serveur ; deploy aligné.

**Scope** `cafe-discovery` **:**

1. Créer `internal/address/` avec `ValidateAndNormalizeAddress` (extraire de `internal/walletscan/engine.go`).
2. Remplacer `DiscoveryService` dans les handlers par le validateur adresse (ou service minimal sans engine).
3. Supprimer :
  - `internal/walletscan/`
  - `pkg/moralis/`
  - `pkg/evm/` (du module serveur)
  - `ScanWallet`, `RecoverPublicKeyFromTransactionData` de `internal/service/discovery.go`
  - init EVM/Moralis dans `internal/app/container.go`
4. Retirer `MORALIS_API_KEY`, `MORALIS_API_URL` de `internal/config/config.go` (serveur).
5. Retirer `MoralisChainName` du struct `Blockchain` dans `internal/config/chain.go` (le YAML partagé peut garder le champ ; ignoré par Discovery).
6. Mettre à jour tests handlers (~15 stubs `NewDiscoveryService(nil,…)` → validateur).
7. Supprimer ou adapter `internal/service/discovery_test.go` (`TestScanWalletPersistsEachExecutionForSameAddress`).
8. `go mod tidy` — retirer `go-ethereum` du module racine si plus référencé (modules CLI migrés en PR4/PR5).

**Scope deploy / expresso (même PR ou PR1b coordonnée) :**


| Fichier                                                                   | Changement                                                                             |
| ------------------------------------------------------------------------- | -------------------------------------------------------------------------------------- |
| `cafe-deploy/compose/20-discovery.yml`                                    | Retirer `MORALIS_*` de `cafe-discovery-backend` ; **garder** sur `cafe-scanner-wallet` |
| `cafe-deploy/env/*.env.template`                                          | Moralis = scanner-wallet only                                                          |
| `cafe-deploy/README.md`                                                   | Idem                                                                                   |
| `cafe-expresso/charts/cafe-platform/values.yaml`                          | Retirer `MORALIS_API_KEY` de `discoveryBackend`                                        |
| `cafe-expresso/docs/secrets.md`, `chart-inventory.md`                     | Mettre à jour inventaire                                                               |
| `cafe-documentation/03-cafe-developer-guide.md`, `04-cafe-admin-guide.md` | Idem                                                                                   |


**Hors scope PR1 :**

- Remplacement Moralis → Etherscan (scanner-wallet).
- Split config YAML.
- Migration CLI (`cmd/cli/wallet-scan`, `cmd/cli/tls-scan`, `cmd/cli/publickey`) — PR4/PR5.

**Validation :**

```bash
cd cafe-discovery && go test ./...
# smoke stack : POST wallet scan → scanner-wallet → résultat OK sans MORALIS sur discovery-backend
curl -s http://localhost:8080/discovery/v1/rpcs | jq .
curl -s http://localhost:8080/discovery/v1/scanners | jq .
```

**Acceptance PR1 :**

- [ ] Aucun import `pkg/evm`, `pkg/moralis`, `internal/walletscan` dans le binaire serveur (`cmd/server`).
- [ ] Discovery démarre sans `MORALIS_API_KEY`.
- [ ] Scan wallet async fonctionne via NATS + scanner-wallet.
- [ ] `GET /discovery/v1/rpcs` inchangé (payload + status).
- [ ] Tests verts ; compose dev sans Moralis sur discovery-backend.

---



### PR2 — Outillage et config morte (après PR1)

**Objectif :** Nettoyer config Moralis morte et docs Discovery ; pas de migration CLI ici.

**Scope :**

1. `cafe-persistence` — retirer `MORALIS_*` de `internal/config/config.go` et `moralis_chain_name` du struct si non lu.
2. `cafe-discovery/README.md` — retirer sections « Moralis required for Discovery » ; documenter scanner-wallet comme seul consommateur indexer.
3. `docker-compose.yml`, `.env.example`, `.vscode/launch.json` — retirer Moralis du service Discovery local.
4. Commentaires dans `cafe-deploy/config/discovery/config.yaml` : champs scanner vs discovery.

**Acceptance PR2 :**

- [ ] Grep `MORALIS` dans `cafe-discovery` limité à docs historiques en attente de PR4/PR5.
- [ ] Persistence démarre sans vars Moralis.

---



### PR4 — Migrer CLI wallet vers `cafe-scanner-wallet`

**Objectif :** Les outils dev wallet vivent avec le moteur de scan wallet, plus dans Discovery.

**Contexte :**

| Source (`cafe-discovery`) | Contenu | Cible |
| --- | --- | --- |
| `cmd/cli/wallet-scan/` | Module `walletscan` ; `main.go` récupère la clé publique depuis un hash tx + RPC URL | `cafe-scanner-wallet/cmd/cli/wallet-scan/` |
| `cmd/cli/publickey/getpublickey.go` | Outil Moralis + RPC + `RecoverPublicKeyFromTransactionData` | `cafe-scanner-wallet/cmd/cli/publickey/` (fusionner ou remplacer par CLI unifié) |

Le scanner-wallet a déjà `internal/walletscan/` + `pkg/evm/` + `pkg/moralis/` — les CLI doivent **réutiliser** ces packages, pas dupliquer la logique Discovery.

**Scope `cafe-scanner-wallet` :**

1. Créer `cmd/cli/wallet-scan/` (module Go séparé ou sous-module) — porter `main.go` + README.
2. Créer `cmd/cli/publickey/` — porter ou fusionner avec un CLI `wallet-scan --address` qui s’appuie sur l’engine scanner.
3. Documenter usage local (RPC, `MORALIS_API_KEY` / futur `ETHERSCAN_API_KEY`) dans le README scanner-wallet.
4. CI : `go test` / build des CLI si applicable.

**Scope `cafe-discovery` (retrait) :**

1. Supprimer `cmd/cli/wallet-scan/`, `cmd/cli/publickey/`.
2. Retirer références dans README, `docs/FIBER_V3_MIGRATION.md`, `docs/SCAN_REFACTORING_PLAN.md`, SBOM si regénéré.
3. Stub README ou redirect : « moved to cafe-scanner-wallet ».

**Acceptance PR4 :**

- [ ] `go run ./cmd/cli/wallet-scan/...` fonctionne depuis `cafe-scanner-wallet`.
- [ ] Plus de `cmd/cli/wallet-scan` ni `cmd/cli/publickey` dans `cafe-discovery`.
- [ ] Doc dev mise à jour (discovery README + scanner-wallet README).

**Effort estimé :** 0.5–1 j.

---



### PR5 — Migrer CLI / docs TLS vers `cafe-scanner-tls`

**Objectif :** Outils et documentation OQS/OpenSSL TLS vivent avec le scanner TLS, plus dans Discovery.

**Contexte :**

| Source (`cafe-discovery`) | Contenu | Cible |
| --- | --- | --- |
| `cmd/cli/tls-scan/` | Module `cafe/pq-scan` (`tools.go` — pin deps) ; README ~700 lignes ; scripts install OQS ; Makefile | `cafe-scanner-tls/cmd/cli/tls-scan/` |
| (absent) | Pas de `main.go` scan local aujourd’hui | **Optionnel PR5b** : CLI `tls-scan <host:port>` branché sur `internal/tlsscan` |

Le repo `cafe-scanner-tls` a déjà `internal/tlsscan/`, bindings natifs OQS (`native/`), et `cmd/scanner-tls/main.go` (worker NATS). La migration est surtout **docs + scripts + module tools** ; un vrai binaire CLI local serait un plus cohérent avec wallet-scan.

**Scope `cafe-scanner-tls` :**

1. Porter `cmd/cli/tls-scan/` : README, `install_oqs_*.sh`, `Makefile`, `tools.go`, `go.mod`.
2. Renommer module si besoin (`cafe-scanner-tls/cli/tls-scan` ou garder `cafe/pq-scan`).
3. **(Optionnel PR5b)** Ajouter `main.go` : scan TLS ponctuel via `tlsscan.Engine` (sans NATS), pour debug local.
4. Mettre à jour `cafe-scanner-tls/README.md` et `TODO.md` avec point d’entrée dev.

**Scope `cafe-discovery` (retrait) :**

1. Supprimer `cmd/cli/tls-scan/`.
2. Retirer pin Fiber v3 du sous-module tls-scan dans `docs/FIBER_V3_MIGRATION.md` (Discovery seul module racine).
3. Stub README ou redirect vers `cafe-scanner-tls`.

**Acceptance PR5 :**

- [ ] Docs OQS/install accessibles depuis `cafe-scanner-tls/cmd/cli/tls-scan/README.md`.
- [ ] Plus de `cmd/cli/tls-scan` dans `cafe-discovery`.
- [ ] `(cd cafe-scanner-tls/cmd/cli/tls-scan && go mod tidy)` OK.
- [ ] (PR5b) Scan local `host:port` produit un résultat TLS/PQC comparable au worker.

**Effort estimé :** 0.5 j (docs/scripts) ; +0.5–1 j si PR5b CLI runnable.

**Dépendance :** indépendante de PR1 ; peut être faite en parallèle de PR4.

---



### PR3 (future, non bloquante) — Moralis → Etherscan sur scanner-wallet

**Repo :** `cafe-scanner-wallet` (pas Discovery).

Indexer (Moralis ou Etherscan) **uniquement** sur le scanner. Discovery non concerné après PR1.

Reporter le **split config** (Option 3) après PR3 stabilisée.

---



### Effort récapitulatif


| PR | Repo(s) | Dev | Tests/CI | Deploy/doc | Total |
| --- | --- | ---: | ---: | ---: | --- |
| PR1 — Slim runtime | `cafe-discovery`, deploy | 1 j | 0.5 j | 0.5 j | **1.5–2 j** |
| PR2 — Config morte | discovery, persistence | 0.5 j | 0.25 j | 0.25 j | **1 j** |
| PR3 — Etherscan | `cafe-scanner-wallet` | 1 j | 0.5 j | 0.5 j | **2 j** |
| PR4 — CLI wallet | discovery → scanner-wallet | 0.5 j | 0.25 j | 0.25 j | **1 j** |
| PR5 — CLI TLS | discovery → scanner-tls | 0.5 j | 0.25 j | 0.25 j | **1 j** (+0.5–1 j PR5b) |


**Ordre recommandé :**

1. **PR1** — avant fermeture compte Moralis (Discovery sans Moralis).
2. **PR3** — en parallèle ou juste après PR1 (indexer sur scanner-wallet).
3. **PR4 / PR5** — en parallèle entre eux, après ou pendant PR1 (indépendants du slim runtime).
4. **PR2** — doc/config cleanup une fois PR1 (+ ideally PR4) mergés.

**Note :** après PR4 + PR5, `cafe-discovery` ne contient plus de `cmd/cli/` — seul `cmd/server/` reste.