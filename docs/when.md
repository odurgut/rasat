---
title: When to use Rasat
description: When Rasat fits — versus SigNoz, Jaeger, or Tempo — and what this version does not install.
---

# When to use Rasat

Rasat is a **self-hosted OpenTelemetry backend**: one process that accepts OTLP traces, stores them in **ClickHouse you run**, and serves a UI for live traffic, search, waterfalls, correlated logs, span-derived charts, and a service map.

Use it when you already emit OTLP (or will), you want that picture without a SaaS account, and you do not want to stand up a multi-piece tracing stack. Skip it — for now — if you need login, alerting, a Helm chart, Prometheus ingest, or OTLP logs. Those limits are documented, not accidental: [Current limits](limits.md).

## Instead of SigNoz, Jaeger, Tempo

**SigNoz** is a broader observability product: metrics ingest, alerts, and more moving parts. Rasat is narrower. It does not ingest Prometheus or OTLP metrics; dashboards are aggregations over **spans you already send**. There are no alert rules. If you need that suite, SigNoz (or what you already run) is the better fit today. If you want one binary in front of ClickHouse for trace-centric investigation, Rasat is the smaller install.

**Jaeger** is a tracing system with its own collectors and storage options. Rasat takes **OTLP** only — no Jaeger agent, no Rasat-specific SDK — and keeps UI, query, and receivers in one process. The hosted cassette at [demo.rasat.dev](https://demo.rasat.dev) is a UI walkthrough, not ingest, the way Jaeger’s HotROD is a demo app rather than the server.

**Grafana Tempo** is a trace backend you typically wire into Grafana. Rasat includes the UI. There is no Grafana plugin and no separate query frontend.

None of this is a claim that Rasat replaces those products. It is a smaller scope on purpose.

## Docker

Yes. Image [`odurgut/rasat`](https://hub.docker.com/r/odurgut/rasat). You still run **ClickHouse** (native 9000). Commands: [Getting started](getting-started.md).

## Kubernetes

No official chart, operator, or Helm release in this version. Run the same image, set `RASAT_CLICKHOUSE_*`, use `/health` and `/ready` as probes, and keep **replicas at 1** if you want a single live UI stream. [Self-hosting](self-hosting.md#kubernetes).

## RAM

We do not publish a memory floor. The Rasat process is a distroless Go binary; **ClickHouse holds the data** and will use most of the RAM and disk. Size ClickHouse for span volume and retention (TTL is a ClickHouse job — Rasat does not expire rows). Then run Rasat next to it. Load numbers from `rasat-bench` are for your hardware, not a contract: [Demo data and load](demo-and-load.md).

## Scale

ClickHouse is the capacity limit for history. Rasat ingest writes every accepted batch. The live UI stream is **in-process** and capped (`RASAT_STREAM_MAX_PER_SEC`); that cap does not throttle ingest. Search and ingest can sit behind a load balancer; a second Rasat replica does **not** share the live hub. One replica if you care about a single live feed. [Self-hosting](self-hosting.md#capacity-notes).

## Trace detail

Open a trace and you get a waterfall, span inspector, and logs that share the `trace_id`. The homepage shows a static waterfall; the full UI is on the [hosted cassette](https://demo.rasat.dev) and in [Traces](traces.md).
