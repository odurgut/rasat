---
title: Compatibility
description: OpenTelemetry, ClickHouse, OS, and Docker versions Rasat supports — and what we have not qualified.
---

# Compatibility

This page is the matrix for **this documentation set** — the Rasat you get from the matching git checkout or image. There is no older “Rasat 0.x supports CH 23” archive yet. A combination not listed as **supported** is not a Rasat bug until we qualify it.

| | |
|---|---|
| **Supported** | We run this. Breaks are our problem. |
| **Best effort** | We do not test it. We may tell you to use the supported line. |
| **Not supported** | Out of scope. |

No login, no TLS. That is not a version matrix; see [Current limits](limits.md).

## ClickHouse

| | |
|---|---|
| **Supported** | ClickHouse **24.8**, native protocol **9000**. Official image `clickhouse/clickhouse-server:24.8` is one way to get that; any 24.8 you operate is in scope. |
| **Best effort** | Other 24.8 patches; 24.3 LTS if native protocol and our DDL work. |
| **Not supported** | 23 and older; connecting Rasat over HTTP 8123; ClickHouse Cloud; cluster recipes we do not ship. |

Rasat uses native protocol only (`RASAT_CLICKHOUSE_ADDR`). Schema is created on startup for 24.8. This version does not migrate older Rasat databases. You run ClickHouse; [Self-hosting](self-hosting.md).

## OpenTelemetry traces

Rasat is an OTLP **traces** backend. It is not an SDK and not a Collector.

| | |
|---|---|
| **Supported** | OTLP traces **Export**: HTTP `POST /v1/traces` (default port 8080) and gRPC `TraceService/Export` (4317). |
| HTTP | `application/x-protobuf` / `application/protobuf` (empty `Content-Type` = protobuf) and `application/json`. `Content-Encoding: gzip`. |
| **Best effort** | SDKs and Collectors that emit that Export. We do not pin language minors. |
| **Not supported** | OTLP logs, OTLP metrics, profiles, Jaeger/Zipkin ingest. |

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:8080
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
```

gRPC: port `4317`, `OTEL_EXPORTER_OTLP_PROTOCOL=grpc`. Collector: `otlp` or `otlphttp` to those endpoints. **Collector v0.96+** (including 1.x) is the intended range. [Send data](send-data.md).

Decode is `go.opentelemetry.io/collector/pdata` v1.36.0. Bad protobuf/JSON is HTTP 400. Oversized batches are rejected (16 MiB body default).

## Rasat and Docker

| | |
|---|---|
| **Build** | Go **1.24.x**, Node **22** (UI from this tree). |
| **Image** | Linux static binary, non-root. Docker Hub `odurgut/rasat` (`0.1.1`, `0.1`, `latest` from the git tag — not from `main`). `linux/amd64` and `linux/arm64`. |
| **Arch** | Published image: both architectures. From-source image build: the machine you build on. |
| **Host binary** | `make build` on Linux or macOS + Go 1.24. Windows: Docker or WSL. |
| **Docker** | Engine. Compose V2 is optional (laptop convenience stack only). |
| **Not supported** | Kubernetes/Helm as a shipped install. |

Apple Silicon uses the arm64 Hub image. `make compose-build` builds for the host if you want an image from this tree.

## Browsers

Current Chrome, Firefox, Safari, and Edge. The UI uses WebSockets. Internet Explorer is not supported.

The supported pair is ClickHouse 24.8 + OTLP traces to 8080/4317. [Getting started](getting-started.md).
