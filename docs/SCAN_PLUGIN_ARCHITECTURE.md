# Question
```
on souhaite generalier les scans; pour le moment on a scan TLS et scan wallet
on veut ajouter des scans de fichiers
mais on veut eviter la duplication de code et les boilerplate

il semble que (1) les scans doivent être implementés en tant que plugin et (2) qu'ils partagent donc des interfaces communes. ils doivent tous exporter des cbom

ne modifie aucun fichier pour le moment; on commence par reflechir à une architecture propre et modulaire, puis à un plan d'action
que peux tu me proposer?

```

# Architecture des scans en plugins (TLS, Wallet, Fichiers)

## Contexte

- **Actuel** : scan TLS et scan wallet implémentés en parallèle avec beaucoup de duplication (handlers, workers, CBOM, limites, NATS).
- **Objectif** : généraliser les scans (ajouter scan de fichiers), réduire le boilerplate et la duplication, tout en gardant chaque type de scan comme plugin et une sortie CBOM commune.

---

## 1. Principes

1. **Chaque type de scan = plugin** : enregistré au démarrage, exposant des interfaces communes.
2. **Sortie unifiée** : tous les scans produisent un **CBOM** (CycloneDX v1.7, avec champs custom CAFE).
3. **Pas de duplication** : handler HTTP générique, worker générique, construction CBOM déléguée au plugin, limites par type de scan.

---

## 2. Interfaces partagées (domaine)

### 2.1 Identité du plugin

```go
// pkg/scan/plugin.go (nouveau package)

const (
    KindWallet = "wallet"
    KindTLS    = "tls-endpoint"
    KindFile   = "file"
)

// PluginDescriptor identifie un plugin de scan (enregistrement).
type PluginDescriptor struct {
    Kind        string   // "wallet" | "tls-endpoint" | "file"
    Subject     string   // NATS subject, ex: "cafe.discovery.wallet.scan"
    PlanLimitKey string  // clé pour les limites de plan, ex: "wallet" -> WalletScanLimit
}
```

### 2.2 Entrée / Message

Chaque plugin a son type de message NATS (déjà le cas : `WalletScanMessage`, `TLSScanMessage`). Pour un file scan on aurait `FileScanMessage { UserID, FileID or Path, ... }`.

Proposition : **garder des messages typés par plugin** (pas un seul message générique avec un discriminator), pour garder la clarté et le typage. L’unification se fait au niveau des **interfaces**, pas des structs de message.

### 2.3 Résultat de scan → CBOM

Aujourd’hui les résultats sont des DTOs différents (`*domain.ScanResult`, `*domain.TLSScanResult`). Pour unifier sans tout casser :

- **Option A** : interface commune “résultat qui peut être converti en CBOM” :

```go
// ScanResult est le contrat commun : tout résultat de scan peut être exposé en CBOM.
type ScanResult interface {
    // ScanKind retourne le kind du plugin (wallet, tls-endpoint, file).
    ScanKind() string
    // ScannedAt est la date/heure du scan (pour metadata CBOM).
    ScannedAt() time.Time
    // ToCBOM produit la représentation CBOM (CycloneDX v1.7 + custom).
    // Retourne un map ou une struct sérialisable en JSON (fiber.Map / map[string]any).
    ToCBOM() (cbom map[string]any, err error)
}
```

- **Option B** : pas d’interface, mais un **type CBOM commun** et chaque plugin a une fonction `ToCBOM(result T) CBOM`.

Recommandation : **Option A** avec une interface `ScanResult` dans un package partagé (ex. `pkg/scan` ou `internal/domain`). Les DTOs existants (`ScanResult`, `TLSScanResult`) sont enrichis par des méthodes `ScanKind()`, `ScannedAt()`, `ToCBOM()` (éventuellement via des adaptateurs pour ne pas toucher aux domaines métier au début).

### 2.4 Structure CBOM commune (sortie)

Tous les CBOM partagent au moins :

- `bomFormat`: "CycloneDX"
- `specVersion`: "1.7"
- `version`: 1
- `metadata`: { `timestamp`, optionnellement `lifecycles` }
- `type`: "wallet" | "tls-endpoint" | "file"
- `components`: [] composants (primitives crypto, certificats, etc.)

Le reste (champs top-level comme `address`, `url`, `risk_score`, etc.) est spécifique au kind. On peut introduire un **helper** qui construit la base CBOM et que chaque plugin remplit :

```go
// pkg/cbom/builder.go
func BaseCBOM(scanKind string, scannedAt time.Time, components []map[string]any, metadataExtra map[string]any) map[string]any
```

Les plugins n’auraient qu’à fournir `scanKind`, `scannedAt`, `components` et les champs custom.

---

## 3. Plugin : contrat d’exécution

Chaque plugin fournit :

1. **Descriptor** : Kind, Subject, PlanLimitKey.
2. **Validation de l’entrée** : selon le kind (adresse, URL, fichier).
3. **Exécution du scan** : `Run(ctx, userID, input) (ScanResult, error)` — l’input peut être une interface ou un type générique selon ce qu’on préfère.
4. **Conversion en CBOM** : soit via `ScanResult.ToCBOM()`, soit via une fonction enregistrée `ResultToCBOM(any) (map[string]any, error)`.
5. **Persistence** : le plugin peut dépendre d’un repo connu par le core (ex. `ScanResultRepository`, `TLSScanResultRepository`) ou on introduit une interface `ScanResultStore` par kind.

Pour éviter un gros refactor d’un coup, on peut découper en :

- **Phase 1** : introduire les interfaces et un **registre de plugins** sans changer le comportement des scans existants (TLS, wallet restent comme aujourd’hui, mais implémentent le contrat).
- **Phase 2** : factoriser handler HTTP, worker et limites autour du registre.
- **Phase 3** : ajouter le plugin “file” et l’enregistrer.

---

## 4. Registre de plugins

```go
// pkg/scan/registry.go

type Runner interface {
    Run(ctx context.Context, userID *uuid.UUID, input string, opts RunOptions) (ScanResult, error)
}

type RunOptions struct {
    IsDefault bool // pour TLS : endpoint par défaut ou non
}

type Plugin struct {
    Descriptor PluginDescriptor
    Validate   func(input string) (normalized string, err error)
    Runner     Runner
    // CountByUser pour les limites de plan (optionnel si on garde PlanService tel quel au début)
    CountByUser func(ctx context.Context, userID uuid.UUID) (int64, error)
}

var registry = make(map[string]*Plugin)

func Register(p *Plugin) { registry[p.Descriptor.Kind] = p }
func Get(kind string) *Plugin { return registry[kind] }
func Kinds() []string { ... }
```

Le **handler unifié** (ex. `POST /discovery/v1/scan` étendu ou `POST /discovery/v1/scan/:kind`) :

1. Lit `kind` (wallet | tls-endpoint | file) + body (address, url, ou fileRef).
2. Récupère le plugin `Get(kind)`.
3. Appelle `Validate`, puis `checkScanLimits` (basé sur `PlanLimitKey`), publie sur le subject NATS du plugin.

Le **worker générique** (ou un worker par subject qui délègue au plugin) :

1. Reçoit un message sur un subject.
2. Détermine le kind (via le subject ou un champ dans le message).
3. Désérialise dans le type de message du plugin.
4. Appelle `Plugin.Runner.Run(...)`.
5. Persiste le résultat (déjà fait dans les services actuels).

Les limites de plan restent gérées par `PlanService` : aujourd’hui `CheckScanLimit(userID, scanType, scanResultRepo, tlsScanResultRepo)`. Pour être extensible (ex. file) sans multiplier les paramètres, on peut introduire une interface **UsageCounter** :

```go
type UsageCounter interface {
    CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
}

// PlanService prend un map[string]UsageCounter : "wallet" -> scanResultRepo, "endpoint" -> tlsScanResultRepo, "file" -> fileScanResultRepo.
func (s *PlanService) CheckScanLimit(userID uuid.UUID, scanType string, counters map[string]UsageCounter) (bool, *PlanUsage, error)
```

Ainsi l’ajout d’un nouveau type (file) = ajouter une entrée dans la map et dans la config des limites de plan (ex. `FileScanLimit`).

---

## 5. Réduction du boilerplate

| Zone | Actuel | Cible |
|------|--------|--------|
| **CBOM** | `scanResultToCBOM` + `getWalletCBOM` dupliqués ; `tlsScanResultToCBOM` + `getTLSCBOM` dupliqués | Un seul flux : résultat implémente `ScanResult.ToCBOM()` ; handler “Get CBOM” appelle `result.ToCBOM()` et renvoie le JSON. Helper `cbom.BaseCBOM()` pour la structure commune. |
| **Handler HTTP** | `scanWallet` / `scanTLS` répètent auth, limites, validation, publish | Un seul `Scan(c)` qui lit le kind + input, récupère le plugin, valide, vérifie limites, publie sur le subject du plugin. |
| **Worker** | TLSWorker / WalletWorker : même pattern (unmarshal, sémaphore, appel service) | Worker générique par subject qui reçoit le message, récupère le plugin par subject→kind, unmarshal, appelle `Runner.Run`, et le service (ou le plugin) persiste. |
| **Limites** | `CheckScanLimit(..., scanResultRepo, tlsScanResultRepo)` avec switch sur scanType | `CheckScanLimit(userID, scanType, counters map[string]UsageCounter)` ; les plans ont des champs `WalletScanLimit`, `EndpointScanLimit`, `FileScanLimit`. |
| **GetCBOM** | `GetCBOM` fait un if 0x → wallet, sinon URL → TLS, puis appelle getWalletCBOM ou getTLSCBOM (avec duplication de la construction CBOM) | `GetCBOM(c)` : déterminer le kind à partir du param (0x, http(s), ou identifiant fichier), charger le résultat via le plugin (GetByID / GetByAddress / GetByURL), puis `result.ToCBOM()` et retour. |

---

## 6. Scan de fichiers (nouveau plugin)

- **Kind** : `"file"`.
- **Entrée** : identifiant de fichier (upload précédent) ou chemin + contenu (selon modèle d’upload). Message NATS : `FileScanMessage { UserID, FileRef }`.
- **Exécution** : metadata inspection of structured containers (CMS/PKCS#7, JWT/JWE, PEM/X.509, custom JSON headers) and classification (legacy/hybrid/pq-ready/unknown).
- **Résultat** : type `FileScanResult` avec `ScanKind() string`, `ScannedAt() time.Time`, `ToCBOM() (map[string]any, error)`.
- **Stockage** : nouvelle table + repo `FileScanResultRepository` implémentant `UsageCounter`.
- **Limites** : ajout de `FileScanLimit` dans les plans et dans `PlanUsage`, et d’une entrée "file" dans la map de counters.

Les détails du format CBOM “file” (structure des `components`) sont à définir (ex. un composant par primitive ou par fichier analysé).

---

## 7. Plan d’action (sans modifier les fichiers tout de suite)

### Phase 1 – Interfaces et CBOM commun

1. Créer le package `pkg/scan` (ou `internal/scan`) avec :
   - `PluginDescriptor`, `ScanResult` (interface), `Plugin`, `Runner`, `RunOptions`.
   - Registre : `Register`, `Get`, `Kinds`.
2. Créer un package `pkg/cbom` avec :
   - `BaseCBOM(kind string, scannedAt time.Time, components []map[string]any, metadataExtra map[string]any) map[string]any`.
3. Sans changer les handlers existants : faire implémenter `ScanResult` aux DTOs (ou adaptateurs) wallet et TLS, et faire en sorte que `ToCBOM()` utilise `cbom.BaseCBOM` + les composants spécifiques. Objectif : centraliser la construction CBOM et supprimer la duplication entre “list/detail” et “getCBOM” pour wallet et TLS.

### Phase 2 – Plugin TLS et Wallet

4. Définir un `Plugin` pour wallet et un pour TLS (descriptor + validate + runner qui délègue à `DiscoveryService.ScanWallet` / `TLSService.ScanTLS`).
5. Enregistrer les deux plugins au démarrage (ex. dans `cmd/api/main.go` ou `internal/app/container.go`).
6. Introduire `UsageCounter` et faire implémenter par `ScanResultRepository` et `TLSScanResultRepository` ; adapter `PlanService.CheckScanLimit` pour accepter une map `map[string]UsageCounter` et un plan avec champs optionnels (pour l’instant seulement wallet + endpoint).

### Phase 3 – Handler et worker génériques

7. Introduire un handler générique `Scan(c)` qui lit le kind (body ou path), récupère le plugin, valide, vérifie limites, publie sur le subject du plugin. Garder les routes actuelles en délégation vers ce handler (ou migrer progressivement).
8. Créer un worker générique qui, par subject, associe un plugin et exécute `Runner.Run` après unmarshal. Soit un worker par subject (comme aujourd’hui) qui appelle le plugin enregistré pour ce subject, soit un seul worker multi-subject. Migrer TLS et wallet vers ce mécanisme.
9. Unifier `GetCBOM` : résolution du kind depuis le paramètre, récupération du résultat via le plugin (ou service existant), puis `result.ToCBOM()`.

### Phase 4 – Scan de fichiers

10. Définir le modèle de données et l’API d’upload de fichier (si pas déjà fait).
11. Implémenter le plugin file : descriptor, validation, runner (analyse de fichier), `FileScanResult` avec `ToCBOM()`, repo + `UsageCounter`.
12. Ajouter `FileScanLimit` (et sujet NATS) dans la config des plans et dans le registre.
13. Brancher le handler générique et le worker pour le kind `file`.

---

## 8. Résumé

- **Scans en plugins** : chaque type (wallet, TLS, file) est un plugin avec descriptor, validation, runner, et résultat exposé en CBOM via l’interface `ScanResult`.
- **Interfaces communes** : `ScanResult` (ScanKind, ScannedAt, ToCBOM), `Plugin`, `Runner`, `UsageCounter` pour les limites, et structure CBOM commune via `cbom.BaseCBOM`.
- **Éviter la duplication** : un seul flux “scan” (handler + worker) basé sur le registre, une seule construction CBOM via `ToCBOM()`, et limites de plan extensibles via une map de counters.
- **Plan d’action** : 4 phases (interfaces + CBOM → enregistrement TLS/wallet → handler/worker génériques → plugin file) pour avancer sans tout casser et en gardant la rétrocompatibilité.

Si tu valides cette direction, la prochaine étape peut être de détailler les signatures exactes dans le code (noms de packages, emplacement du registre, compatibilité avec les routes et NATS actuels) puis d’appliquer la Phase 1 fichier par fichier.
