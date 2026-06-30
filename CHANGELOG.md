# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- **GET /metrics**: serve from a dedicated Prometheus registry (like CPM) instead of `promhttp.Handler()` on the default gatherer, which could return HTTP 500 in the minimal runtime image when `process`/`go` collectors fail. HTTP metric `path` labels now use route templates only (`_unmatched` fallback) with sanitized values to avoid gather errors from high-cardinality or invalid paths.

### Changed
- **Prometheus HTTP metrics** (`internal/metrics/http.go`): the `method` label on `http_requests_total` and `http_request_duration_seconds` is now normalized via `canonicalHTTPMethod`. Only standard RFC 7231 methods are emitted as-is; empty input becomes `UNKNOWN`, anything else becomes `OTHER` to avoid high-cardinality or garbage labels in Grafana/Prometheus.
