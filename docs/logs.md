---
title: Logs
description: Live structured logs and jumping to traces.
---

# Logs

Logs is a **live tail** of structured records Rasat has ingested — closer to `tail -f` than to a log-file explorer. It is not syslog and not OTLP logs (yet).

Send JSON as described in [Send data](send-data.md). Empty UI means nothing has been posted to `/api/logs`.

## Filters

| Field | Meaning |
|---|---|
| **service** | Emitting service |
| **level** | e.g. `ERROR`, `WARN`, `INFO` |
| **trace** | Trace id |
| **start** / **end** | Time window |
| **limit** | Max rows |

**Search** applies the filter. **Reset** clears it.

Level color applies to the **ERROR** / **WARN** / **INFO** word only, not the whole line.

## Live follow

When the window ends at “now”, new matching logs prepend as they arrive. Scroll or hover to pause; **N new** counts the backlog. The same drop-and-cap rules as traces apply: ingest never waits on this page.

## Selection and jumps

Click a line for detail. If `trace_id` is present, jump to that [trace](traces.md). Service names can open the service dashboard.

From a waterfall inspector **logs** tab you can jump the other way: the logs page opens already filtered to that trace.

## Correlation

Rasat does not stitch logs to spans by magic. Set `trace_id` (and optionally `span_id`) in the JSON you send. That is the contract between your logger and this UI.
