# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0-alpha](https://github.com/create2-labs/cafe-discovery/compare/v0.1.1-alpha...v0.2.0-alpha) (2026-04-04)


### Features

* benchmark ([#20](https://github.com/create2-labs/cafe-discovery/issues/20)) ([7548e4b](https://github.com/create2-labs/cafe-discovery/commit/7548e4badc1ec2dc3f58493e0ae8807d2320099e))
* build multi arch docker image amd64 and arm64 ([#28](https://github.com/create2-labs/cafe-discovery/issues/28)) ([287fd61](https://github.com/create2-labs/cafe-discovery/commit/287fd618888a72a7ea1d4b3940e5599f856e7bf8))
* rearchitecture – split worker into Persistence and Scanner services ([#30](https://github.com/create2-labs/cafe-discovery/issues/30)) ([73dab32](https://github.com/create2-labs/cafe-discovery/commit/73dab32d19fbacb55b4165b1e7639f513db9207c))
* **scanners:** binaires dédiés wallet/tls, images Docker et CI alignés ([6c5524e](https://github.com/create2-labs/cafe-discovery/commit/6c5524e50ee2fec5901daf2838e479f4df11d038))


### Bug Fixes

* anonymous cannot scan anything ([#26](https://github.com/create2-labs/cafe-discovery/issues/26)) ([6f3ed47](https://github.com/create2-labs/cafe-discovery/commit/6f3ed47cf199da3a862522dafc9c51f3905b471a))
* correction in the github action to build the docker images ([#22](https://github.com/create2-labs/cafe-discovery/issues/22)) ([c6f499d](https://github.com/create2-labs/cafe-discovery/commit/c6f499dc27edab4e5d2c4210278552a29f246740))
* dev and prod ([#21](https://github.com/create2-labs/cafe-discovery/issues/21)) ([1263602](https://github.com/create2-labs/cafe-discovery/commit/1263602944db8cad5d52c59cfa481a28e43e18dd))
* docker image mgt improved ([#29](https://github.com/create2-labs/cafe-discovery/issues/29)) ([f4031ea](https://github.com/create2-labs/cafe-discovery/commit/f4031ea1f28227015cd96c7dac5f5c285e15af78))
* expose protocol_version in TLS CBOM response ([#32](https://github.com/create2-labs/cafe-discovery/issues/32)) ([ebf3022](https://github.com/create2-labs/cafe-discovery/commit/ebf3022c0ae0a1b8aa05ec6e80e1c8efca5b5bc2))
* move TLS scanner to its own repository ([#37](https://github.com/create2-labs/cafe-discovery/issues/37)) ([97fe317](https://github.com/create2-labs/cafe-discovery/commit/97fe31750f1a48f44dcb9a5d295fee43d38604e9))
* nats worker ([#27](https://github.com/create2-labs/cafe-discovery/issues/27)) ([3ff2abb](https://github.com/create2-labs/cafe-discovery/commit/3ff2abb26a4f29a331d9d6b0a70d3ba901693434))
* nil pointer ([#25](https://github.com/create2-labs/cafe-discovery/issues/25)) ([c4f479b](https://github.com/create2-labs/cafe-discovery/commit/c4f479b12327cf3096e548e0b29816d96cb40c50))
* persistence image must be checked and too ([#31](https://github.com/create2-labs/cafe-discovery/issues/31)) ([f6127da](https://github.com/create2-labs/cafe-discovery/commit/f6127da504f53462960e44e36842904809a7ca5d))
* persistence service imlage does not need to be built over OQS ([#35](https://github.com/create2-labs/cafe-discovery/issues/35)) ([bacf6bf](https://github.com/create2-labs/cafe-discovery/commit/bacf6bffb7026f1d8dd2d4ae5de8fd7fca85b0b5))
* using caches improves image builds ([#23](https://github.com/create2-labs/cafe-discovery/issues/23)) ([c0f6a49](https://github.com/create2-labs/cafe-discovery/commit/c0f6a49e7c744bda9c056ec7051b50248d94f22d))

## [Unreleased]

### Changed
- **Prometheus HTTP metrics** (`internal/metrics/http.go`): the `method` label on `http_requests_total` and `http_request_duration_seconds` is now normalized via `canonicalHTTPMethod`. Only standard RFC 7231 methods are emitted as-is; empty input becomes `UNKNOWN`, anything else becomes `OTHER` to avoid high-cardinality or garbage labels in Grafana/Prometheus.
