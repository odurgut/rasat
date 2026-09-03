---
title: Getting started
description: Run Rasat against ClickHouse, open the UI, and send OpenTelemetry traces.
---

# Getting started

Rasat is **one process** (image or binary). It reads and writes **your ClickHouse**. We do not ship, host, or operate ClickHouse — point Rasat at a server you already run, or start one yourself. Versions: [Compatibility](compatibility.md). Ports, replicas, proxies: [Self-hosting](self-hosting.md). Every knob: [Configuration](configuration.md).

This version has **no login**. Anyone who can reach the HTTP port can query and ingest.

| Address | Role |
|---|---|
| `http://localhost:8080` | UI, OTLP/HTTP, query API |
| `localhost:4317` | OTLP/gRPC |

## ClickHouse

Native protocol on **9000**. Rasat creates its schema in `RASAT_CLICKHOUSE_DATABASE` on startup. Use a user that can do that. **24.8** is what we support.

If you already have ClickHouse, skip to [Run Rasat](#run-rasat) and set `RASAT_CLICKHOUSE_*` to that server.

If you need a throwaway instance for this walkthrough:

```bash
docker run -d --name clickhouse \
  --ulimit nofile=262144:262144 \
  -p 9000:9000 \
  -e CLICKHOUSE_DB=rasat \
  -e CLICKHOUSE_USER=rasat \
  -e CLICKHOUSE_PASSWORD=rasat \
  -e CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT=1 \
  clickhouse/clickhouse-server:24.8
```

That is the official ClickHouse image, not Rasat. Run ClickHouse however you prefer (package, VM, existing cluster) as long as native 9000 is reachable.

## Run Rasat

Image: [`odurgut/rasat`](https://hub.docker.com/r/odurgut/rasat). Match user, password, database, and address to **your** ClickHouse.

```bash
docker run -d --name rasat \
  -p 8080:8080 -p 4317:4317 \
  -e RASAT_CLICKHOUSE_ADDR=host.docker.internal:9000 \
  -e RASAT_CLICKHOUSE_DATABASE=rasat \
  -e RASAT_CLICKHOUSE_USER=rasat \
  -e RASAT_CLICKHOUSE_PASSWORD=rasat \
  odurgut/rasat:0.1.1
```

On Linux, add `--add-host=host.docker.internal:host-gateway` if the ClickHouse port is on the host. If both containers share a Docker network, use the ClickHouse service hostname instead of `host.docker.internal`. Same variables on a host binary (`make build` then `bin/rasat`).

Open `http://localhost:8080`. `http://localhost:8080/ready` returns **200** when ClickHouse accepts connections.

## Quick stack (optional)

Compose that starts ClickHouse **and** Rasat is a convenience for a laptop, not the product. Skip it if you already run ClickHouse.

```yaml
services:
  clickhouse:
    image: clickhouse/clickhouse-server:24.8
    environment:
      CLICKHOUSE_DB: rasat
      CLICKHOUSE_USER: rasat
      CLICKHOUSE_PASSWORD: rasat
      CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT: 1
    volumes:
      - clickhouse-data:/var/lib/clickhouse
    ulimits:
      nofile:
        soft: 262144
        hard: 262144
    healthcheck:
      test:
        [
          "CMD-SHELL",
          "clickhouse-client --user rasat --password rasat --query 'SELECT 1'",
        ]
      interval: 5s
      timeout: 3s
      retries: 12
      start_period: 20s

  rasat:
    image: odurgut/rasat:0.1.1
    ports:
      - "8080:8080"
      - "4317:4317"
    environment:
      RASAT_CLICKHOUSE_ADDR: clickhouse:9000
      RASAT_CLICKHOUSE_DATABASE: rasat
      RASAT_CLICKHOUSE_USER: rasat
      RASAT_CLICKHOUSE_PASSWORD: rasat
    depends_on:
      clickhouse:
        condition: service_healthy

volumes:
  clickhouse-data:
```

```bash
docker compose up -d
```

Default passwords in these examples are for a **private** machine. Change them before this is on a network you do not trust.

## Open the product

Left rail: **overview**, **traces**, **services**, **logs**, **map**. Overview is empty until traces arrive. That is expected.

No application yet: [demo data](demo-and-load.md) posts synthetic OTLP at the same endpoint. Dark and light themes are in the rail footer. The preference stays in the browser.

## Send traces from an application

Rasat speaks **OpenTelemetry Protocol**. Point any OTLP-compatible SDK or Collector at it. You do not install a Rasat agent.

**OTLP/HTTP** (same host as the UI):

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:8080
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
```

**OTLP/gRPC** (default OpenTelemetry port):

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317
export OTEL_EXPORTER_OTLP_PROTOCOL=grpc
```

If the app is another container, use a hostname it can resolve (`http://rasat:8080` or `rasat:4317`), not `127.0.0.1`.

Restart the app (or Collector) so the exporter picks up the endpoint. Generate traffic. On **overview**, KPIs and the activity feed should start moving. On **traces**, search the last hour and open a row.

A full Collector example and log ingest are in [Send data](send-data.md).

## Confirm ingest without the UI

```bash
curl -sS http://127.0.0.1:8080/health    # process is up
curl -sS http://127.0.0.1:8080/ready     # storage is reachable
curl -sS http://127.0.0.1:8080/version   # image tag
```

`/health` does not talk to ClickHouse. Use `/ready` in orchestration so you do not send OTLP at a process that cannot write.

## Next

- [Concepts](concepts.md) — what a trace, a log line, and a derived metric mean in Rasat
- [Overview](overview.md) — how to read the landing page
- [Traces](traces.md) — search and waterfall
- [Self-hosting](self-hosting.md) — ClickHouse you operate, health, scale
