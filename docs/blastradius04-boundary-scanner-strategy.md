# CAFE repository boundaries and scanner strategy

## Purpose

This document describes the intended repository split for CAFE, the public/private boundary, and the execution model for scanners.

The goal is to make CAFE publicly understandable and technically credible, while preserving the private parts that currently represent the strongest implementation and business differentiation.

The proposed structure separates:

- discovery orchestration
- scanner implementations
- blast radius analytics
- policy evaluation

This split also supports gradual open-sourcing.

## High-level repository model

The proposed repository structure is:

- cafe-discovery
- cafe-scanners
- cafe-blastradius
- cafe-policy

The intended responsibility of each repository is different.

## cafe-discovery

`cafe-discovery` is the entrypoint and orchestration layer for Discovery.

Its main responsibilities are:

- expose the Discovery API
- receive scan requests
- normalize input targets
- create and track scan jobs
- route scan jobs to the correct scanner workers
- orchestrate execution across scanner containers
- collect scanner outputs
- publish canonical facts downstream
- manage job lifecycle events
- provide operator-visible status and control surfaces

In this model, `cafe-discovery` is not the full analytics engine.
It is the orchestration and integration layer for Discovery.

Typical responsibilities include:

- API server
- request validation
- scan job scheduling
- NATS subjects and message contracts
- scanner routing
- job persistence
- status transitions
- retries / failure handling
- integration with downstream fact ingestion

This repository is a strong candidate for public open source release because it shows:

- the CAFE architecture
- the plugin and scanner model
- the execution flow
- the integration points
- the API and event contracts

without necessarily exposing all private detection logic or analytics logic.

## cafe-scanners

`cafe-scanners` contains the scanner implementations.

Each scanner is an execution unit dedicated to one scanning concern.
Each scanner runs in its own container, as in the current architecture.

This means the execution model remains:

- one scanner implementation
- one scanner container
- one scanner worker process
- one isolated runtime per scanner

Examples of scanners include:

- wallet scanner
- TLS scanner
- ERC-20 scanner
- ERC-721 scanner
- ERC-1155 scanner
- Safe scanner
- ERC-4337 account scanner
- ERC-4337 paymaster scanner
- ERC-4337 factory scanner
- EIP-7702 scanner
- delegation scanner
- bridge scanner
- governance scanner

The scanner repository should provide the implementation layer, not the orchestration layer.

Each scanner should:

- consume normalized scan requests
- execute scanner-specific logic
- collect evidence
- produce canonical facts directly
- return confidence and metadata
- emit deterministic structured outputs
- include fixtures and normalization tests

The target design is that scanners write facts directly.

That means the long-term output of a scanner is not just a scanner-specific blob, but structured canonical facts with evidence and confidence.

This repository may remain private initially if scanner heuristics and detection logic are considered sensitive.
A later hybrid model is possible, where some demo scanners are public and the more strategic scanners remain private.

## cafe-blastradius

`cafe-blastradius` contains the analytics layer built on top of scanner outputs.

Its responsibilities are:

- ingest canonical facts
- deduplicate and store facts
- resolve entities
- derive graph nodes and edges
- materialize graph views
- compute traversals
- score exposure and impact
- produce blast radius summaries

This repository is not a scanner repository.
It is the analysis engine.

The blast radius engine should be treated as a derived analytics layer on top of:

- scanner outputs
- canonical facts
- graph materialization

It should not be modeled as “just another scanner”.

This repository is likely one of the most strategically sensitive parts of CAFE because it contains:

- graph logic
- relationship derivation
- impact propagation rules
- exposure scoring
- blast radius summarization

For that reason, it is a strong candidate to remain private initially.

## cafe-policy

`cafe-policy` contains the policy layer.

Its responsibilities are:

- define policy models
- evaluate facts and blast radius outputs against policy rules
- map technical findings to risk states
- produce remediation guidance
- support crypto-agility migration logic
- align findings with governance or compliance overlays

This repository should remain separate from both Discovery and Blast Radius because it solves a different problem.

A useful way to think about the full system is:

- Discovery finds and normalizes evidence
- Blast Radius computes exposure and impact
- Policy determines what should happen next

`cafe-policy` is therefore the decision and action layer, not the collection layer and not the analytics layer.

## Public and private boundary

A practical initial publication strategy is:

Public:
- cafe-discovery

Private:
- cafe-scanners
- cafe-blastradius
- cafe-policy

This split allows CAFE to publish the overall architecture, interfaces, and orchestration model while keeping the most sensitive logic private.

What becomes public through `cafe-discovery`:

- API structure
- scan job lifecycle
- plugin model
- scanner integration contract
- event flow
- orchestration design
- integration examples
- documentation for adding scanners

What remains private initially:

- scanner heuristics
- scanner-specific detection logic
- high-value fixtures
- canonical fact production details where strategically sensitive
- graph derivation logic
- blast radius scoring
- policy evaluation logic
- remediation decision logic

## Why this split is coherent

This repository split makes it possible to:

- open source the framework without exposing all implementation details
- preserve business differentiation
- keep the architecture understandable
- allow external contributors to understand the orchestration layer
- maintain a clean separation between collection, analytics, and policy

This is especially useful while the platform is still in alpha and schemas or boundaries may still evolve.

## Important execution model

Each scanner runs in its own container.

This is a core architectural property and should be preserved.

The container-per-scanner model provides:

- isolation
- independent deployment
- operational clarity
- clear ownership boundaries
- independent scaling
- independent failure domains

The public orchestration layer should therefore be designed to work with external scanner workers as independent services or containers.

This is already aligned with the current architecture and should remain the target model.

## Interface between discovery and scanners

The boundary between `cafe-discovery` and `cafe-scanners` must be explicit and stable.

The scanner contract should define:

- normalized input target
- scanner identity
- scanner kind
- evidence payload shape
- fact output shape
- confidence fields
- error handling
- execution metadata

This is important because scanners are separate runtime units and may remain private while the orchestration layer is public.

The goal is that `cafe-discovery` can orchestrate scanners without embedding their logic.

## Interface between discovery and blast radius

The boundary between `cafe-discovery` and `cafe-blastradius` should also be explicit.

The recommended target architecture is:

- scanners emit canonical facts directly
- `cafe-discovery` orchestrates and transports those facts
- `cafe-blastradius` ingests and analyzes those facts

This keeps responsibilities clear:

- Discovery orchestrates and transports
- Blast Radius analyzes and computes

## What should exist in public `cafe-discovery`

To make `cafe-discovery` credible as a public repository, it should include enough material to be understandable and runnable, even if production scanners remain private.

Recommended public contents include:

- API definitions
- request/response examples
- event schema definitions
- scan job lifecycle model
- plugin interfaces
- scanner integration docs
- local development instructions
- demo or mock scanners
- example end-to-end execution path
- container orchestration examples for scanner workers

Without at least some runnable demonstration path, a public orchestration repository may look too abstract.

## Demo scanner strategy

If private scanners are not published, `cafe-discovery` should still expose a minimal demonstration model.

Recommended options include:

- mock scanners
- demo scanners with intentionally limited detection logic
- static example scanner plugins
- fake scanner workers used only for documentation and local integration tests

This ensures the public repository is educational and credible without exposing strategic implementations.

## Repository responsibilities summary

### cafe-discovery
Responsibilities:
- public orchestration and integration layer
- API and scan job management
- scanner routing
- execution lifecycle
- event contracts
- fact transport

### cafe-scanners
Responsibilities:
- scanner implementations
- one scanner per container
- evidence collection
- canonical fact generation
- fixtures and normalization tests

### cafe-blastradius
Responsibilities:
- fact ingestion
- entity resolution
- graph construction
- traversal and scoring
- blast radius summaries

### cafe-policy
Responsibilities:
- policy model
- risk evaluation
- remediation logic
- migration guidance
- governance and compliance overlays

## Recommended initial publication strategy

Phase 1:
- publish `cafe-discovery`
- keep production scanners private
- keep `cafe-blastradius` private
- keep `cafe-policy` private

Phase 2:
- optionally publish selected non-sensitive demo scanners
- keep strategic scanners private
- continue stabilizing fact contracts

Phase 3:
- evaluate whether some scanner families can be open-sourced without exposing critical differentiation

## Final recommendation

The recommended structure is:

- `cafe-discovery` as the public orchestration and API layer
- `cafe-scanners` as the scanner implementation repository, with one scanner per container
- `cafe-blastradius` as the private analytics engine
- `cafe-policy` as the private policy and remediation layer

This split is technically clean, aligned with the current execution model, and well suited to gradual public opening while preserving the most sensitive parts of the platform.






# Cursor prompt to create the cafe-scanners repository

Please create a new repository named `cafe-scanners`.

Goal:
This repository will contain scanner implementations for CAFE.
It is not the orchestration backend.
It is not the blast radius engine.
It is not the policy layer.

It is the scanner implementation layer only.

Important architectural constraint:
Each scanner must run in its own container, as in the current architecture.
Please preserve that model explicitly in the repository structure, examples, and documentation.

## Context

The broader CAFE architecture is split into several repositories:

- `cafe-discovery`: public API and orchestration layer
- `cafe-scanners`: scanner implementations
- `cafe-blastradius`: facts, graph, blast radius analytics engine
- `cafe-policy`: policy evaluation and remediation logic

This new repository is only for scanners.

The target architecture is:
- scanners consume normalized scan requests
- scanners execute detection logic
- scanners collect evidence
- scanners emit canonical facts directly
- downstream systems derive graph and blast radius from those facts

Do not design this repository around raw opaque scanner-specific outputs as the long-term model.
The target model is direct canonical fact production.

## Initial scope

Please create the initial repository structure for:

- wallet scanner
- TLS scanner

These are the first two scanners to include.

They should be implemented as separate scanner units, with separate runtime/container boundaries.

## Repository requirements

Please design the repository so that it is easy to add more scanners later, such as:

- ERC-20 scanner
- ERC-721 scanner
- ERC-1155 scanner
- Safe scanner
- ERC-4337 account scanner
- ERC-4337 paymaster scanner
- ERC-4337 factory scanner
- EIP-7702 scanner
- delegation scanner
- governance scanner
- bridge scanner

The repository should support scanner growth without becoming monolithic.

## Required design principles

### 1. One scanner, one container
Make this explicit in both code layout and documentation.

Each scanner should have:
- its own implementation package or module
- its own executable entrypoint
- its own Dockerfile or container build target
- isolated runtime concerns

A future deployment should be able to run scanners independently.

### 2. Shared scanner contract
Create a shared package for common scanner contracts and shared types.

This should include things such as:
- normalized scan target
- scanner descriptor
- canonical fact output types
- evidence structure
- confidence structure
- execution metadata
- error model

The shared contract should be stable and scanner-agnostic.

### 3. Canonical facts as primary output
Please model scanner output around canonical facts directly.

Each scanner should produce structured outputs such as:
- descriptive facts
- relationship facts
- evidence
- confidence
- metadata

Do not make the primary design centered on scanner-specific raw blobs.
If raw details are needed for debugging, they should be secondary.

### 4. Testability is mandatory
Every scanner delivered must include:
- fixtures
- deterministic unit tests
- normalization tests for canonical outputs
- evidence/confidence validation where relevant

Please bake this into the repository structure from the start.

A scanner is not considered complete unless it ships with its fixtures and normalization tests.

### 5. Clear separation between shared code and scanner-specific code
Please avoid mixing generic scanner framework code with scanner-specific heuristics.

Create a structure that clearly separates:
- shared scanner SDK / contracts
- wallet scanner code
- TLS scanner code
- test fixtures
- local dev tooling
- container definitions

### 6. Repo should support private operation
This repository may remain private initially.
Please structure it professionally as an internal product-quality repository, not as a toy example.

## Suggested repository contents

Please create a repo skeleton with something close to the following intent:

- root README
- architecture note
- scanner contract documentation
- shared package for scanner contracts and fact models
- scanner implementations:
  - wallet
  - tls
- fixtures per scanner
- tests per scanner
- container definitions per scanner
- local development compose or equivalent
- examples showing independent scanner execution

The exact file layout is up to you, but it should reflect the one-container-per-scanner model clearly.

## Scanner expectations

### Wallet scanner
The wallet scanner is the evolution of the current wallet scanner and should conceptually become an EOA-oriented scanner.

At minimum, the design should anticipate outputs such as:
- object classification facts
- network presence facts
- algorithm or cryptographic facts where applicable
- relationship facts when relevant later

Do not over-implement advanced heuristics yet if not needed for the initial skeleton, but structure it so that those can be added cleanly.

### TLS scanner
The TLS scanner should fit the same canonical scanner contract.

At minimum, the design should anticipate outputs such as:
- endpoint or certificate-related facts
- algorithm facts
- protocol facts
- evidence and confidence fields

Again, structure matters more than feature completeness at this step.

## Documentation requirements

Please write documentation in English.

Include:
- what this repository is
- what it is not
- how scanners fit into the larger CAFE architecture
- why each scanner runs in its own container
- how to add a new scanner
- what “canonical facts” mean in this repository
- testing expectations for every scanner

## Non-goals

Do not implement:
- blast radius graph logic
- policy engine logic
- orchestration backend API
- graph traversal
- scoring
- remediation logic

Those belong in other repositories.

## Deliverable

Please generate a repository proposal or initial repository scaffold for `cafe-scanners` that includes:

- a clear directory structure
- shared scanner contracts
- wallet scanner skeleton
- TLS scanner skeleton
- per-scanner test and fixture structure
- per-scanner containerization approach
- local dev run strategy
- concise repository documentation

Please optimize for:
- modularity
- long-term maintainability
- clear repo boundaries
- one scanner per container
- direct canonical fact output
- easy future addition of new scanners






QUESTION

Dans cafe-discovery, on a actuellement deux scanners: TLS scanner et Wallet scanner

j'aimerai qu'on les sorte du Discovery et qu'ils soient migres vers leur propre repository "cafe-scanners"

pour le moment, 
(1) on garde la même logique, on ne change pas leur implémentation
- consommation de messages NATS
- publication de messages NATS
- aucun accès aux DB


(2) on garde le même principe de deploiement: 
- chaque scanner a son image Docker
- car chaque scanner tourne dans son container Docker

(3) on garde les mêmes github actions

(4) par contre, actuellement le Wallet scanner utilise la même image de base "oleglod/cafe-crypto-backend:build-oqs" qui contient la librarie OQS ; or le Wallet scanner n'en a pas besoin

Il faut analyser les codes pour comprendre quelles sont les parties de codes en commun et comment factoriser ce qui est factorisable; et separer les deux scanners afin que l'image Docker du Wallet scanner ne soit pas basée sur l'image "oleglod/cafe-crypto-backend:build-oqs", mais sur une image plus petite, (alpine, par exemple)

ne fait aucune modification, fait l'analyse et ecrit un rapport de cette analyse, ainsi qu'un plan d'action; dans un premier temps, on gardera TLS et Wallet scanners en double avec les versions actuelles dans cafe-discvoery et les nouvelles version dans cafe-scanners


REPONSE


1. Contexte et objectif
Vous souhaitez extraire les scanners TLS et Wallet de cafe-discovery vers un dépôt cafe-scanners, en conservant la même logique NATS et le même modèle de déploiement (une image Docker par scanner), puis en permettant au Wallet de ne plus dépendre de l’image oleglod/cafe-crypto-backend:build-oqs / runtime-oqs. En première phase, le **code** et les **pipelines** coexistent dans deux dépôts pour permettre validation et rollback ; en **runtime**, un seul jeu de scanners est actif à la fois sur une file donnée (déploiement alternatif exclusif — voir section 6).

2. État actuel dans cafe-discovery
2.1 Point d’entrée et sélection du mode
Binaire unique : cmd/scanner/main.go lit DISCOVERY_SCANNER_TYPE (tls, wallet, all ou vide = les deux) et enregistre les runners appropriés.
main.go importe en dur les deux packages tlsrunner et walletrunner (lignes 12–13, 91–96), même si au runtime un seul type est actif.
2.2 NATS (consommation / publication)
Comportement aligné avec vos contraintes (pas d’accès DB côté scanner ; la persistance vient des événements) :

Rôle	Sujets / files
Abonnement (queue)	scan.requested.tls / scan.requested.wallet via QueueSubscribe(..., QueueScanners) (cafe.scanners)
Présence / heartbeat	cafe.discovery.scanners.presence, scanner.heartbeat.tls, scanner.heartbeat.wallet
Cycle de scan	scan.started, scan.completed, scan.failed
Les structs sont dans pkg/nats/messages.go ; les constantes dans pkg/nats/nats.go.

2.3 Santé HTTP
internal/scanner/core/core.go : serveur Fiber sur SCANNER_HEALTH_PORT (défaut 8081), endpoint /health (NATS + état des scanners).
2.4 Docker et GitHub Actions
TLS : Dockerfile-discovery-scanner-tls — build sur oleglod/cafe-crypto-backend:build-oqs, runtime runtime-oqs, CGO_ENABLED=1, flags liboqs/OpenSSL, DISCOVERY_SCANNER_TYPE=tls.
Wallet : Dockerfile-discovery-scanner-wallet — même pipeline (commentaire explicite : partage des deps TLS/PQC), seule la variable d’environnement finale diffère (DISCOVERY_SCANNER_TYPE=wallet).
Workflows concernés :

.github/workflows/docker-rc.yml : build multi-arch + push oleglod/cafe-discovery-scanner-tls et ...-scanner-wallet (tags sha-*, optionnel rc).
.github/workflows/docker-release.yml : promotion des mêmes images vers version / latest.
.github/workflows/ci.yml : cible --target ci des deux Dockerfiles (lint + tests + govulncheck dans l’image builder OQS).
3. Cause racine : pourquoi le Wallet “embarque” OQS aujourd’hui
Ce n’est pas parce que le code métier du wallet utilise liboqs.

Un seul artefact Go : les deux Dockerfiles font go build -o scanner ./cmd/scanner/main.go, c’est-à-dire le même graphe d’imports pour TLS et Wallet.
main importe toujours tlsrunner et walletrunner, donc le compilateur inclut la chaîne TLS complète, y compris :
internal/scanner/tlsrunner → plugin TLS → internal/service/tls.go → pkg/tls/...
pkg/tls/pqc_scanner.go appelle native.GetPQCInfo
native est du CGO (tls_pqc_wrapper.h, OpenSSL via pkg-config).
Les Dockerfiles fixent CGO_ENABLED=1 et l’image build-oqs pour que cette liaison C fonctionne ; le runtime runtime-oqs fournit les bibliothèques nécessaires au binaire lié.
Conséquence : même avec DISCOVERY_SCANNER_TYPE=wallet au runtime, l’image Wallet transporte un binaire gras (TLS + PQC/CGO) et une image de base lourde — c’est cohérent avec le build actuel, mais redondant pour le wallet.

4. Code partagé vs spécifique (factorisation)
4.1 Fortement partageable (générique “scanner NATS”)
internal/scanner/base_scanner.go, internal/scanner/helper.go (ProcessWithConcurrency)
internal/scanner/core (Setup NATS + config chaîne, Run + health Fiber)
Enveloppe des handlers : internal/scanner/tls_scanner.go et wallet_scanner.go (structure quasi miroir : unmarshal → scan.started → plugin → scan.completed / scan.failed)
4.2 Contrat stable avec le reste de la plateforme
pkg/nats (sujets, messages, PublishJSON, Connection)
pkg/scan (interface Plugin, descripteurs, KindTLS / KindWallet, cibles)
Éventuellement un sous-ensemble de internal/config (NATS, ports santé, versions plugins, Moralis, clés chain) — à extraire ou dupliquer de façon maîtrisée pour ne pas tirer tout le backend.
4.3 Spécifique TLS (doit rester avec CGO / OQS)
internal/scanner/tlsrunner, internal/scan/tls, internal/service/tls.go (dont règles PQC côté “métier”), pkg/tls/*, répertoire native/ (wrapper C), éventuellement binaires/outils OpenSSL dans l’image.
4.4 Spécifique Wallet (pas besoin d’OQS)
internal/scanner/walletrunner, internal/scan/wallet, internal/service/discovery.go, pkg/evm, pkg/moralis, internal/domain (résultats), internal/metrics (si conservés tels quels)
Config : LoadChainConfig + clés Moralis (internal/config + YAML monté comme aujourd’hui)

4.5 Suppression du couplage « API + scanner » sur DiscoveryService (objectif explicite)
Aujourd’hui `DiscoveryService` concentre à la fois l’exécution du scan wallet (RPC, Moralis, agrégation) et les responsabilités API (persist, limites de plan, listes) via `scanResultRepo` et `planService`, contournées par des `nil` et `SkipPersist` côté scanner. Ce modèle est fragile pour l’extraction vers `cafe-scanners` : on veut un type qui ne compile et n’expose **aucun** chemin vers DB/plan.

Piste de refactor (à faire dans `cafe-discovery` et/ou en premier dans `cafe-scanners`, puis rétroporter si besoin) :
- Extraire la logique pure « exécuter un scan wallet et produire un `domain.ScanResult` » dans un composant dédié (ex. package `internal/walletscan` ou `WalletScanEngine`) qui ne dépend ni de `repository`, ni de `PlanService` — uniquement clients RPC, Moralis, métriques, domain.
- Le `DiscoveryService` côté backend API compose ce composant + repo + plan pour les handlers HTTP ; le runner NATS n’importe **que** le composant pur (ou une interface étroite `WalletScanner`).

Effet : plus de `nil` « magiques » pour désactiver la persistance ; le scanner et le module extrait ne portent pas les types repository/plan dans leur graphe de dépendances.

4.6 Suppression du couplage « API + scanner » sur TLSService (objectif explicite)
Aujourd’hui `TLSService` (`internal/service/tls.go`) mélange dans un même type : (1) l’**exécution** du scan TLS / PQC (`pkg/tls` : `Scanner`, `PQCRules`, agrégation vers `domain.TLSScanResult`, scores, recommandations — fichier volumineux) et (2) les responsabilités **API** : persistance via `tlsScanResultRepo`, limites de plan via `planService`. Le chemin scanner NATS utilise `NewTLSService(nil, nil)` et `ScanTLS(..., skipPersist=true)` — le même contournement par `nil` et flag que sur le wallet.

Ce modèle est fragile pour `cafe-scanners` : le plugin TLS (`internal/scan/tls/plugin.go`) importe `*service.TLSService` et entraîne conceptuellement tout le périmètre « application », alors que le runner n’a besoin que de la chaîne **scan → résultat domaine**.

**Spécificité TLS (à ne pas confondre avec le Wallet)** : la dépendance **CGO / liboqs / `native`** pour la détection PQC reste **requise** pour le binaire et l’image du scanner TLS ; ce refactor ne vise pas à l’alléger (contrairement au Wallet Alpine). Il vise uniquement à **retirer repository et PlanService** du graphe d’imports du runner/plugin.

Piste de refactor (dans `cafe-discovery` en premier, puis copie vers `cafe-scanners`) :
- Extraire la logique pure « exécuter un scan TLS et produire un `*domain.TLSScanResult` » dans un composant dédié (ex. package `internal/tlsscan` ou `TLSScanEngine`) qui dépend de `pkg/tls`, `internal/domain`, métriques éventuelles, mais **ni** de `repository.TLSScanResultRepository`, **ni** de `PlanService`.
- Exposer une API du type `Execute(ctx, userID *uuid.UUID, targetURL string, isDefault bool) (*domain.TLSScanResult, error)` : **pas** de `skipPersist` — le moteur ne persiste jamais.
- Conserver sur une **façade** (le `TLSService` actuel ou un nom explicite type service applicatif TLS) la composition : `checkPlanLimitForScan` → `engine.Execute` → `persistTLSScanResult`, ainsi que `GetTLSScanByURL` / `ListTLSScanResults` qui restent indissociables du repo.
- Côté NATS : `tlsrunner` et le plugin TLS ne reçoivent **que** le moteur (ou une interface étroite), pas la façade avec repo/plan.

Effet : disparition des `nil` magiques pour le scanner TLS ; le graphe du runner TLS ne remonte plus vers `internal/repository` ni `PlanService`. Les payloads NATS (`scan.started` / `scan.completed` / `scan.failed`) restent inchangés si le comportement du moteur est identique à `ScanTLS` hors persist/plan.

5. Piste technique pour un Wallet “Alpine / léger”
Pour une image Wallet plus petite sans liboqs :

Découpler les binaires (recommandé) :
cmd/scanner-wallet/main.go n’importe que walletrunner (+ core),
cmd/scanner-tls/main.go n’importe que tlsrunner.
Build Wallet avec CGO_ENABLED=0 si toute la chaîne wallet est pure Go (très probable pour RPC Moralis + go-ethereum dans ce usage) ; valider avec go test / exécution réelle.
Image runtime : Alpine ou distroless + ca-certificates, sans runtime-oqs.
Le scanner TLS conserve le pipeline actuel (build-oqs + runtime-oqs + CGO).

6. Mode de déploiement pendant la transition : alternatif exclusif (pas de concurrence NATS)
La coexistence des deux implémentations dans le temps sert la **validation** et le **rollback**, pas le traitement parallèle.

Règle : sur un environnement donné, on déploie **soit** les scanners embarqués dans `cafe-discovery` **soit** ceux de `cafe-scanners`, jamais les deux en même temps sur les **mêmes sujets** et le **même queue group** (`cafe.scanners`). Ainsi aucune course ni double consommation : à tout instant, un seul producteur d’événements `scan.*` pour une file donnée.

Comparer fonctionnellement deux versions = basculer d’un déploiement à l’autre (blue/green ou révision précédente), pas scaler deux chartes actives sur la même infra NATS.

6.1 Migration vers cafe-scanners (structure suggérée)
Un module Go cafe-scanners (module path à fixer), avec :
packages communs copiés/adaptés depuis cafe-discovery : scanner/core, scanner base, pkg/nats, pkg/scan, config minimale.
deux commandes distinctes pour éviter le graphe TLS dans le wallet.
Option long terme : extraire les types NATS dans un petit module cafe-discovery-contracts / pkg versionné consommé par backend et scanners — pas obligatoire pour la phase “double maintenance” si vous acceptez la duplication temporaire des structs/sujets (avec discipline pour les garder alignés).
7. CI/CD dans le nouveau dépôt
Reprendre les mêmes idées que docker-rc.yml / docker-release.yml / ci.yml mais :

Cibler uniquement les images cafe-scanners-tls et cafe-scanners-wallet (noms à définir).
CI : job séparé — builder wallet sur image non-OQS (ex. golang:alpine ou golang officiel) pour le wallet ; TLS sur image OQS ou builder dédié.
Secrets Docker Hub : mêmes patterns (DOCKERHUB_*).
8. Plan d’action (phases)
Phase	Actions
0 – Prérequis	Créer le repo cafe-scanners, README, go.mod, politique de versionnement alignée avec cafe-discovery.
1 – Copie fidèle + découplage wallet / TLS	Importer le code scanner + dépendances directes ; idéalement après (ou en même temps que) l’extraction du scan wallet pur (section 4.5) et du scan TLS pur (section 4.6), pour que `cafe-scanners` n’embarque ni `DiscoveryService` ni `TLSService` avec repo/plan. Faire passer les tests/lint sans changer le comportement observable NATS.
2 – Découpage binaire Wallet	Introduire cmd/scanner-wallet sans import TLS ; ajuster Dockerfile Wallet (Alpine + CGO_ENABLED=0 si validé). Laisser TLS inchangé.
3 – Images & CI	Deux Dockerfiles (ou un avec TARGET), workflows RC/release/CI calqués sur cafe-discovery, nouvelles coordonnées d’images.
4 – Validation par alternance	Déployer **exclusivement** l’une ou l’autre stack scanner (cafe-discovery vs cafe-scanners) sur les mêmes sujets/queue ; mesurer et comparer par bascule contrôlée, pas par exécution simultanée.
5 – Bascule & nettoyage	Mettre à jour cafe-deploy / charts pour pointer vers cafe-scanners en production ; retirer ou déprécier les builds scanner dans cafe-discovery une fois la bascule validée.

Garde-fou opérationnel : si par erreur deux déploiements actifs partageaient la même queue, on aurait une double consommation — la procédure de release doit donc **désactiver** l’ancien déploiement avant d’activer le nouveau (ou l’inverse pour rollback), conformément au mode alternatif exclusif.

9. Synthèse
Les scanners respectent déjà NATS-only pour la persistance ; le modèle une image par scanner et les workflows RC/release sont documentés et reproductibles.
Le Wallet réutilise l’image OQS à cause du build monolithique (cmd/scanner + imports TLS/CGO), pas à cause des besoins fonctionnels du wallet.
La factorisation naturelle est un noyau commun (NATS, plugin, base scanner, core) et deux binaires ; le Wallet peut viser Alpine après séparation des imports TLS/CGO.
Pendant la transition, le déploiement est **alternatif exclusif** : validation et rollback par remplacement, sans concurrence sur la même file NATS.
Le découpage du scan wallet pur (section 4.5) et du scan TLS pur (section 4.6) enlève la friction `nil` / flags `SkipPersist` / `skipPersist` sur `DiscoveryService` et `TLSService`, et clarifie la frontière `cafe-scanners` / backend API.

10. Suivi d’exécution (historique des actions)

Statut global (aujourd’hui) :
- **Fait** : cadrage d’architecture et stratégie de déploiement en mode alternatif exclusif (pas de double consommation NATS sur les mêmes sujets/queue groups).
- **Fait** : section 4.5 (Wallet) implémentée dans `cafe-discovery` : extraction d’un moteur wallet pur sans dépendance repository/plan côté scanner.
- **Fait** : section 4.6 (TLS) implémentée dans `cafe-discovery` : extraction d’un moteur TLS pur sans dépendance repository/plan côté scanner.
- **Hors périmètre immédiat** : optimisation d’image Wallet / dépendance OQS (traitée plus tard, explicitement hors sujet pour cette étape).

Historique récent :
- H1 — Analyse initiale TLS/Wallet, Dockerfiles et workflows : identification du couplage scanner ↔ services API.
- H2 — Clarification de la stratégie de transition : coexistence temporelle pour validation/rollback, mais activation exclusive par environnement.
- H3 — Ajout des sections de découplage 4.5 (Wallet) et 4.6 (TLS) dans ce document.
- H4 — Exécution du découplage Wallet (4.5) dans le code : scanner wallet indépendant de `DiscoveryService` côté chemin NATS.
- H5 — Plan d’action TLS validé.
- H6 — Exécution du découplage TLS (4.6) dans le code : scanner TLS indépendant de `TLSService` côté chemin NATS (nouveau package `internal/tlsscan`).
- H7 — Vérifications post-refactor : `go build ./...` et `go test ./...` OK ; diagnostics IDE sans erreur sur les fichiers modifiés.
- H8 — Nettoyage Persistence : migration de `Dockerfile-discovery-persistence` vers un build/runtime non-OQS (builder Go officiel, runtime Debian slim + CA certs) ; `go build ./cmd/persistence` validé.
- H9 — Nettoyage Wallet scanner : création de `cmd/scanner-wallet/main.go` (binaire dédié wallet) et migration de `Dockerfile-discovery-scanner-wallet` vers un build/runtime non-OQS.
- H10 — Gate dépendances wallet validée : `go list -deps ./cmd/scanner-wallet` sans `pkg/pqc`, `native`, `pkg/tls`, `internal/tlsscan`, `internal/scanner/tlsrunner`.
- H11 — Validation runtime NATS wallet : sur NATS isolé, publication `scan.requested.wallet` et réception confirmée `scan.started` puis `scan.completed` pour le même `scan_id`.
- H12 — Alignement workflows CI/RC/Release pour les migrations non-OQS wallet+persistence : suppression de l’hypothèse `CGO_ENABLED=1` globale dans RC/Release, et ajout d’un build persistence dans CI pour couvrir `Dockerfile-discovery-persistence`.
- H13 — Scanner TLS dédié : ajout de `cmd/scanner-tls/main.go` et migration de `Dockerfile-discovery-scanner-tls` vers ce binaire dédié (`scanner-tls`), ce qui supprime le couplage build TLS→wallet via `cmd/scanner`.
- H14 — Matrice de migration fichier → dépôt rédigée pour `cafe-scanner-wallet` et `cafe-scanner-tls` (section 12), prête pour exécution ultérieure.

Prochaine étape opérationnelle :
- Exécuter l’extraction vers **`cafe-scanner-wallet`** et **`cafe-scanner-tls`** en suivant la **matrice fichier → dépôt** (section 12) : copier/adaptation des imports (`go.mod`), duplication contrôlée du socle NATS, puis CI/CD et validation en déploiement alternatif exclusif.

11. Exit Criteria OQS (Discovery + Wallet non-OQS)

Objectif de sortie :
- `cmd/server` (Discovery) et scanner Wallet doivent être buildés/runnés sans base OQS.
- scanner TLS reste OQS (hors périmètre de cette sortie).

Checklist minimale par composant :

11.1 Discovery (`cmd/server`)
- Isoler la fonctionnalité PQC (auth/signature) pour que le binaire serveur ne tire plus `pkg/pqc` (et ses dépendances natives OQS).
- Vérifier le graphe : `go list -deps ./cmd/server` ne doit plus inclure `cafe-discovery/pkg/pqc`, `cafe-discovery/native` (et `pkg/tls` si non requis côté API). **Statut actuel** : non atteint (`pkg/pqc` et `native` encore présents).
- Introduire un Dockerfile Discovery non-OQS (builder/runtime standards) et migrer CI/RC/Release sur cette variante.
- Valider le comportement API (auth + endpoints discovery/tls) après migration.

11.2 Wallet scanner
- [x] Découper le binaire scanner pour wallet (`cmd/scanner-wallet/main.go`) afin d’éviter l’import du graphe TLS/OQS de `cmd/scanner/main.go`.
- [x] Mettre à jour le Dockerfile wallet pour builder ce binaire dédié (runtime non-OQS).
- [x] Passer `CGO_ENABLED=0` dans le Dockerfile wallet.
- [x] Valider la chaîne NATS inchangée : `scan.requested.wallet` -> `scan.started/completed/failed` (test d’intégration runtime).

11.3 Persistence service
- [x] Le code ne dépend pas d’OQS.
- [x] Dockerfile migré en non-OQS.
- [x] Aligner les workflows CI/RC/Release sur ce Dockerfile non-OQS.

Gates de validation (avant merge) :
- `go list -deps ./cmd/server` ne contient plus `pkg/pqc` ni `native`.
- `go list -deps ./cmd/scanner-wallet` ne contient plus `pkg/pqc`, `native`, `pkg/tls`, `internal/tlsscan` (validé).
- Dockerfiles Discovery/Wallet/Persistence n’utilisent plus `cafe-crypto-backend:*oqs`.
- CI verte (`go build`, `go test`, scans d’images) avec les nouveaux Dockerfiles.

12. Matrice de migration vers `cafe-scanner-wallet` et `cafe-scanner-tls`

Objectif : documenter **quel fichier** va **dans quel dépôt**, avec quelle **action** (copie, adaptation, exclusion), pour exécution ultérieure sans ambiguïté. Les deux dépôts sont des modules Go distincts (chemins de module à fixer, ex. `github.com/<org>/cafe-scanner-wallet`).

12.1 Règles transverses

| Règle | Détail |
|--------|--------|
| Module Go | Un `go.mod` par dépôt ; remplacer le préfixe d’import `cafe-discovery/...` par le module cible sur **tous** les fichiers migrés. |
| Contrats NATS | Sujets, queue group, forme des payloads : **inchangés** par rapport à `cafe-discovery` (alignement avec persistence-service / backend). |
| Duplication | Le socle `internal/scanner/*`, `pkg/nats`, `pkg/scan` est **dupliqué** dans chaque dépôt pour l’instant (pas de module `contracts` obligatoire à cette étape). |
| `pkg/scan` | Chaque dépôt peut ne garder que le `Kind` utile (`KindWallet` ou `KindTLS`) + descripteurs associés ; retirer l’autre kind du registre si on épure. |
| `internal/config` | Copier `chain.go` + les constantes / `GetDefaultConfigValues` **effectivement lues** par `cmd/scanner-*` et `pkg/nats` (NATS URL, ports santé, clés Moralis + versions plugin wallet ou TLS selon le binaire). Éviter d’embarquer tout le backend PostgreSQL si inutile au scanner seul. |
| `internal/metrics` | Copier `metrics.go` tel quel (simplest) **ou** scinder en `metrics_wallet.go` / `metrics_tls.go` dans chaque dépôt pour ne publier que les métriques du scanner concerné (refactor optionnel). |
| `internal/domain` | Les DTO JSON (`ScanResult`, `NetworkResult`, `TLSScanResult`, `CertificateInfo`, …) vivent aujourd’hui dans `internal/domain/models.go`. **Option A** : copier `models.go` en entier dans chaque dépôt (rapide, un peu redondant). **Option B** : extraire des sous-fichiers par périmètre (`domain/wallet.go`, `domain/tls.go`, `domain/enums.go`) lors de la migration. Ne **pas** migrer les entités GORM (`scan_result.go`, `tls_scan_result.go`) si le scanner ne persiste pas en base. |
| `native/` + CGO | Uniquement **`cafe-scanner-tls`** (avec `pkg/tls` + toolchain OQS comme aujourd’hui). **`cafe-scanner-wallet`** reste **`CGO_ENABLED=0`**. |
| Déploiement | Toujours **alternatif exclusif** avec les images `cafe-discovery` jusqu’à bascule validée (section 6). |

12.2 `cafe-scanner-wallet` — fichiers sources

| Chemin actuel (`cafe-discovery`) | Action | Notes |
|----------------------------------|--------|--------|
| `cmd/scanner-wallet/main.go` | Copier | Point d’entrée ; ajuster imports module. |
| `internal/scanner/walletrunner/runner.go` | Copier | Câblage NATS + `WalletScanEngine` + plugin. |
| `internal/scanner/wallet_scanner.go` | Copier | Consommateur `scan.requested.wallet` + concurrence. |
| `internal/scanner/base_scanner.go` | Copier | Partagé avec TLS aujourd’hui — même fichier dans le repo wallet. |
| `internal/scanner/helper.go` | Copier | `ProcessWithConcurrency`. |
| `internal/scanner/core/core.go`, `internal/scanner/core/deps.go` | Copier | `Setup` NATS + `LoadChainConfig` ; health Fiber. |
| `internal/scan/wallet/plugin.go` | Copier | Plugin NATS wallet ; métriques scanner. |
| `internal/walletscan/engine.go`, `internal/walletscan/impl.go` | Copier | Moteur pur wallet. |
| `pkg/nats/nats.go`, `pkg/nats/messages.go` | Copier | Renommer éventuellement `nats.Name("cafe-discovery")` → identifiant du nouveau module. |
| `pkg/scan/*.go` | Copier puis élaguer | Garder `KindWallet`, enregistrement plugin wallet ; tests `state_test.go` si présents. |
| `pkg/evm/client.go`, `pkg/evm/recover.go` | Copier | Clients RPC / récupération clé. |
| `pkg/moralis/moralis.go`, `pkg/moralis/types.go` | Copier | Index Moralis. |
| `internal/domain/models.go` | Copier (ou extraire) | DTO wallet + enums ; exclure besoin TLS si fichier scindé. |
| `internal/config/chain.go` | Copier | Blockchains YAML. |
| `internal/config/config.go` | Copier partiel ou intégral | Au minimum : clés utilisées par `main`, `core`, `walletrunner`, `pkg/nats`. |
| `internal/metrics/metrics.go` | Copier (ou sous-ensemble wallet) | Préfixes métriques `cafe_discovery_*` : option renommage `cafe_scanner_wallet_*` pour clarté ops. |
| `Dockerfile-discovery-scanner-wallet` | Copier → `Dockerfile` (ou `Dockerfile.scanner`) | Builder/runtime non-OQS, binaire `scanner-wallet`. |
| `.github/workflows/*` | Adapter depuis `cafe-discovery` | CI build/test `cmd/scanner-wallet`, build image, pas de cible OQS. |

**Exclusions explicites (ne pas mettre dans ce dépôt)** : `internal/service/discovery.go`, `internal/handler/*`, `internal/repository/*`, `cmd/server`, `cmd/persistence`, `internal/scan/tls/*`, `internal/tlsscan/*`, `internal/scanner/tlsrunner/*`, `pkg/tls/*`, `native/*`, `cmd/scanner/main.go` monolithique.

12.3 `cafe-scanner-tls` — fichiers sources

| Chemin actuel (`cafe-discovery`) | Action | Notes |
|----------------------------------|--------|--------|
| `cmd/scanner-tls/main.go` | Copier | Point d’entrée ; ajuster imports module. |
| `internal/scanner/tlsrunner/runner.go` | Copier | Câblage NATS + `TLSScanEngine` + plugin. |
| `internal/scanner/tls_scanner.go` | Copier | Consommateur `scan.requested.tls`. |
| `internal/scanner/base_scanner.go` | Copier | Identique à la copie wallet (duplication). |
| `internal/scanner/helper.go` | Copier | Idem. |
| `internal/scanner/core/core.go`, `internal/scanner/core/deps.go` | Copier | Idem wallet ; `LoadChainConfig` peut rester (YAML minimal même si peu utilisé par TLS). |
| `internal/scan/tls/plugin.go` | Copier | Plugin NATS TLS. |
| `internal/tlsscan/engine.go`, `internal/tlsscan/impl.go` | Copier | Moteur pur TLS/PQC. |
| `pkg/tls/scanner.go`, `pkg/tls/pqc_scanner.go`, `pkg/tls/pqc.go`, `pkg/tls/pqc_groups.go` | Copier | Chaîne TLS + PQC / CGO. |
| `native/*` (sources + `Makefile` / en-têtes utilisés par le build Go) | Copier | Aligner `Dockerfile` / `CGO_CFLAGS` / `LDFLAGS` sur `Dockerfile-discovery-scanner-tls` actuel. |
| `pkg/nats/*`, `pkg/scan/*` | Copier puis élaguer | Garder `KindTLS` + plugin TLS. |
| `internal/domain/models.go` | Copier (ou extraire) | DTO TLS + enums + `CertificateInfo` / `CipherSuiteInfo`. |
| `internal/config/chain.go`, `internal/config/config.go` | Comme wallet | Clés TLS plugin + NATS + santé. |
| `internal/metrics/metrics.go` | Copier (ou sous-ensemble TLS) | Idem renommage optionnel. |
| `Dockerfile-discovery-scanner-tls` | Copier → `Dockerfile` | Builder OQS + `scanner-tls`. |
| `.github/workflows/*` | Adapter | CI avec `CGO_ENABLED=1`, image build-oqs / runtime-oqs comme aujourd’hui. |
| `scripts/benchmark_tlsscan.sh` (si utilisé) | Optionnel | Outils perf TLS. |

**Exclusions explicites** : `internal/service/tls.go`, `internal/handler/tls.go`, `internal/repository/*tls*`, `internal/walletscan/*`, `internal/scan/wallet/*`, `internal/scanner/walletrunner/*`, `pkg/evm`, `pkg/moralis`, `cmd/scanner-wallet`.

12.4 Artefacts hors racine `internal/` / `pkg/`

| Artefact | Wallet | TLS |
|----------|--------|-----|
| `go.mod` / `go.sum` | Nouveau module ; reprendre dépendances de `go list -deps ./cmd/scanner-wallet` | Idem pour `./cmd/scanner-tls` (incl. CGO / openssl). |
| `README.md` | Build, config, variables d’env, image Docker | Idem + prérequis OQS. |
| `config.yaml` exemple | `blockchains`, Moralis, `scan.plugins.wallet.version`, NATS | `scan.plugins.tls.version`, NATS ; blockchains peuvent être minimales. |
| `cafe-deploy` | Mettre à jour chemins d’images / tags quand les images `cafe-scanner-*` existent ; scripts `verify-*-nats-chain.sh` pointent vers le binaire ou l’image de ce dépôt. | Idem. |

12.5 Vérifications post-copie (gates avant de retirer les scanners de `cafe-discovery`)

1. `go build ./...` et `go test ./...` dans chaque nouveau dépôt.  
2. `go list -deps ./cmd/scanner-wallet` sans `pkg/tls`, `native`, `tlsscan` (déjà la règle dans cafe-discovery).  
3. `go list -deps ./cmd/scanner-tls` inclut `pkg/tls` et `native` comme aujourd’hui.  
4. Tests NATS : réutiliser / adapter `cafe-deploy/scripts/verify-wallet-nats-chain.sh` et `verify-tls-nats-chain.sh`.  
5. Image Docker : build multi-arch aligné sur la stratégie actuelle (Docker Hub / GHCR).

12.6 Nettoyage ultérieur dans `cafe-discovery` (après bascule validée — pas au premier commit des nouveaux repos)

- Retirer ou réduire les Dockerfiles / workflows **scanner** du dépôt discovery si une seule source d’images subsiste.  
- Documenter la version de référence des payloads NATS si duplication des structs `domain` entre repos.




## [x] Phase A — Préparation (dans cafe-discovery)

- Geler les contrats NATS utilisés par scanners (subjects + messages).
- Vérifier qu’aucune dépendance API/DB/plan ne remonte dans les chemins scanner.
- Écrire une checklist d’extraction (fichiers obligatoires par scanner).


## [x] Phase B — Créer cafe-scanner-wallet

- Initialiser repo + go.mod.
- Copier les packages wallet nécessaires (liste ci-dessus) avec imports adaptés.
- Ajouter Dockerfile wallet non-OQS.
- Reproduire workflows ci, docker-rc, docker-release.
- Vérifier:
  - go list -deps ./cmd/scanner-wallet sans pkg/pqc/native/pkg/tls.
  - test NATS chain wallet (script déjà prêt).
- **Statut** : fait — dépôt `cafe-scanner-wallet`, image `oleglod/cafe-scanner-wallet`, variable `SCANNER_WALLET_VERSION` et compose `cafe-deploy` alignés ; scanner wallet retiré du runtime `cafe-discovery`.

## [x] Phase C — Créer cafe-scanner-tls

- Initialiser repo + go.mod.
- Copier packages TLS nécessaires.
- Ajouter Dockerfile TLS OQS.
- Reproduire workflows ci, docker-rc, docker-release.
- Vérifier:
  - build TLS + test NATS chain TLS.
  - cohérence native/OQS dans image runtime.
- **Statut** : fait — dépôt `cafe-scanner-tls` (TLS-only) avec `go.mod`, `Dockerfile` OQS, workflows `CI` + `docker-rc` + `docker-release` et `README` complété (run/build, OQS/CGO, stratégie de tags).

## [x] Phase D — Intégration déploiement (cafe-deploy)

- Ajouter nouvelles images/version vars:
  - SCANNER_WALLET_VERSION
  - SCANNER_TLS_VERSION
- Mettre à jour compose/templates pour pointer vers les nouveaux repos d’images.
- Garder switch simple pour rollback vers images historiques cafe-discovery-scanner-*.
- **Statut** : fait — `SCANNER_WALLET_VERSION` déjà aligné (`cafe-scanner-wallet`) ; ajout de `SCANNER_TLS_VERSION` et bascule du service TLS dans `cafe-deploy/compose/20-discovery.yml` vers `oleglod/cafe-scanner-tls`, avec adaptation des scripts (ex: `verify-tls-nats-chain.sh`) et doc associée.

## [x] Phase E — Validation en environnement

- Déployer wallet seul depuis nouveau repo (TLS reste ancien), puis inversement.
- Vérifier événements NATS, health, perf, stabilité.
- Puis déployer les deux scanners depuis leurs repos dédiés.
- **Statut** : fait — reconstruction locale complète + redeploy local validé ; fonctionnement confirmé avec `oleglod/cafe-scanner-tls`.

## [x] Phase F — Décommission dans cafe-discovery

- Retirer complètement les artefacts TLS de `cafe-discovery` (après bascule validée via Phase D/E):
  - `cmd/scanner-tls/main.go` (entrypoint TLS)
  - `Dockerfile-discovery-scanner-tls`
  - Service compose `cafe-discovery-scanner-tls` (si présent dans `docker-compose*.yml` et templates)
  - `internal/scanner/tlsrunner/*`
  - `internal/scanner/tls_scanner.go`
  - `internal/scan/tls/*`
  - `internal/tlsscan/*`
  - `pkg/tls/*`
  - `native/*` (code/headers utilisés uniquement par le build TLS)
  - CI/CD dans `.github/workflows/*` qui build/push `cafe-discovery-scanner-tls`
- Conserver docs de compatibilité/rollback.
- **Statut** : fait — suppression des artefacts TLS worker/build dans `cafe-discovery` (entrypoint/scanner TLS, Dockerfile TLS, dépendances `pkg/tls` + `native`, compose local TLS, et workflows RC/Release/CI liés à l’image TLS).

### Clarification architecture (post Phase F)

- **Backend (`cafe-discovery`)** :
  - valide les entrées (wallet / endpoint),
  - gère l’authentification et l’autorisation (plan/quotas),
  - publie les demandes de scan sur NATS (`scan.requested.*`).
- **Scanners dédiés** (`cafe-scanner-wallet`, `cafe-scanner-tls`) :
  - exécutent les scans,
  - publient les événements de cycle de vie (`scan.started`, `scan.completed`, `scan.failed`).
- **Persistence service (`cafe-discovery-persistence`)** :
  - consomme les événements NATS,
  - écrit les résultats en stockage (PostgreSQL + Redis).

Note : le découplage TLS est complet dans `cafe-discovery`. Le chemin wallet côté code API garde encore des éléments de service métier locaux ; une harmonisation totale backend-only (validation/plan + publish NATS) pour wallet peut être traitée dans une étape ultérieure.

## Critères de réussite (DoD)

- Chaque repo scanner build/test/publish indépendamment.
- Aucun import scanner vers services API DB/plan.
- Wallet scanner non-OQS, TLS scanner OQS.
- Déploiement et rollback pilotables indépendamment par scanner.
- Chaînes NATS inchangées et validées en runtime.

## Risques principaux + mitigation

- Dérive des contrats NATS → versionner/figer un package contrat commun.
- Duplication de code commun → extraire plus tard un petit module partagé (scanner-core-contracts) si utile.
- Régression CI/CD → copier les workflows actuels puis adapter minimalement.
- Rollback complexe → garder variables d’image séparées et documentées dans cafe-deploy.

