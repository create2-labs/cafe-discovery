# Plan d’implémentation – Redis write-through et lecture cache

## Choix retenus

- **Structure** : un hash par user (wallet et TLS).
- **TLS defaults** : clé Redis globale `tls:default:endpoints`, remplie au démarrage ou à la première requête.
- **TTL** : clés user avec TTL « session » (ex. 24h ou configurable).
- **Écriture** : Postgres d’abord, puis Redis ; en cas d’échec Redis → informer l’utilisateur et inviter à réessayer.
- **Sign-in** : après succès, avant de renvoyer la réponse, appeler le « warm user cache » (tous les wallet + TLS du user depuis Postgres vers Redis).

---

## 1. Modèle Redis

### 1.1 Clés

| Clé | Type | Contenu | TTL |
|-----|------|---------|-----|
| `user:{user_id}:wallet:scans` | Hash | field = `address` (checksum), value = JSON `ScanResult` (CBOM ou DTO) | Session (ex. 24h) |
| `user:{user_id}:tls:scans` | Hash | field = `url` (normalisé), value = JSON `TLSScanResult` (CBOM ou DTO) | Session |
| `tls:default:endpoints` | String (ou Hash) | JSON array des endpoints par défaut (CBOMs) | Long (ex. 1h ou pas de TTL, rafraîchi au démarrage) |

- **Pagination** : avec un seul hash, la liste = `HGETALL`. Pour garder limit/offset, soit on pagine en mémoire après `HGETALL` (acceptable si nombre de scans par user raisonnable), soit on ajoute plus tard une liste `user:{user_id}:wallet:order` / `user:{user_id}:tls:order` pour faire du `LRANGE` + `HMGET`. **Phase 1** : pagination en mémoire après `HGETALL` (ou tout retourner si peu d’entrées).
- **Normalisation** : pour le hash TLS, utiliser une URL normalisée (scheme + host + port + path) comme clé de champ pour éviter doublons (ex. `https://example.com` et `https://example.com/` → même champ).

### 1.2 Config

- `USER_CACHE_TTL` (ex. 24h) pour les clés `user:*`.
- `TLS_DEFAULT_CACHE_TTL` (ex. 1h) pour `tls:default:endpoints` si on met un TTL, ou remplissage unique au démarrage sans TTL.

---

## 2. Nouveaux composants

### 2.1 Repository Redis « cache user »

- **Fichier** : `internal/repository/user_scan_cache_repository.go` (ou deux fichiers `user_wallet_cache.go` / `user_tls_cache.go` si tu préfères séparer).
- **Interface** (ex. `UserScanCacheRepository`) :
  - **Wallet** : `SetWalletScans(ctx, userID, map[address]ScanResult)` (remplace tout le hash user), `GetWalletScans(ctx, userID) (map[address]ScanResult, error)`, `AddWalletScan(ctx, userID, address, result)`, `DeleteWalletScan(ctx, userID, address)` (optionnel pour plus tard).
  - **TLS** : `SetTLSScans(ctx, userID, map[url]TLSScanResult)`, `GetTLSScans(ctx, userID) (map[url]TLSScanResult, error)`, `AddTLSScan(ctx, userID, url, result)`.
  - Toutes les écritures avec `user_id` appliquent le TTL session sur la clé.
- **Implémentation** : utilisation du client Redis (ex. go-redis) : `HSET`, `HGETALL`, `EXPIRE`, sérialisation JSON des valeurs.

### 2.2 Repository Redis « defaults TLS »

- **Fichier** : `internal/repository/tls_default_cache_repository.go` (ou intégré au cache TLS si tu préfères).
- **Interface** : `GetDefaultEndpoints(ctx) ([]TLSScanResult, error)`, `SetDefaultEndpoints(ctx, results) error`.
- **Implémentation** : clé `tls:default:endpoints`, valeur = JSON array. Pas de TTL ou TTL long ; remplie au démarrage ou au premier accès (voir § 4).

### 2.3 Service « warm user cache »

- **Fichier** : `internal/service/user_cache_warm_service.go` (ou nom similaire).
- **Rôle** : `WarmForUser(ctx, userID)` :
  1. Charger tous les wallet scans du user : `scanResultRepo.FindByUserID(userID, 0, 0)` ou une méthode sans pagination (ex. `FindAllByUserID`).
  2. Charger tous les TLS scans du user (hors defaults) : `tlsScanResultRepo.FindByUserID(userID, 0, 0)` ou équivalent.
  3. Construire les maps address→result et url→result, puis appeler `UserScanCacheRepository.SetWalletScans` et `SetTLSScans`.
- **Dépendances** : `ScanResultRepository`, `TLSScanResultRepository`, `UserScanCacheRepository`.
- **Appel** : depuis le handler Signin (ou depuis AuthService si tu injectes le warm service là), après un sign-in réussi, avec le `userID` de l’utilisateur connecté.

---

## 3. Remplissage des default endpoints (Redis global)

- **Option A – Au démarrage** : dans `internal/app/container.go` (ou un `init` dédié), après création des repos :
  - Appeler `tlsScanResultRepo.FindAllDefault()`.
  - Convertir en DTO/CBOM, puis `tlsDefaultCacheRepo.SetDefaultEndpoints(ctx, results)`.
  - À faire une fois au boot du serveur API.
- **Option B – À la première requête** : dans le handler ou le service qui sert la liste TLS, si `GetDefaultEndpoints(ctx)` renvoie vide (ou clé absente), charger depuis Postgres `FindAllDefault()`, appeler `SetDefaultEndpoints`, puis utiliser le résultat.
- **Recommandation** : Option A pour éviter le premier hit Postgres sur la première liste TLS ; Option B si tu veux éviter de toucher au cycle de vie du container. Documenter le choix dans le code.

---

## 4. Write-through (worker)

### 4.1 Wallet

- **Où** : `internal/service/discovery.go`, dans `saveScanResult(userID, result)` (ou dans le worker après le Create, selon où tu centralises la logique).
- **Ordre** :
  1. `scanResultRepo.Create(scanResultEntity)` (inchangé).
  2. Si succès, `userScanCacheRepo.AddWalletScan(ctx, userID, result.Address, result)`.
  3. Si l’étape 2 échoue : ne pas rollback Postgres ; remonter une erreur dédiée (ex. `ErrCacheWrite`) pour que le worker/handler puisse informer l’utilisateur (« scan enregistré mais cache temporairement indisponible ; réessayez ou rechargez la page »).
- **Worker** : le worker appelle `DiscoveryService.ScanWallet` ; si le service retourne une erreur après le Create Postgres à cause de Redis, le worker log l’erreur et renvoie une erreur (NATS peut redélivrer ou on renvoie un message d’erreur métier). Côté API, pour les scans asynchrones, l’utilisateur sera informé soit via un message dans la réponse après un « get scan » qui fait read-through (voir § 6), soit via un message générique « en cas de problème, réessayez ».

### 4.2 TLS

- **Où** : `internal/service/tls.go`, après `tlsScanResultRepo.Create(tlsScanResultEntity)` (dans `ScanTLS`).
- **Même principe** : Create Postgres d’abord, puis `userScanCacheRepo.AddTLSScan(ctx, userID, result.URL, result)`. En cas d’échec Redis, retourner une erreur explicite pour informer l’utilisateur et l’inviter à réessayer.

### 4.3 Dépendances worker

- Le worker doit avoir accès à `UserScanCacheRepository` (et Redis). Donc dans `cmd/worker/main.go` : réintroduire la connexion Redis et le repo cache user ; injecter ce repo dans `DiscoveryService` et `TLSService` (ou dans un petit service « write-through » appelé par les deux après Create).

---

## 5. Sign-in et warm cache

- **Où** : `internal/handler/auth.go`, dans `Signin`.
- **Flux** :
  1. `response, err := h.authService.Signin(req)`.
  2. Si `err != nil`, retourner l’erreur comme aujourd’hui.
  3. Si succès : appeler `h.userCacheWarmService.WarmForUser(c.Context(), response.User.ID)` (ou `WarmForUser(c.Context(), response.User.ID)` sur un champ du handler). Attendre la fin (ou fire-and-forget avec log en cas d’erreur).
  4. `return c.JSON(response)`.
- **Recommandation** : attendre le warm (avec timeout court, ex. 5s) pour que la première liste après sign-in soit déjà servie depuis Redis. En cas de timeout ou d’erreur, logger et quand même renvoyer la réponse sign-in (le read-through rattrapera au premier list).
- **Injection** : `AuthHandler` reçoit un `UserCacheWarmService` (ou interface) dans le constructeur ; le container crée ce service et l’injecte dans `AuthHandler`.

---

## 6. Lecture (API) – uniquement Redis + read-through

### 6.1 Liste wallet (GET /discovery/scans)

- **Service** : `DiscoveryService.ListScanResults(ctx, userID, limit, offset)`.
- **Nouveau comportement** :
  1. `userScanCacheRepo.GetWalletScans(ctx, userID)`.
  2. Si erreur ou cache vide (pas de clé) → **read-through** : `scanResultRepo.FindByUserID(userID, 0, 0)` (ou méthode « all »), puis `userScanCacheRepo.SetWalletScans(ctx, userID, map)`, puis appliquer limit/offset en mémoire sur la liste et retourner.
  3. Si cache présent : convertir le hash en slice, trier si besoin (ex. par `scanned_at`), appliquer limit/offset en mémoire, retourner.
- **Total count** : soit déduire du nombre d’entrées du hash (pas de COUNT Postgres en lecture normale), soit stocker un compteur dans une clé dédiée si tu veux éviter de tout charger. Phase 1 : count = len(map).

### 6.2 Liste TLS (GET /discovery/tls/scans)

- **Service** : `TLSService.ListTLSScanResults(ctx, userID, limit, offset)`.
- **Nouveau comportement** :
  1. `userScanCacheRepo.GetTLSScans(ctx, userID)`.
  2. Si miss → read-through : `tlsScanResultRepo.FindByUserID(userID, 0, 0)` (ou équivalent), `SetTLSScans`, puis continuer.
  3. Récupérer les default endpoints : `tlsDefaultCacheRepo.GetDefaultEndpoints(ctx)`. Si vide → charger Postgres `FindAllDefault()`, `SetDefaultEndpoints`, puis utiliser.
  4. Fusionner : user scans + default endpoints, trier (ex. par date), appliquer limit/offset, retourner.

### 6.3 Get by address / Get by URL

- **Wallet** : `GetScanByAddress` → d’abord `userScanCacheRepo` (ex. une méthode GetWalletScan(ctx, userID, address)). Si miss → Postgres `FindByUserIDAndAddress`, puis `AddWalletScan` (write-through partiel), retourner.
- **TLS** : `GetTLSScanByURL` → idem avec cache TLS + éventuellement fallback default depuis `tlsDefaultCacheRepo` ou Postgres.

---

## 7. Gestion d’erreurs et messages utilisateur

- **Write-through échoue (Redis)** : après un Create Postgres réussi, si `AddWalletScan` / `AddTLSScan` échoue, retourner une erreur métier (ex. `ErrCacheUnavailable`) et ne pas annuler le Create. Côté worker : log + retour d’erreur (pour que l’appelant puissant afficher un message). Côté frontend : si le scan est « en cours » et qu’on reçoit une erreur ou un statut indiquant problème cache, afficher un message du type « Scan enregistré ; en cas de liste vide, réessayez dans quelques instants ou rechargez la page ».
- **Read-through** : si Redis est down, le read-through lit Postgres et tente de réécrire en Redis ; si l’écriture échoue, on a quand même renvoyé les données (degraded mode).

---

## 8. Ordre d’implémentation suggéré

| Phase | Tâche | Fichiers / zones |
|-------|--------|-------------------|
| **1** | Repository Redis cache user (wallet + TLS, hash, TTL) | `internal/repository/user_scan_cache_repository.go` |
| **2** | Repository Redis default endpoints | `internal/repository/tls_default_cache_repository.go` |
| **3** | Remplissage `tls:default:endpoints` au démarrage (ou première requête) | `internal/app/container.go` ou service liste TLS |
| **4** | Service WarmUserCache + `FindAllByUserID` si besoin | `internal/service/user_cache_warm_service.go`, éventuellement `FindAllByUserID` dans les repos Postgres |
| **5** | Intégration warm dans Signin (handler + injection) | `internal/handler/auth.go`, `internal/app/container.go` |
| **6** | Write-through dans `saveScanResult` (wallet) + retour d’erreur si Redis échoue | `internal/service/discovery.go`, worker si besoin |
| **7** | Write-through dans `ScanTLS` (TLS) | `internal/service/tls.go` |
| **8** | Worker : réactiver Redis et injecter cache repo dans services | `cmd/worker/main.go`, container worker si séparé |
| **9** | Lecture liste wallet : GetWalletScans + read-through + pagination en mémoire | `internal/service/discovery.go` (ListScanResults) |
| **10** | Lecture liste TLS : GetTLSScans + defaults + read-through + merge | `internal/service/tls.go` (ListTLSScanResults) |
| **11** | Get by address / Get by URL avec read-through | Handlers / services concernés |
| **12** | Tests (unitaire + intégration) et ajustement TTL / messages d’erreur | Tests, config |

---

## 9. Points d’attention

- **Pagination** : avec un hash, la phase 1 fait tout en mémoire après `HGETALL`. Si un user a beaucoup de scans (ex. >500), envisager plus tard une liste d’ordre `user:{id}:wallet:order` / `user:{id}:tls:order` et pagination côté Redis.
- **Format de valeur** : utiliser le même format (CBOM) que l’API actuelle pour éviter une double conversion (ex. stocker en Redis le JSON déjà prêt pour la réponse liste/détail).
- **Sérialisation** : normaliser les clés de hash (address en lowercase, URL normalisée) pour éviter doublons et recherches.
- **Auth anonymous** : les endpoints « list anonymous » (GET /discovery/scans/anonymous, GET /discovery/tls/scans/anonymous) ne touchent pas au cache user ; ils continuent à renvoyer uniquement les defaults (TLS) ou liste vide (wallet), sans utiliser les clés `user:*`.

Ce plan peut servir de base pour des tickets ou des PRs par phase.
