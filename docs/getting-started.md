---
title: Getting started
description: Run Rasat, open the UI, and send OpenTelemetry traces from your application.
---

# Getting started

This walkthrough gets you from nothing to **traces on the screen**. It assumes Docker is available. For other layouts, see [Self-hosting](self-hosting.md).

## What you will have

Two containers:

1. **ClickHouse** — stores spans, events, links, and logs.
2. **Rasat** — receives telemetry, queries storage, and serves the UI.

The UI, OTLP/HTTP, the query API, and live WebSockets share one HTTP port. OTLP/gRPC uses a second port.

| Address | Role |
|---|---|
| `http://localhost:8080` | UI and HTTP APIs |
| `localhost:4317` | OTLP/gRPC |

## Start Rasat

Use the Compose file published with Rasat (`deploy/docker-compose.yml`). From a checkout of the project:

```bash
make compose-up
```

That is Compose plus a git-stamped binary. `GET /version` and the UI rail show `git describe`. Plain `docker compose -f deploy/docker-compose.yml up --build` without `VERSION` / `COMMIT` reports `dev`.

Wait until Rasat is ready. `http://localhost:8080/ready` returns **200** when ClickHouse accepts connections. The UI is `http://localhost:8080`.

If ClickHouse was previously started with different credentials, remove the old volume once (`docker compose -f deploy/docker-compose.yml down -v`) and start again. The published Compose file uses a dedicated ClickHouse user for Rasat.

## Open the product

In the browser you should see a left rail: **overview**, **traces**, **services**, **logs**, **map**. Overview is empty until traces arrive. That is expected.

No application yet: [demo data](demo-and-load.md) posts synthetic OTLP at the same endpoint (`make seed`). Dark and light themes are in the rail footer. The preference stays in the browser.

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

If the process runs in another container on the same Compose network, use the Rasat service hostname (`rasat:4317` or `http://rasat:8080`), not `127.0.0.1`.

Restart the app (or Collector) so the exporter picks up the endpoint. Generate traffic. On **overview**, KPIs and the activity feed should start moving. On **traces**, search the last hour and open a row.

A full Collector example and log ingest are in [Send data](send-data.md).

## Confirm ingest without the UI

```bash
curl -sS http://127.0.0.1:8080/health    # process is up
curl -sS http://127.0.0.1:8080/ready     # storage is reachable
curl -sS http://127.0.0.1:8080/version   # git-stamped build
```

`/health` does not talk to ClickHouse. Use `/ready` in orchestration so you do not send OTLP at a process that cannot write.

## Next

- [Concepts](concepts.md) — what a trace, a log line, and a derived metric mean in Rasat
- [Overview](overview.md) — how to read the landing page
- [Traces](traces.md) — search and waterfall
- [Self-hosting](self-hosting.md) — running this beyond a laptop
