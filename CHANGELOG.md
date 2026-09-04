# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0-alpha](https://github.com/create2-labs/cafe-discovery/compare/v0.1.1-alpha...v0.2.0-alpha) (2026-09-04)


### Features

* add OpenAPI spec for Discovery API v1 ([#49](https://github.com/create2-labs/cafe-discovery/issues/49)) ([8c4d58b](https://github.com/create2-labs/cafe-discovery/commit/8c4d58b0c7c17fc7f2cd3143962bfd6ddf4fc3f3))
* add scan_usage_events ledger for success-only plan quota ([#83](https://github.com/create2-labs/cafe-discovery/issues/83)) ([5ff462d](https://github.com/create2-labs/cafe-discovery/commit/5ff462dfd2cfb5e1d91ce5245541b195f0506f29))
* allow multiple scan rows per user target ([#65](https://github.com/create2-labs/cafe-discovery/issues/65)) ([f4efd41](https://github.com/create2-labs/cafe-discovery/commit/f4efd41a93d8eb2a9a7f30ad10ce65895a1f8a73))
* atomic success slot on scan completed with stripped over-limit failure ([#86](https://github.com/create2-labs/cafe-discovery/issues/86)) ([fe536f7](https://github.com/create2-labs/cafe-discovery/commit/fe536f70afa0e149dd50041e7b1a5b0b9e0fa14e))
* **auth:** session validation for CPM integration ([#47](https://github.com/create2-labs/cafe-discovery/issues/47)) ([678f330](https://github.com/create2-labs/cafe-discovery/commit/678f330e17075fe4173c384dfee0fff3581dcbaa))
* backfill scan usage ledger from historical successes ([#88](https://github.com/create2-labs/cafe-discovery/issues/88)) ([c577794](https://github.com/create2-labs/cafe-discovery/commit/c57779472ef9ec84663642541558b19337adb6ed))
* benchmark ([#20](https://github.com/create2-labs/cafe-discovery/issues/20)) ([7548e4b](https://github.com/create2-labs/cafe-discovery/commit/7548e4badc1ec2dc3f58493e0ae8807d2320099e))
* block in-flight wallet rescans ([#71](https://github.com/create2-labs/cafe-discovery/issues/71)) ([b0e595d](https://github.com/create2-labs/cafe-discovery/commit/b0e595dcc8b1bbed8df17277ffe14c144d80995a))
* block wallet POST /scan when CPM policy or draft exists (IMM-9) ([#76](https://github.com/create2-labs/cafe-discovery/issues/76)) ([b005c4c](https://github.com/create2-labs/cafe-discovery/commit/b005c4cbb41b02080ccad042df13337d45fd26de))
* build multi arch docker image amd64 and arm64 ([#28](https://github.com/create2-labs/cafe-discovery/issues/28)) ([287fd61](https://github.com/create2-labs/cafe-discovery/commit/287fd618888a72a7ea1d4b3940e5599f856e7bf8))
* **discovery:** add authenticated wallet policy contexts for CPM ([#48](https://github.com/create2-labs/cafe-discovery/issues/48)) ([6063024](https://github.com/create2-labs/cafe-discovery/commit/60630248805c74b0a18ad5a99a9334933302101c))
* **discovery:** canonical wallet address handling and boundary normalization ([#45](https://github.com/create2-labs/cafe-discovery/issues/45)) ([023cc43](https://github.com/create2-labs/cafe-discovery/commit/023cc430bc3c22e57667da1a8df286fe5671a2d1))
* **discovery:** derive deterministic current_pq_posture for wallet o… ([#42](https://github.com/create2-labs/cafe-discovery/issues/42)) ([47a7906](https://github.com/create2-labs/cafe-discovery/commit/47a79069617a24bb8d5a98577151c3fdd6bba8f5))
* **discovery:** enrich v1 scan result for UI parity and TLS defaults ([#56](https://github.com/create2-labs/cafe-discovery/issues/56)) ([09383b5](https://github.com/create2-labs/cafe-discovery/commit/09383b5e4efb6b67f2d5b9fbb4a9a153b5e73ccc))
* **discovery:** export wallet.observed v0.1 et publication NATS (persistence) ([#41](https://github.com/create2-labs/cafe-discovery/issues/41)) ([4b0fea5](https://github.com/create2-labs/cafe-discovery/commit/4b0fea56306cf526aaed0f47f9b6b4f41cc57d24))
* **discovery:** expose v1 CBOM by scan_id on demand (IMM-12) ([#80](https://github.com/create2-labs/cafe-discovery/issues/80)) ([4a7f181](https://github.com/create2-labs/cafe-discovery/commit/4a7f181c29319db33c180c654ffa54f6fbe285b7))
* **discovery:** gate v1 scan deletes with CPM policy reference check ([#53](https://github.com/create2-labs/cafe-discovery/issues/53)) ([bc18000](https://github.com/create2-labs/cafe-discovery/commit/bc18000b0fc3bbb6f4432d49f4439d0fbcb6c86b))
* **discovery:** implement v1 post scan acceptance contract ([#51](https://github.com/create2-labs/cafe-discovery/issues/51)) ([76f3534](https://github.com/create2-labs/cafe-discovery/commit/76f3534a5c81e08fa2915c749e516f3168b69d96))
* **discovery:** PERS-D2b: scan DDL owned by persistence + boot guard ([#104](https://github.com/create2-labs/cafe-discovery/issues/104)) ([2fd7e22](https://github.com/create2-labs/cafe-discovery/commit/2fd7e224e73672e2d327b101fdb1277c75607c52))
* **discovery:** PERS-D6a-delete: Discovery v1 scan DELETE via cafe-persistence ([#106](https://github.com/create2-labs/cafe-discovery/issues/106)) ([834cdd3](https://github.com/create2-labs/cafe-discovery/commit/834cdd332dec2a80c674183a1d68587d22b9436b))
* **discovery:** PERS-D6a-pending — scan pending via cafe-persistence ([#107](https://github.com/create2-labs/cafe-discovery/issues/107)) ([8e1bae3](https://github.com/create2-labs/cafe-discovery/commit/8e1bae3717ea7924302aacb0d0f915d26bbb84f0))
* **discovery:** publish explicit assessment request command over NATS ([#44](https://github.com/create2-labs/cafe-discovery/issues/44)) ([107507a](https://github.com/create2-labs/cafe-discovery/commit/107507a8350c106afac80792051f55440ccd564b))
* **discovery:** route W1/W3 policy refs through cafe-persistence (PERS-D6b) ([#108](https://github.com/create2-labs/cafe-discovery/issues/108)) ([ec0df94](https://github.com/create2-labs/cafe-discovery/commit/ec0df94e5e13dd48d55235fde96103d0f00c70ef))
* **discovery:** v1 wallet and TLS scan lists and scan detail ([#52](https://github.com/create2-labs/cafe-discovery/issues/52)) ([efc6877](https://github.com/create2-labs/cafe-discovery/commit/efc6877d0d0bc7dd8f4166f6ad09823cd1d0538a))
* Expose internal scan authorization lookup for CPM ([#46](https://github.com/create2-labs/cafe-discovery/issues/46)) ([b7a0076](https://github.com/create2-labs/cafe-discovery/commit/b7a0076c633a1e35330df5a0345e5ee7afa0165e))
* expose used visible and deleted scan counts in usage API ([#87](https://github.com/create2-labs/cafe-discovery/issues/87)) ([d06ced2](https://github.com/create2-labs/cafe-discovery/commit/d06ced2c88273b24778d52bd1459649d11779a84))
* Plan quota: scan usage ledger repository and counters ([#84](https://github.com/create2-labs/cafe-discovery/issues/84)) ([33ee2f3](https://github.com/create2-labs/cafe-discovery/commit/33ee2f3b3308eb6047d20717e0cc6a5926f4abaf))
* post scan guards on successful plus in-flight and parallel cap ([#85](https://github.com/create2-labs/cafe-discovery/issues/85)) ([c9546f5](https://github.com/create2-labs/cafe-discovery/commit/c9546f5b3cc3f18bdd7aed99fa2d7252ee24189f))
* rearchitecture – split worker into Persistence and Scanner services ([#30](https://github.com/create2-labs/cafe-discovery/issues/30)) ([73dab32](https://github.com/create2-labs/cafe-discovery/commit/73dab32d19fbacb55b4165b1e7639f513db9207c))
* route v1 scan reads through cafe-persistence (PERS-D6a-read) ([#105](https://github.com/create2-labs/cafe-discovery/issues/105)) ([06a3977](https://github.com/create2-labs/cafe-discovery/commit/06a3977af429717a24025fa3b31a314b90e50168))
* scan history latest completed ([#70](https://github.com/create2-labs/cafe-discovery/issues/70)) ([5e7cdd2](https://github.com/create2-labs/cafe-discovery/commit/5e7cdd293717a4a5960b2286fc07e42a11d84c3a))
* **scanners:** binaires dédiés wallet/tls, images Docker et CI alignés ([6c5524e](https://github.com/create2-labs/cafe-discovery/commit/6c5524e50ee2fec5901daf2838e479f4df11d038))


### Bug Fixes

* anonymous cannot scan anything ([#26](https://github.com/create2-labs/cafe-discovery/issues/26)) ([6f3ed47](https://github.com/create2-labs/cafe-discovery/commit/6f3ed47cf199da3a862522dafc9c51f3905b471a))
* block wallet POST /scan only when persisted policy exists (IMM-W1-4) ([#92](https://github.com/create2-labs/cafe-discovery/issues/92)) ([8c9eefd](https://github.com/create2-labs/cafe-discovery/commit/8c9eefddc387bf59e08b26ac9980d7de738195cf))
* cbom only for completed successful scans ([#89](https://github.com/create2-labs/cafe-discovery/issues/89)) ([34d79c0](https://github.com/create2-labs/cafe-discovery/commit/34d79c06e4b014cccbc9c0b035a808fc8e849815))
* clean up wallet redis paths after scan history migration ([#72](https://github.com/create2-labs/cafe-discovery/issues/72)) ([f9fb4b4](https://github.com/create2-labs/cafe-discovery/commit/f9fb4b42544f65c3f3666add676d5c4248535377))
* correction in the github action to build the docker images ([#22](https://github.com/create2-labs/cafe-discovery/issues/22)) ([c6f499d](https://github.com/create2-labs/cafe-discovery/commit/c6f499dc27edab4e5d2c4210278552a29f246740))
* count scan executions for quota after history model ([#73](https://github.com/create2-labs/cafe-discovery/issues/73)) ([56607fd](https://github.com/create2-labs/cafe-discovery/commit/56607fdc878e9a901ecbb00a8c5319fa229a77bc))
* **deps:** update fiber to v2.52.14 and add v3 migration plan ([#113](https://github.com/create2-labs/cafe-discovery/issues/113)) ([0e53bb5](https://github.com/create2-labs/cafe-discovery/commit/0e53bb5e3bc86d8343b18673a47bafe613ea23e7))
* derive wallet_type only from scanner completion ([#97](https://github.com/create2-labs/cafe-discovery/issues/97)) ([4e4432f](https://github.com/create2-labs/cafe-discovery/commit/4e4432fa84f7e65e33f9ba79d886044f75c0dae1))
* dev and prod ([#21](https://github.com/create2-labs/cafe-discovery/issues/21)) ([1263602](https://github.com/create2-labs/cafe-discovery/commit/1263602944db8cad5d52c59cfa481a28e43e18dd))
* docker image mgt improved ([#29](https://github.com/create2-labs/cafe-discovery/issues/29)) ([f4031ea](https://github.com/create2-labs/cafe-discovery/commit/f4031ea1f28227015cd96c7dac5f5c285e15af78))
* docupdate ([#63](https://github.com/create2-labs/cafe-discovery/issues/63)) ([683eb74](https://github.com/create2-labs/cafe-discovery/commit/683eb74ef529b8cb5cfd1ed93ba44b326d0b306f))
* expose protocol_version in TLS CBOM response ([#32](https://github.com/create2-labs/cafe-discovery/issues/32)) ([ebf3022](https://github.com/create2-labs/cafe-discovery/commit/ebf3022c0ae0a1b8aa05ec6e80e1c8efca5b5bc2))
* **metrics:** clone fasthttp method before Prometheus labels ([#122](https://github.com/create2-labs/cafe-discovery/issues/122)) ([cad33b6](https://github.com/create2-labs/cafe-discovery/commit/cad33b65814fdafcf1887f6ba2059035b3c2ef4e))
* **metrics:** serve /metrics from dedicated Prometheus registry ([#111](https://github.com/create2-labs/cafe-discovery/issues/111)) ([e785980](https://github.com/create2-labs/cafe-discovery/commit/e78598071f2635135b47e9bb283f778abe0dfb47))
* migrate cafe-discovery to gofiber v3.4.0 ([#114](https://github.com/create2-labs/cafe-discovery/issues/114)) ([94782fb](https://github.com/create2-labs/cafe-discovery/commit/94782fb9705266c16fba902387d331d6d2130770))
* minimal fields on plan limit exceeded stub rows (IMM-D3) ([#95](https://github.com/create2-labs/cafe-discovery/issues/95)) ([899eda8](https://github.com/create2-labs/cafe-discovery/commit/899eda8efa91eb0bde19608b59aac4b4b7553608))
* move TLS scanner to its own repository ([#37](https://github.com/create2-labs/cafe-discovery/issues/37)) ([97fe317](https://github.com/create2-labs/cafe-discovery/commit/97fe31750f1a48f44dcb9a5d295fee43d38604e9))
* nats worker ([#27](https://github.com/create2-labs/cafe-discovery/issues/27)) ([3ff2abb](https://github.com/create2-labs/cafe-discovery/commit/3ff2abb26a4f29a331d9d6b0a70d3ba901693434))
* nil pointer ([#25](https://github.com/create2-labs/cafe-discovery/issues/25)) ([c4f479b](https://github.com/create2-labs/cafe-discovery/commit/c4f479b12327cf3096e548e0b29816d96cb40c50))
* no default scan status (IMM-D2) ([#94](https://github.com/create2-labs/cafe-discovery/issues/94)) ([8dc5fd1](https://github.com/create2-labs/cafe-discovery/commit/8dc5fd1a397cac37e9277897a66ff009bd75be96))
* OnStarted inserts lifecycle fields only (IMM-D1) ([#93](https://github.com/create2-labs/cafe-discovery/issues/93)) ([602d2ed](https://github.com/create2-labs/cafe-discovery/commit/602d2ed8e354224f56b5e910346ab2ca3a2edcb4))
* persistence image must be checked and too ([#31](https://github.com/create2-labs/cafe-discovery/issues/31)) ([f6127da](https://github.com/create2-labs/cafe-discovery/commit/f6127da504f53462960e44e36842904809a7ca5d))
* persistence service imlage does not need to be built over OQS ([#35](https://github.com/create2-labs/cafe-discovery/issues/35)) ([bacf6bf](https://github.com/create2-labs/cafe-discovery/commit/bacf6bffb7026f1d8dd2d4ae5de8fd7fca85b0b5))
* removing deadcode ([#100](https://github.com/create2-labs/cafe-discovery/issues/100)) ([d5149a5](https://github.com/create2-labs/cafe-discovery/commit/d5149a5e66967f43573c364e575e0382fcbb8214))
* slim discovery pr1 ([#117](https://github.com/create2-labs/cafe-discovery/issues/117)) ([f9bc8a8](https://github.com/create2-labs/cafe-discovery/commit/f9bc8a8598b5de3aa6568c3c6356f237e84ea3de))
* using caches improves image builds ([#23](https://github.com/create2-labs/cafe-discovery/issues/23)) ([c0f6a49](https://github.com/create2-labs/cafe-discovery/commit/c0f6a49e7c744bda9c056ec7051b50248d94f22d))
* wallet scan history list filters ([#69](https://github.com/create2-labs/cafe-discovery/issues/69)) ([dd84d1c](https://github.com/create2-labs/cafe-discovery/commit/dd84d1c2068fd2de929e8214b84a554da78bdfe5))

## [Unreleased]

### Fixed
- **GET /metrics HTTP 500 (`method="GETT"` duplicates):** Prometheus labels for HTTP metrics no longer keep fasthttp/Fiber request-buffer views. `c.Method()` is cloned before `Next()`, and `canonicalHTTPMethod` / `sanitizeLabelValue` always return owned strings. Without this, buffer reuse corrupted `method` labels (e.g. `GET` → `GETT`), so `/metrics` failed gather with “collected before with the same name and label values”, `cafe-discovery-api` stayed down, and `platform_up` stayed 0.
- **GET /metrics**: serve from a dedicated Prometheus registry (like CPM) instead of `promhttp.Handler()` on the default gatherer, which could return HTTP 500 in the minimal runtime image when `process`/`go` collectors fail. HTTP metric `path` labels now use route templates only (`_unmatched` fallback) with sanitized values to avoid gather errors from high-cardinality or invalid paths.

### Changed
- **Prometheus HTTP metrics** (`internal/metrics/http.go`): the `method` label on `http_requests_total` and `http_request_duration_seconds` is now normalized via `canonicalHTTPMethod`. Only standard RFC 7231 methods are emitted as-is; empty input becomes `UNKNOWN`, anything else becomes `OTHER` to avoid high-cardinality or garbage labels in Grafana/Prometheus.
