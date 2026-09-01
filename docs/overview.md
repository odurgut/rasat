---
title: Overview
description: Fleet health — KPIs, charts, live activity, issues, and regressions.
---

# Overview

Overview is the default landing page. It answers: **is the system healthy in this window, and what should I open first?**

Pick a range in the header: **1h**, **6h**, **24h**, or **7d**. The window is stored in the URL. Charts refresh about every 15 seconds while this page is visible; you do not need to reload.

## KPIs

Across all services in the window:

| Label | Meaning |
|---|---|
| **RATE** | Span-derived request rate |
| **ERR/s** | Error spans per second |
| **ERR** | Error share of spans |
| **P95** / **P99** | Latency percentiles, weighted by span volume |
| **SPANS** | Span count in the window |

Error figures turn into an error signal when they are non-zero. These numbers are [derived from traces](concepts.md), not a separate metrics pipeline.

## Charts

- **traffic** — rate over time
- **errors** — error rate over time
- **latency** — p50, p95, p99 over time

Bucket size follows the preset (1 minute on 1h, up to 1 hour on 7d).

## Activity

Below the charts, a live **activity** feed lists recent traces:

| Kind | When |
|---|---|
| **in** | New work that is not an error and not slow |
| **slow** | Duration at or above the window **p95** |
| **err** | Trace status error |

The feed hydrates from search, then follows the live trace stream. Click a row to open that trace’s waterfall.

If you scroll the feed or hover it, Rasat stops jumping to the top. **N new** appears for events that arrived while you were reading. Return to the top (or use that control) to resume follow.

The live channel is rate-limited so a large ingest does not freeze the browser. Historical search and ClickHouse still see every span.

## Issues, incidents, regressions, busiest

Three cards under activity:

**Issues** — services with the highest error rates. Click opens that service’s dashboard.

**Incidents** — top error **causes** (exception type or status message) fleet-wide, with a count and **first seen** in the window. Click opens trace search for errors.

**Regressions** — services whose p95 rose in the second half of the window versus the first (at least ~10%). Shows previous → current p95.

**Slowest** — highest p95. **Busiest** — most spans, with p95.

Use these as a triage list, not as alerting. This version does not page anyone; see [Current limits](limits.md).

## Where to go next

- A specific service → [Services](services.md)
- A specific trace from activity → [Traces](traces.md)
- Dependencies → [Service map](service-map.md)
