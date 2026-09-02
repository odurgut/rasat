---
title: Demo data and load
description: Fill a local Rasat with synthetic traces, or measure ingest on your own hardware.
---

# Demo data and load

`rasat-seed` and `rasat-bench` are **separate commands**. They are not in the Rasat container. They speak OTLP/HTTP at a process that is already up — the same way an SDK would.

Jaeger ships HotROD the same way: demo traffic is an app you point at the collector, not a sidecar inside the server image. Load generators (k6, vegeta, project-specific benches) stay out of production images.

Need a real service instead? [Send data](send-data.md).

## Demo traces (`rasat-seed`)

Rasat is running (`http://localhost:8080/ready` is **200**). From a checkout:

```bash
make seed        # one batch of shop traces and correlated logs
make seed-live   # keep posting until Ctrl-C
```

Defaults: OTLP `http://127.0.0.1:8080/v1/traces`. Override with `-url` or `RASAT_SEED_URL`.

Overview, traces, logs, and the map should populate. This is for a laptop, not a shared cluster.

## Load (`rasat-bench`)

Measures ingest spans/s and search/detail latency against **your** machine. Printed numbers are not a product contract.

```bash
make bench
```

Default is about 10 000 spans/s for 20 seconds against `http://127.0.0.1:8080`. Flags: `-spans`, `-duration`, `-url` (`RASAT_BENCH_URL`). Live UI publish is still capped (`RASAT_STREAM_MAX_PER_SEC`); ingest writes every accepted span.

Do not point this at a deployment you care about unless you intend to flood it.
