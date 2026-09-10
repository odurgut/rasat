---
title: Changelog
description: Notable changes in each Rasat release.
---

# Changelog

Versions are git tags `vMAJOR.MINOR.PATCH`. The running process reports the tag on [`GET /version`](api.md) and in the UI rail. While major is 0, a **minor** tag may break env vars, the query HTTP API, OTLP paths, or the image contract; those breaks are listed on the release that introduces them. How tags relate to `main` and Docker Hub: [CONTRIBUTING.md](../CONTRIBUTING.md).

Pull `odurgut/rasat:<version>` from [Docker Hub](https://hub.docker.com/r/odurgut/rasat). Each tag also updates `odurgut/rasat:<major>.<minor>` and `latest`.

## Unreleased

## 0.1.2 — 2026-09-10

Patch. Image: `odurgut/rasat:0.1.2`.

### Added

- [When to use Rasat](when.md): when it fits versus SigNoz, Jaeger, or Tempo, and what this version installs (Docker yes, Helm no, RAM is ClickHouse, live UI is one replica).

### Changed

- Light/dark follows the operating system until you pick a theme in the rail.

### Fixed

- Narrow viewports: the tab rail, search, waterfall, and map stay on the page instead of overflowing.
- The service map draws as soon as the graph returns, instead of waiting on metrics (which could leave it blank).
- Span inspector log correlation filters by `span_id`, so you see that span's lines, not the whole trace.

## 0.1.1 — 2026-09-03

Patch. Image: `odurgut/rasat:0.1.1`.

### Fixed

- **Waterfall placement.** `GET /api/traces/{id}` span objects include `start_offset_ns` (nanoseconds from the trace start). The UI positions bars from that offset instead of JSON timestamps, which could round and drift.
- **Local Compose data.** `make compose-build` keeps the ClickHouse volume. A rebuild no longer wipes traces you already ingested.

### Changed

- The hosted cassette at [demo.rasat.dev](https://demo.rasat.dev) is rebuilt from the same git tag as the Hub image.

## 0.1.0 — 2026-09-02

First public release. Image: `odurgut/rasat:0.1.0`.

### Added

- Single process: OTLP ingest (HTTP and gRPC), ClickHouse storage, query API, live stream, and UI.
- Multi-arch image (`linux/amd64`, `linux/arm64`) published from the git tag.
- Overview (rate, errors, latency, activity), trace search and waterfall, structured logs with optional `trace_id`, per-service dashboards, and a service map from span parent/child edges.
- Dashboard metrics derived from spans. There is no separate metrics ingest.
- Build identity on `GET /version` and in the UI rail.

Install: [Getting started](getting-started.md). Scope: [Current limits](limits.md).
