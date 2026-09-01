---
title: Services
description: Discovered services and per-service dashboards.
---

# Services

Services is the **catalog** of names Rasat has seen on spans, plus a **dashboard** for one service. Nothing is registered ahead of time: emit traces, and the name appears.

The time range matches overview presets: **1h**, **6h**, **24h**, **7d**.

## Catalog

The list is every service with spans in the window. Select a name to open its dashboard. You can also arrive here from overview issues, the map, or a span’s service link.

## Dashboard

For the selected service and window:

### KPIs

**RATE**, **ERR/s**, **ERR**, **P50**, **P95**, **P99** — same definitions as [overview](overview.md), scoped to this service. Error KPIs signal when errors are present.

### Charts

**traffic**, **errors**, and **latency** (p50 / p95 / p99) over time.

### Error causes

Top causes for this service (exception type or status message), with count and first seen. Click a cause to open [trace search](traces.md) for that service with status **error**.

### Dependencies and dependents

From the [service map](service-map.md) in the same window:

- **Dependencies** — services this one calls
- **Dependents** — services that call this one

Click stays on dashboards: you switch the selected service rather than leaving the page.

### Operations

- **Top operations** — highest volume
- **Slowest** — highest latency

Each row includes volume, error rate, p50, and p95 where available. Click opens traces filtered to that service and operation.

## How to use it

1. Overview says `checkout` error rate is high → open **services** → `checkout`.
2. Read whether it is one operation or many, and whether neighbors are involved.
3. Jump to traces for the slow or failing operation, then the waterfall.

The dashboard is derived from spans. If a service only logs and never traces, it will not appear here.
