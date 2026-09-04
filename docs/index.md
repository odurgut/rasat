---
title: Rasat
description: Self-hosted observability for OpenTelemetry traces, logs, and service maps.
---

# Rasat

Rasat is an open-source observability product. It helps you **see what your services are doing** — in real time and after the fact — using traces, structured logs, and a service map derived from those traces.

You send OpenTelemetry data. Rasat stores it, lets you search it, and shows a live picture of traffic, errors, and latency. You run it yourself. There is no cloud account and no proprietary agent.

## What you can do

- **Watch the fleet.** Overview shows request rate, error rate, and latency for everything that is currently emitting traces, plus a live activity feed of incoming, slow, and failing work.
- **Find a bad request.** Search traces by service, operation, status, duration, time range, or attribute. Open a waterfall to see every span, its timing, and why it failed.
- **Understand a service.** Each discovered service has a dashboard: traffic, errors, latency percentiles, neighbors, top operations, and error causes.
- **Follow logs next to traces.** Structured logs can carry a `trace_id`. From a span you jump to related logs; from a log you jump to the trace.
- **See how services call each other.** The map is built from parent/child relationships in spans. No inventory file, no manual edges.

## Who it is for

Backend, platform, and SRE teams who already (or want to) emit OpenTelemetry, and who want a single self-hosted place to look — without standing up a stack of separate tools for ingest, storage, search, and UI.

Typical work: production debugging, incident investigation, dependency analysis, finding latency bottlenecks.

## How it fits together

```text
Your applications  ──OTLP──►  Rasat  ──writes──►  ClickHouse
                                 │
                                 ├── query API  ──►  UI
                                 └── live stream ──►  UI
```

Applications (or an OpenTelemetry Collector in front of them) export traces with **OTLP**. Rasat accepts OTLP over HTTP and gRPC, writes ClickHouse, and serves the UI from the same process. Metrics on dashboards are **derived from spans** — you do not send a separate metrics pipeline for rate, error rate, or p95.

Logs use a structured JSON API today, optionally correlated by trace id.

## Documentation

| | |
|---|---|
| [Getting started](getting-started.md) | Point Rasat at ClickHouse and send traces |
| [Hosted demo](https://demo.rasat.dev) | Cassette UI — synthetic shop, not ingest |
| [Compatibility](compatibility.md) | ClickHouse, OTLP, OS, Docker — what we support |
| [Concepts](concepts.md) | Traces, logs, derived metrics, live data, time windows |
| [Send data](send-data.md) | OpenTelemetry SDKs, Collector, structured logs |
| [Overview](overview.md) | Fleet KPIs, charts, activity, issues |
| [Traces](traces.md) | Search, live list, waterfall, inspector |
| [Services](services.md) | Catalog and per-service dashboards |
| [Logs](logs.md) | Live tail and correlation |
| [Service map](service-map.md) | Dependencies from spans |
| [Self-hosting](self-hosting.md) | Deploy, health, storage, scale |
| [Demo data and load](demo-and-load.md) | Hosted cassette, then `rasat-seed` / `rasat-bench` |
| [Configuration](configuration.md) | Environment variables |
| [HTTP API](api.md) | Query, ingest, and stream reference |
| [Changelog](changelog.md) | Releases and `GET /version` |
| [Current limits](limits.md) | What this version does not include |

Rasat is production-oriented: bounded queries, timeouts, graceful shutdown, and ingest that does not stall because a browser is slow. It is also honest about scope — see [Current limits](limits.md). Licensed under [Apache 2.0](../LICENSE).
