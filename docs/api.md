---
title: HTTP API
description: Query, ingest, and live stream over the HTTP listener.
---

# HTTP API

JSON over the HTTP listener. Default origin: `http://127.0.0.1:8080`. Times are RFC3339 UTC. Durations are Go duration strings (`500ms`, `1s`, `1m`) unless a `_ns` field is used.

OTLP/gRPC is a separate listener (`RASAT_GRPC_ADDR`, default `:4317`). It is not on this port.

Each endpoint below is **signature → parameters → response → example**.

## Conventions

| Name | Rule |
|---|---|
| `limit` | Integer **1–1000** |
| Time window | `end` after `start`; `end − start` ≤ `RASAT_QUERY_MAX_WINDOW` (default `168h`) |
| Optional window | `start` and `end` together, or both omitted → `[now − max window, now]` |
| Error body | `{ "error": "..." }` |

| Status | Meaning |
|---|---|
| **200** | Success |
| **400** | Invalid query (bad time, missing `limit`, window too wide) |
| **404** | Trace id not found |
| **415** | Unsupported `Content-Type` on ingest |
| **503** | ClickHouse down (`/ready`), storage error, or too many stream subscribers |

## Process

### Health

<p class="api-sig"><span class="api-verb">GET</span>/health</p>

Liveness. Does not ping ClickHouse.

<p class="api-label">parameters</p>

<p class="api-none">None.</p>

<p class="api-label">response</p>

<p class="api-status">200</p>

```json
{ "ok": true, "version": "v0.1.0" }
```

<p class="api-label">example</p>

```bash
curl -sS http://127.0.0.1:8080/health
```

### Ready

<p class="api-sig"><span class="api-verb">GET</span>/ready</p>

Readiness. Bounded ClickHouse ping.

<p class="api-label">parameters</p>

<p class="api-none">None.</p>

<p class="api-label">response</p>

<p class="api-status">200</p>

```json
{ "ok": true }
```

<p class="api-status">503</p>

```json
{ "ok": false, "reason": "clickhouse" }
```

<p class="api-label">example</p>

```bash
curl -sS http://127.0.0.1:8080/ready
```

### Version

<p class="api-sig"><span class="api-verb">GET</span>/version</p>

Build identity from link flags. A tag is `vMAJOR.MINOR.PATCH`. Untagged builds use `git describe`. Builds without ldflags report `dev` / `none`. See [Changelog](changelog.md).

<p class="api-label">parameters</p>

<p class="api-none">None.</p>

<p class="api-label">response</p>

<p class="api-status">200</p>

```json
{ "version": "v0.1.0", "commit": "a1b2c3d" }
```

<p class="api-label">example</p>

```bash
curl -sS http://127.0.0.1:8080/version
```

### UI

<p class="api-sig"><span class="api-verb">GET</span>/</p>

Embedded product UI.

<p class="api-label">parameters</p>

<p class="api-none">None.</p>

<p class="api-label">response</p>

<p class="api-status">200</p>

`text/html`

<p class="api-label">example</p>

```bash
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8080/
```

## Ingest

### OTLP traces

<p class="api-sig"><span class="api-verb">POST</span>/v1/traces</p>

OpenTelemetry Protocol. See [Send data](send-data.md).

<p class="api-label">parameters</p>

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| `Content-Type` | header | string | yes | `application/x-protobuf` or `application/json` |
| `Content-Encoding` | header | string | no | `gzip` |
| `body` | body | OTLP | yes | `ExportTraceServiceRequest`. Max `RASAT_HTTP_MAX_BODY` (16 MiB) |

<p class="api-label">response</p>

<p class="api-status">200</p>

Accepted.

<p class="api-status">415</p>

Wrong `Content-Type`.

<p class="api-label">example</p>

```bash
curl -sS -X POST 'http://127.0.0.1:8080/v1/traces' \
  -H 'Content-Type: application/x-protobuf' \
  --data-binary @traces.pb
```

### Structured logs

<p class="api-sig"><span class="api-verb">POST</span>/api/logs</p>

JSON object or array (max 10 000 records). See [Send data](send-data.md).

<p class="api-label">parameters</p>

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| `Content-Type` | header | string | yes | `application/json` |
| `Content-Encoding` | header | string | no | `gzip` |
| `service` | body | string | yes | Emitting service |
| `timestamp` | body | RFC3339 | no | Receive time if omitted |
| `level` | body | string | no | `INFO`, `WARN`, `ERROR`, … |
| `message` | body | string | no | Truncated if extremely long |
| `trace_id` | body | string | no | Correlates with a trace |
| `span_id` | body | string | no | Optional finer correlation |

<p class="api-label">response</p>

<p class="api-status">200</p>

```json
{ "accepted": 1 }
```

<p class="api-label">example</p>

```bash
curl -sS -X POST 'http://127.0.0.1:8080/api/logs' \
  -H 'Content-Type: application/json' \
  -d '{
    "timestamp": "2026-08-26T12:00:00Z",
    "service": "checkout",
    "level": "ERROR",
    "message": "database timeout",
    "trace_id": "abc123"
  }'
```

## Traces

### Search

<p class="api-sig"><span class="api-verb">GET</span>/api/traces</p>

List traces in a window. Newest first.

<p class="api-label">parameters</p>

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| `start` | query | RFC3339 | yes | Window start |
| `end` | query | RFC3339 | yes | Window end |
| `limit` | query | int | yes | Max rows, 1–1000 |
| `service` | query | string | no | Exact service name |
| `operation` | query | string | no | Span name. Alias: `op` |
| `trace_id` | query | hex | no | Trace id. Alias: `trace` |
| `status` | query | string | no | `unset`/`0`, `ok`/`1`, `error`/`err`/`2`. Alias: `status_code` |
| `min_duration` | query | duration | no | e.g. `500ms`. Do not set with `min_duration_ns` |
| `max_duration` | query | duration | no | Do not set with `max_duration_ns` |
| `min_duration_ns` | query | uint | no | Nanoseconds |
| `max_duration_ns` | query | uint | no | Nanoseconds |
| `attr` | query | string | no | Span attribute `key=value` |

<p class="api-label">response</p>

<p class="api-status">200</p>

```json
{
  "traces": [
    {
      "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
      "service": "checkout",
      "operation": "HTTP POST /pay",
      "duration_ns": 412000000,
      "span_count": 13,
      "timestamp": "2026-08-26T12:00:00Z",
      "status_code": 2
    }
  ]
}
```

`status_code`: `0` unset, `1` ok, `2` error.

<p class="api-label">example</p>

```bash
curl -sSG 'http://127.0.0.1:8080/api/traces' \
  --data-urlencode 'start=2026-08-26T00:00:00Z' \
  --data-urlencode 'end=2026-08-27T00:00:00Z' \
  --data-urlencode 'limit=50' \
  --data-urlencode 'status=error' \
  --data-urlencode 'service=checkout'
```

### One trace

<p class="api-sig"><span class="api-verb">GET</span>/api/traces/{id}</p>

One trace: span tree, attributes, events, links, critical path, bottlenecks.

<p class="api-label">parameters</p>

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| `id` | path | hex | yes | 1–32 hex characters |
| `start` | query | RFC3339 | no | Must be set with `end` |
| `end` | query | RFC3339 | no | Omit both for `[now − max, now]` |

<p class="api-label">response</p>

<p class="api-status">200</p>

```json
{
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "timestamp": "2026-08-26T12:00:00Z",
  "duration_ns": 412000000,
  "span_count": 13,
  "critical_path_ns": 390000000,
  "critical_path": [
    { "span_id": "a", "service": "gateway", "operation": "HTTP POST /pay", "duration_ns": 412000000 }
  ],
  "bottlenecks": [
    { "span_id": "b", "service": "checkout", "operation": "db.query", "exclusive_ns": 210000000 }
  ],
  "spans": [
    {
      "timestamp": "2026-08-26T12:00:00Z",
      "span_id": "a1",
      "parent_span_id": "",
      "service": "gateway",
      "operation": "HTTP POST /pay",
      "kind": 2,
      "duration_ns": 412000000,
      "status_code": 2,
      "status_message": "",
      "scope_name": "",
      "scope_version": "",
      "resource_attributes": {},
      "span_attributes": {},
      "events": [{ "time": "2026-08-26T12:00:00.1Z", "name": "exception", "attributes": {} }],
      "links": [{ "trace_id": "…", "span_id": "…", "attributes": {} }]
    }
  ]
}
```

<p class="api-status">404</p>

```json
{ "error": "trace not found" }
```

<p class="api-label">example</p>

```bash
curl -sS 'http://127.0.0.1:8080/api/traces/4bf92f3577b34da6a3ce929d0e0e4736'
```

### Live stream

<p class="api-sig"><span class="api-verb">WS</span>/api/stream/traces</p>

After each successful trace ingest, one JSON object. Slow clients are dropped. Caps: [Self-hosting](self-hosting.md).

<p class="api-label">parameters</p>

<p class="api-none">None.</p>

<p class="api-label">response</p>

One message per ingest (same shape as a search row):

```json
{
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "service": "checkout",
  "operation": "HTTP POST /pay",
  "duration_ns": 412000000,
  "span_count": 13,
  "timestamp": "2026-08-26T12:00:00Z",
  "status_code": 2
}
```

<p class="api-status">503</p>

`RASAT_STREAM_MAX_CLIENTS` exhausted.

<p class="api-label">example</p>

```bash
websocat ws://127.0.0.1:8080/api/stream/traces
```

## Logs

### Search logs

<p class="api-sig"><span class="api-verb">GET</span>/api/logs</p>

Search log records in a window.

<p class="api-label">parameters</p>

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| `start` | query | RFC3339 | yes | Window start |
| `end` | query | RFC3339 | yes | Window end |
| `limit` | query | int | yes | Max rows, 1–1000 |
| `service` | query | string | no | Exact service name |
| `level` | query | string | no | `INFO`, `WARN`/`WARNING`, `ERROR`/`ERR`, … |
| `trace_id` | query | string | no | Max 128 chars. Alias: `trace` |
| `span_id` | query | string | no | Max 32 chars |

<p class="api-label">response</p>

<p class="api-status">200</p>

```json
{
  "logs": [
    {
      "timestamp": "2026-08-26T12:00:00Z",
      "service": "checkout",
      "level": "ERROR",
      "message": "database timeout",
      "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
      "span_id": "a1"
    }
  ]
}
```

<p class="api-label">example</p>

```bash
curl -sSG 'http://127.0.0.1:8080/api/logs' \
  --data-urlencode 'start=2026-08-26T00:00:00Z' \
  --data-urlencode 'end=2026-08-27T00:00:00Z' \
  --data-urlencode 'limit=50' \
  --data-urlencode 'level=ERROR'
```

### Live logs

<p class="api-sig"><span class="api-verb">WS</span>/api/stream/logs</p>

One JSON log row per ingest. Same backpressure as the traces stream.

<p class="api-label">parameters</p>

<p class="api-none">None.</p>

<p class="api-label">response</p>

One message per ingest (same shape as a search row):

```json
{
  "timestamp": "2026-08-26T12:00:00Z",
  "service": "checkout",
  "level": "ERROR",
  "message": "database timeout",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "a1"
}
```

<p class="api-status">503</p>

client cap exhausted.

<p class="api-label">example</p>

```bash
websocat ws://127.0.0.1:8080/api/stream/logs
```

## Catalog

`limit` is required. `start` and `end` are optional together.

### Services

<p class="api-sig"><span class="api-verb">GET</span>/api/services</p>

Discovered services in the window.

<p class="api-label">parameters</p>

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| `limit` | query | int | yes | Max rows, 1–1000 |
| `start` | query | RFC3339 | no | Must be set with `end` |
| `end` | query | RFC3339 | no | Omit both for `[now − max, now]` |

<p class="api-label">response</p>

<p class="api-status">200</p>

```json
{
  "services": [
    {
      "service": "checkout",
      "last_seen": "2026-08-26T12:00:00Z",
      "spans": 4120,
      "errors": 18
    }
  ]
}
```

<p class="api-label">example</p>

```bash
curl -sSG 'http://127.0.0.1:8080/api/services' --data-urlencode 'limit=50'
```

### Operations

<p class="api-sig"><span class="api-verb">GET</span>/api/operations</p>

Operations for one service: volume, error rate, p50, p95.

<p class="api-label">parameters</p>

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| `service` | query | string | yes | Service name |
| `limit` | query | int | yes | Max rows, 1–1000 |
| `start` | query | RFC3339 | no | Must be set with `end` |
| `end` | query | RFC3339 | no | Omit both for `[now − max, now]` |

<p class="api-label">response</p>

<p class="api-status">200</p>

```json
{
  "operations": [
    {
      "operation": "HTTP POST /pay",
      "spans": 800,
      "errors": 12,
      "error_rate": 0.015,
      "p50_ns": 42000000,
      "p95_ns": 180000000
    }
  ]
}
```

<p class="api-label">example</p>

```bash
curl -sSG 'http://127.0.0.1:8080/api/operations' \
  --data-urlencode 'service=checkout' \
  --data-urlencode 'limit=50'
```

### Service map

<p class="api-sig"><span class="api-verb">GET</span>/api/service-map</p>

Nodes and directed edges.

<p class="api-label">parameters</p>

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| `limit` | query | int | yes | Max nodes/edges bound, 1–1000 |
| `start` | query | RFC3339 | no | Must be set with `end` |
| `end` | query | RFC3339 | no | Omit both for `[now − max, now]` |

<p class="api-label">response</p>

<p class="api-status">200</p>

```json
{
  "nodes": [
    { "service": "gateway", "spans": 900, "errors": 4 }
  ],
  "edges": [
    {
      "from": "gateway",
      "to": "checkout",
      "calls": 800,
      "errors": 3,
      "avg_duration_ns": 51000000
    }
  ]
}
```

<p class="api-label">example</p>

```bash
curl -sSG 'http://127.0.0.1:8080/api/service-map' --data-urlencode 'limit=50'
```

## Analytics

Same window rules as catalog.

### Metrics

<p class="api-sig"><span class="api-verb">GET</span>/api/metrics</p>

Per-service rate, error rate, avg, p50, p95, p99. Optional bucketed series when `step` is set.

<p class="api-label">parameters</p>

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| `limit` | query | int | yes | Max services, 1–1000 |
| `start` | query | RFC3339 | no | Must be set with `end` |
| `end` | query | RFC3339 | no | Omit both for `[now − max, now]` |
| `service` | query | string | no | Restrict to one name |
| `step` | query | duration | no | ≥ `1s` (e.g. `1m`). Fills `series` |

<p class="api-label">response</p>

<p class="api-status">200</p>

```json
{
  "window_s": 86400,
  "step_s": 600,
  "metrics": [
    {
      "service": "checkout",
      "spans": 4120,
      "errors": 18,
      "rate": 0.047,
      "error_rate": 0.0044,
      "avg_ns": 52000000,
      "p50_ns": 41000000,
      "p95_ns": 180000000,
      "p99_ns": 410000000
    }
  ],
  "series": [
    {
      "service": "checkout",
      "points": [
        {
          "t": "2026-08-26T12:00:00Z",
          "spans": 40,
          "errors": 1,
          "rate": 0.067,
          "error_rate": 0.025,
          "avg_ns": 50000000,
          "p50_ns": 40000000,
          "p95_ns": 120000000,
          "p99_ns": 200000000
        }
      ]
    }
  ]
}
```

`series` is empty if `step` is omitted.

<p class="api-label">example</p>

```bash
curl -sSG 'http://127.0.0.1:8080/api/metrics' \
  --data-urlencode 'start=2026-08-25T12:00:00Z' \
  --data-urlencode 'end=2026-08-26T12:00:00Z' \
  --data-urlencode 'limit=1' \
  --data-urlencode 'service=checkout' \
  --data-urlencode 'step=10m'
```

### Error causes

<p class="api-sig"><span class="api-verb">GET</span>/api/error-causes</p>

Top error causes in the window. Fleet-wide if `service` is omitted.

<p class="api-label">parameters</p>

| Name | In | Type | Required | Description |
|---|---|---|---|---|
| `limit` | query | int | yes | Max rows, 1–1000 |
| `start` | query | RFC3339 | no | Must be set with `end` |
| `end` | query | RFC3339 | no | Omit both for `[now − max, now]` |
| `service` | query | string | no | Restrict to one name |

<p class="api-label">response</p>

<p class="api-status">200</p>

```json
{
  "causes": [
    {
      "cause": "connection reset",
      "count": 12,
      "first_seen": "2026-08-26T11:02:00Z"
    }
  ]
}
```

<p class="api-label">example</p>

```bash
curl -sSG 'http://127.0.0.1:8080/api/error-causes' \
  --data-urlencode 'limit=8'
```
