---
title: Configuration
description: Environment variables that control Rasat.
---

# Configuration

Rasat is configured entirely through the environment. There is no config file. Empty or unset variables use the defaults below.

Copy `.env.example` as a checklist when you deploy. Do not commit secrets.

## Listeners and process

| Variable | Default | Description |
|---|---|---|
| `RASAT_HTTP_ADDR` | `:8080` | HTTP: UI, OTLP/HTTP, query API, log ingest, WebSockets, `/health`, `/ready` |
| `RASAT_GRPC_ADDR` | `:4317` | OTLP/gRPC |
| `RASAT_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `RASAT_LOG_FORMAT` | `json` | `json` or `text` |
| `RASAT_SHUTDOWN_TIMEOUT` | `15s` | Time to drain HTTP on SIGINT/SIGTERM |
| `RASAT_HTTP_MAX_BODY` | `16777216` | Max request body size in bytes |

## ClickHouse

| Variable | Default | Description |
|---|---|---|
| `RASAT_CLICKHOUSE_ADDR` | `127.0.0.1:9000` | Native protocol `host:port` |
| `RASAT_CLICKHOUSE_DATABASE` | `rasat` | Database; created during migrate |
| `RASAT_CLICKHOUSE_USER` | `default` | Compose uses `rasat` |
| `RASAT_CLICKHOUSE_PASSWORD` | empty | Compose sets a password |
| `RASAT_CLICKHOUSE_PING_TIMEOUT` | `2s` | `/ready` ping |
| `RASAT_CLICKHOUSE_DIAL_TIMEOUT` | `5s` | Connect |
| `RASAT_CLICKHOUSE_MIGRATE_TIMEOUT` | `30s` | Schema DDL on startup |
| `RASAT_CLICKHOUSE_INSERT_TIMEOUT` | `30s` | Trace and log writes |
| `RASAT_CLICKHOUSE_QUERY_TIMEOUT` | `10s` | Search, detail, catalog, metrics |

## Queries

| Variable | Default | Description |
|---|---|---|
| `RASAT_QUERY_MAX_WINDOW` | `168h` | Maximum `end − start` for read APIs |

## Live stream

| Variable | Default | Description |
|---|---|---|
| `RASAT_STREAM_BUFFER` | `64` | Per-client WebSocket buffer; overflow drops that client |
| `RASAT_STREAM_MAX_CLIENTS` | `64` | Max concurrent stream subscribers; extra connections get 503 |
| `RASAT_STREAM_WRITE_TIMEOUT` | `10s` | Write deadline per WebSocket message |
| `RASAT_STREAM_MAX_PER_SEC` | `100` | Max live events published per second (traces and logs). `0` = no cap. Does not slow ingest |

## Example

JSON logs in production, text on a workstation:

```bash
RASAT_LOG_FORMAT=text RASAT_HTTP_ADDR=:8080 rasat
```

Compose already sets the ClickHouse address to the `clickhouse` service and matching user/password. Override there if you change credentials.
