---
title: Concepts
description: Traces, logs, derived metrics, live streams, and time windows in Rasat.
---

# Concepts

Rasat is built around OpenTelemetry’s trace model, plus structured logs that can point at a trace. Dashboards do not require a second metrics product: rate, errors, and latency come from the spans you already send.

## Traces and spans

A **trace** is one request (or one unit of work) as it crosses services. It is identified by a hex **trace id**.

A **span** is one operation inside that trace: an HTTP handler, a database call, a queue publish. Spans form a tree through parent ids.

```text
trace
 ├─ HTTP GET /checkout          web-bff
 │   ├─ Authorize               auth
 │   └─ CreateOrder             checkout
 │       └─ INSERT orders       checkout
 └─ …
```

Rasat stores:

- Span timing, status, name, and service
- Span attributes and resource attributes
- Events on a span (including exceptions)
- Links to other traces or spans

Services are **discovered from spans**. You do not register a catalog.

## Status

OpenTelemetry span status is one of **unset**, **ok**, or **error**. In the UI, error spans and traces are marked **ERR**. Search can filter on that status.

## Logs

A log line in Rasat is a structured record: time, service, level, message, and optional **trace id** / **span id**.

Logs are not inferred from traces. You send them explicitly (JSON HTTP API). When `trace_id` is set, the trace inspector can show **related logs**, and the logs page can jump to the waterfall.

Levels such as `WARNING` and `ERR` are normalized to `WARN` and `ERROR`.

## Metrics (from traces)

Request **rate**, **error rate**, and latency **p50 / p95 / p99** on overview and service dashboards are aggregations over spans in the selected time window. Optional `step` buckets those series for charts.

This is not Prometheus scrape and not OTLP metrics ingest. If you only send traces, the dashboards still fill.

## Service map

Edges are parent/child **service** relationships in spans: if a span in `checkout` is a child of a span in `web-bff`, the map draws `web-bff → checkout`. Call counts, error share, and latency on an edge come from those spans. Nothing is configured by hand.

## Live data

After Rasat accepts a trace or a log, it **stores** the data and **publishes** a small summary to connected browsers (WebSocket).

- Lists and the overview activity feed can **prepend** new rows as they arrive.
- If you scroll away or hover the list, follow mode pauses. A **N new** control appears; new events queue until you catch up.
- Storage always gets the full write. The live feed is a view. Slow browsers are disconnected rather than blocking ingest.
- Publish rate is capped (default 100 events per second) so a firehose does not freeze the UI. ClickHouse still holds every span. See [Self-hosting](self-hosting.md).

Live follow applies when the search **end** time is “now” (a live tail). Historical windows are snapshots.

## Time windows

Almost every read is bounded:

- Trace and log **search** require `start`, `end`, and a **limit**.
- Catalog, map, metrics, and error-causes require a **limit**. If you omit `start`/`end`, Rasat uses a default window of “now minus max window” through now.
- The maximum window is **7 days** by default (`168h`). Wider queries are rejected.

The UI keeps the window in the URL so you can share a view. Overview and service dashboards offer presets: **1h**, **6h**, **24h**, **7d**.

## What Rasat is not

Rasat is not a log file tailer on disk, not an APM agent, and not a metrics TSDB. It is the place you look once OpenTelemetry (and optional structured logs) are already leaving your services. [Current limits](limits.md) lists features that are not in this version (auth, alerting, Kubernetes install, OTLP logs).
