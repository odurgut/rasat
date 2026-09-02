---
title: Send data
description: Export OpenTelemetry traces and structured logs to Rasat.
---

# Send data

Rasat does not ship a language-specific agent. If a library can export **OTLP traces**, it can send them here. Protocol and Collector floors: [Compatibility](compatibility.md).

## Traces: OTLP/HTTP

POST traces to the HTTP listener (default port **8080**):

```text
POST /v1/traces
```

Accepted content types: `application/x-protobuf` and `application/json`. Gzip (`Content-Encoding: gzip`) is supported.

Environment variables used by most OpenTelemetry SDKs:

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:8080
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
```

Some SDKs append `/v1/traces` themselves. In that case the endpoint is the origin only (`http://127.0.0.1:8080`). If a request 404s, check whether the path was doubled.

Request bodies are size-capped (16 MiB by default). Split large batches.

## Traces: OTLP/gRPC

The gRPC listener defaults to port **4317** (`TraceService/Export`). The stored schema is the same as HTTP.

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317
OTEL_EXPORTER_OTLP_PROTOCOL=grpc
```

Use this when the Collector or SDK already speaks gRPC, or when you want ingest off the UI port.

## OpenTelemetry Collector

Rasat is a **backend**. Sampling, batching, tail sampling, and attribute processing belong in the Collector (or the SDK) before export.

gRPC exporter:

```yaml
exporters:
  otlp/rasat:
    endpoint: rasat:4317
    tls:
      insecure: true

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlp/rasat]
```

HTTP exporter:

```yaml
exporters:
  otlphttp/rasat:
    endpoint: http://rasat:8080
```

Replace `rasat` with a hostname the Collector can resolve. From a laptop toward Rasat on localhost, use `127.0.0.1` and published ports.

TLS is not terminated by Rasat in this version. Put a proxy in front if you need HTTPS, or keep Collector → Rasat on a private network.

## What to put on spans

Rasat is more useful when spans carry:

- A stable **service name** (resource `service.name`)
- Meaningful **span names** (route or operation, not a raw URL with ids if you want to group)
- **Error status** on failures (`Unset` vs `Error`)
- Attributes you will search later (`http.method`, `http.route`, customer-facing ids you are allowed to store)

The trace search **attr** filter is `key=value` on span attributes.

## Structured logs

There is no OTLP logs receiver yet. Send JSON to:

```text
POST /api/logs
Content-Type: application/json
```

The body is one object or an array (up to 10 000 items). Gzip is accepted.

| Field | Required | Description |
|---|---|---|
| `service` | yes | Emitting service |
| `timestamp` | no | RFC3339; receive time if omitted |
| `level` | no | e.g. `INFO`, `WARN`, `ERROR` |
| `message` | no | Truncated if extremely long |
| `trace_id` | no | Correlates with a trace |
| `span_id` | no | Optional finer correlation |

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

Include `trace_id` whenever the log line is about a traced request. That is what fills the inspector **logs** tab and enables jump-to-trace on the logs page.

## After a successful write

Data is persisted, then a compact event is published to live UI subscribers. Ingest does not wait for browsers. See [Concepts](concepts.md) (live data) and [Self-hosting](self-hosting.md) (caps).
