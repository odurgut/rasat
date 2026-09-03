<p align="center">
  <img src="https://raw.githubusercontent.com/odurgut/rasat/main/docs/images/logo.svg" width="150" height="65" style="width:150px;height:65px" alt="Rasat">
</p>

# Rasat

Self-hosted observability for **OpenTelemetry traces**, structured **logs**, and a **service map**. One process serves the UI, ingest, and query. **Your ClickHouse** stores the data.

**Website:** [https://rasat.dev](https://rasat.dev) · **Docs:** [getting started](https://rasat.dev/docs/getting-started) · **Source:** [github.com/odurgut/rasat](https://github.com/odurgut/rasat)

This image is **linux/amd64** and **linux/arm64**, distroless, non-root. It does **not** include ClickHouse, `rasat-seed`, or `rasat-bench`.

This version has **no login**. Anyone who can reach the HTTP port can query and ingest. Bind it to a network you trust.

## Quick start

You need ClickHouse **24.8** on native port **9000**. Point Rasat at it:

```bash
docker run -d --name rasat \
  -p 8080:8080 -p 4317:4317 \
  -e RASAT_CLICKHOUSE_ADDR=host.docker.internal:9000 \
  -e RASAT_CLICKHOUSE_DATABASE=rasat \
  -e RASAT_CLICKHOUSE_USER=rasat \
  -e RASAT_CLICKHOUSE_PASSWORD=rasat \
  odurgut/rasat:0.1.1
```

On Linux, add `--add-host=host.docker.internal:host-gateway` if ClickHouse is on the host. Wait until `http://localhost:8080/ready` returns **200**. UI: `http://localhost:8080`.

Throwaway ClickHouse, Compose, OTLP examples: [getting started](https://rasat.dev/docs/getting-started).

## Ports

| Port | Role |
|---|---|
| 8080 | UI, OTLP/HTTP, query API, log ingest, `/health`, `/ready` |
| 4317 | OTLP/gRPC |

## Configuration

Environment only. There is no config file. Match `RASAT_CLICKHOUSE_*` to **your** server.

| Variable | Default | Description |
|---|---|---|
| `RASAT_HTTP_ADDR` | `:8080` | HTTP listen |
| `RASAT_GRPC_ADDR` | `:4317` | OTLP/gRPC listen |
| `RASAT_CLICKHOUSE_ADDR` | `127.0.0.1:9000` | Native `host:port` |
| `RASAT_CLICKHOUSE_DATABASE` | `rasat` | Created on startup |
| `RASAT_CLICKHOUSE_USER` | `default` | Must be able to create the database and tables |
| `RASAT_CLICKHOUSE_PASSWORD` | empty | Match the ClickHouse user |

Full list: [configuration](https://rasat.dev/docs/configuration).

## Tags

Published from git tags `vMAJOR.MINOR.PATCH` (not from `main`):

| Tag | Meaning |
|---|---|
| `0.1.1` | This release |
| `0.1` | Latest `0.1.x` |
| `latest` | Latest tagged release |

## Send traces

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:8080
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
```

No Rasat agent. [Send data](https://rasat.dev/docs/send-data).

## License

[Apache License 2.0](https://github.com/odurgut/rasat/blob/main/LICENSE).
