<p align="center">
  <img src="docs/images/logo.svg#gh-light-mode-only" width="150" height="65" alt="Rasat">
  <img src="docs/images/logo-dark.svg#gh-dark-mode-only" width="150" height="65" alt="Rasat">
</p>

# Rasat

[![CI](https://github.com/odurgut/rasat/actions/workflows/ci.yml/badge.svg)](https://github.com/odurgut/rasat/actions/workflows/ci.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/odurgut/rasat)](go.mod)
[![Docker](https://img.shields.io/docker/v/odurgut/rasat/latest?label=docker)](https://hub.docker.com/r/odurgut/rasat)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

Self-hosted observability for **OpenTelemetry traces**, structured **logs**, and a **service map**. One process serves the UI, ingest, and query. **Your ClickHouse** stores the data.

**Website and docs:** [https://rasat.dev](https://rasat.dev) · [docs/](docs/index.md) · [compatibility](docs/compatibility.md)

This version has **no login**. Anyone who can reach the HTTP port can query and ingest. Bind it to a network you trust. What is missing: [Current limits](docs/limits.md).

## Run

The image is [`odurgut/rasat`](https://hub.docker.com/r/odurgut/rasat). Point it at ClickHouse you run (native protocol, port 9000):

```bash
docker run -d --name rasat \
  -p 8080:8080 -p 4317:4317 \
  -e RASAT_CLICKHOUSE_ADDR=host.docker.internal:9000 \
  -e RASAT_CLICKHOUSE_DATABASE=rasat \
  -e RASAT_CLICKHOUSE_USER=rasat \
  -e RASAT_CLICKHOUSE_PASSWORD=rasat \
  odurgut/rasat:0.1.1
```

Open `http://localhost:8080` when `http://localhost:8080/ready` is **200**. OTLP/HTTP is that port; OTLP/gRPC is `localhost:4317`.

Walkthrough (including a throwaway ClickHouse and an optional Compose stack): [Getting started](docs/getting-started.md). How you run ClickHouse, health, and scale: [Self-hosting](docs/self-hosting.md). Variables: [Configuration](docs/configuration.md).

## Send traces

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:8080
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
```

[Send data](docs/send-data.md) covers gRPC, the Collector, and logs. No Rasat agent.

## Contribute

[CONTRIBUTING.md](CONTRIBUTING.md): `main` only, semver tags, CI, and how a release reaches Docker Hub.

## License

[Apache License 2.0](LICENSE).
