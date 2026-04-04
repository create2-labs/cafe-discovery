# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- **Prometheus HTTP metrics** (`internal/metrics/http.go`): the `method` label on `http_requests_total` and `http_request_duration_seconds` is now normalized via `canonicalHTTPMethod`. Only standard RFC 7231 methods are emitted as-is; empty input becomes `UNKNOWN`, anything else becomes `OTHER` to avoid high-cardinality or garbage labels in Grafana/Prometheus.
