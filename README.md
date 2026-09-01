# Rasat

Self-hosted observability for **OpenTelemetry traces**, structured **logs**, and a **service map**. One process serves the UI, ingest, and query. ClickHouse stores the data.

**Website and docs:** [https://rasat.dev](https://rasat.dev) · this tree: [docs/](docs/index.md)

This version has **no login**. Anyone who can reach the HTTP port can query and ingest. Bind it to a network you trust. What is missing: [Current limits](docs/limits.md).

## Run

Docker is required. From a checkout:

```bash
make compose-up
```

Wait until `http://localhost:8080/ready` returns **200**. UI: `http://localhost:8080`. OTLP/HTTP is the same port; OTLP/gRPC is `localhost:4317`.

Full walkthrough: [Getting started](docs/getting-started.md). Deploy notes: [Self-hosting](docs/self-hosting.md).

## Send traces

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:8080
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
```

[Send data](docs/send-data.md) covers gRPC, the Collector, and logs. No Rasat agent.

## License

[Apache License 2.0](LICENSE).
