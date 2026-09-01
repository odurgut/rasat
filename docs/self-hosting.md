---
title: Self-hosting
description: Deploy Rasat with ClickHouse, health checks, storage, and scaling constraints.
---

# Self-hosting

Rasat is **self-hosted**. A complete install is two processes: **ClickHouse** and **Rasat**. The UI is inside the Rasat process. There is no separate frontend server in production.

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

## Docker Compose

This is the supported install. The published file is `deploy/docker-compose.yml`. It runs ClickHouse 24.8 and Rasat, wires matching credentials, waits for ClickHouse health, and publishes:

| Port | Service |
|---|---|
| 8080 | Rasat HTTP |
| 4317 | Rasat OTLP/gRPC |
| 9000 | ClickHouse native (optional to publish; Rasat uses it on the Docker network) |
| 8123 | ClickHouse HTTP (Compose publishes it; Rasat does not need it) |

```bash
make compose-up
```

`make compose-up` passes `git describe` into the image (`GET /version`). The same Compose file without `VERSION` / `COMMIT` reports `dev`.

The Rasat image is distroless and runs as non-root. Compose default credentials are for a **private** machine. Change the password before this is reachable on a network you do not trust. Rasat has **no authentication** in this version; anyone who can reach port 8080 can query and ingest.

Stop:

```bash
docker compose -f deploy/docker-compose.yml down
```

Add `-v` only when you intend to delete stored telemetry.

## Binary

Run a Rasat binary against a ClickHouse you already operate. Listen addresses and ClickHouse DSN are [environment variables](configuration.md). Defaults expect ClickHouse at `127.0.0.1:9000` with user `default` and an empty password — that is **not** the Compose user. Align `RASAT_CLICKHOUSE_*` with the server.

The process reads the environment, not a config file.

## Health

| Endpoint | Meaning |
|---|---|
| `GET /health` | Process is serving. Use as **liveness**. |
| `GET /ready` | ClickHouse ping within the ping timeout. Use as **readiness**. **200** or **503**. |
| `GET /version` | Git-stamped version and commit. See [Changelog](changelog.md). |

Logs are structured. In containers they are JSON (`RASAT_LOG_FORMAT`). SIGINT and SIGTERM drain in-flight HTTP for `RASAT_SHUTDOWN_TIMEOUT` (default 15 seconds).

## Storage and retention

All traces, events, links, and logs live in ClickHouse. Disk growth is a ClickHouse problem: size volumes, and apply TTL or merges there if you need expiry. Rasat does not run a retention job in this version.

Losing the ClickHouse volume loses history. Back up ClickHouse if the data matters.

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

A Kubernetes install is **not** shipped in this version. Compose is the documented path. If you run Rasat on a cluster yourself, keep **replicas at 1** until a shared live hub exists, and use `/health` and `/ready` as probes.
