---
title: Self-hosting
description: Run Rasat against your ClickHouse — health, storage, and scaling constraints.
---

# Self-hosting

Rasat is **self-hosted**. You run **ClickHouse**; you run **Rasat**. The UI is inside the Rasat process. There is no separate frontend server in production. We do not manage ClickHouse for you.

## Architecture

```text
Applications / Collector
        │ OTLP/HTTP :8080
        │ OTLP/gRPC :4317
        ▼
     Rasat ────── native protocol ────► ClickHouse
        │
        ├── HTTP :8080  UI, query API, log ingest, /health, /ready
        └── WebSocket    live traces and logs
```

ClickHouse is the system of record. Rasat migrates its schema on startup. If ClickHouse is down, Rasat still answers `/health` but `/ready` fails and you should not send traffic.

The **live stream hub is in-process**. Connected browsers subscribe to that process. A second Rasat replica does not share WebSockets: live overview, traces, and logs would split. Search and ingest can sit behind a load balancer; **keep one replica** if you care about a single live feed. See [Current limits](limits.md).

## ClickHouse

You operate it: package, VM, existing 24.8, Docker, whatever you already use. Rasat needs:

- Native protocol **9000** (`RASAT_CLICKHOUSE_ADDR` as `host:port`). HTTP 8123 is unused.
- A database name (`RASAT_CLICKHOUSE_DATABASE`, default `rasat`). Schema is created on startup for **24.8**. This version does not migrate older Rasat databases.
- A user that can create that database and tables.

Versions: [Compatibility](compatibility.md). Disk, TTL, backups, and HA are ClickHouse problems — [Storage and retention](#storage-and-retention).

## Run Rasat

Image [`odurgut/rasat`](https://hub.docker.com/r/odurgut/rasat) or a binary from `make build`. Distroless, non-root. Configuration is **environment only** — [Configuration](configuration.md). Defaults are `127.0.0.1:9000`, user `default`, empty password; set `RASAT_CLICKHOUSE_*` to match **your** server.

```bash
docker run -d --name rasat \
  -p 8080:8080 -p 4317:4317 \
  -e RASAT_CLICKHOUSE_ADDR=clickhouse.internal:9000 \
  -e RASAT_CLICKHOUSE_DATABASE=rasat \
  -e RASAT_CLICKHOUSE_USER=rasat \
  -e RASAT_CLICKHOUSE_PASSWORD=rasat \
  odurgut/rasat:0.1.1
```

Same variables in systemd, Kubernetes, or a shell. This version has **no authentication**; anyone who can reach port 8080 can query and ingest. Bind it to a network you trust.

| Port | Role |
|---|---|
| 8080 | Rasat HTTP (UI, OTLP/HTTP, API) |
| 4317 | Rasat OTLP/gRPC |

A laptop Compose file that also starts ClickHouse is in [Getting started](getting-started.md#quick-stack-optional). Use it only if you want that convenience stack.

## Health

| Endpoint | Meaning |
|---|---|
| `GET /health` | Process is serving. Use as **liveness**. |
| `GET /ready` | ClickHouse ping within the ping timeout. Use as **readiness**. **200** or **503**. |
| `GET /version` | Image tag or git describe. See [Changelog](changelog.md). |

Logs are structured. In containers they are JSON (`RASAT_LOG_FORMAT`). SIGINT and SIGTERM drain in-flight HTTP for `RASAT_SHUTDOWN_TIMEOUT` (default 15 seconds).

## Storage and retention

All traces, events, links, and logs live in ClickHouse. Disk growth is a ClickHouse problem: size volumes, and apply TTL or merges there if you need expiry. Rasat does not run a retention job in this version.

Losing ClickHouse loses history. Back it up if the data matters.

## Capacity notes

- Query APIs require a time window and a limit. The max window defaults to 7 days.
- Insert and query timeouts are configurable. Slow ClickHouse surfaces as errors, not hangs without bound.
- Live publish defaults to **100 events per second** (`RASAT_STREAM_MAX_PER_SEC`). `0` disables the cap. Ingest throughput is independent: every accepted batch is written.
- Each browser has a small WebSocket buffer; a full buffer disconnects that client.
- Concurrent stream subscribers are capped (`RASAT_STREAM_MAX_CLIENTS`); extras receive 503.

To fill a local UI without an app, or to measure ingest on your hardware, use [Demo data and load](demo-and-load.md). Those binaries are not in the Rasat image.

## Reverse proxies

If you terminate TLS in nginx, Caddy, or a cloud load balancer:

- Forward `/` and `/api/` to Rasat HTTP.
- Support **WebSocket** upgrades for `/api/stream/traces` and `/api/stream/logs`.
- OTLP/gRPC is HTTP/2 on **4317**; either expose it separately or use a proxy that can route gRPC.

Do not cache the UI as a substitute for rebuilding the image when you upgrade.

## Kubernetes

No chart, operator, or Helm release in this version. Run the image, point `RASAT_CLICKHOUSE_*` at ClickHouse you already run in the cluster, keep **replicas at 1** until a shared live hub exists, and use `/health` and `/ready` as probes.
