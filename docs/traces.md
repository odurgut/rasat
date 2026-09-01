---
title: Traces
description: Search traces, follow them live, and inspect a waterfall.
---

# Traces

Traces is where you **find a request** and **see how it ran**.

The page has two modes: a **table** of matching traces, then a dedicated **waterfall** for one id. Filters stay in the URL so a search is shareable.

## Search

Required context: a **start** and **end** time and a **limit** (how many rows). Defaults cover roughly the last 24 hours with limit 50.

| Field | What it does |
|---|---|
| **service** | Exact service name (picker from discovered services) |
| **operation** | Span name (picker scoped to the selected service) |
| **min** | Minimum duration, e.g. `50ms`, `1s` |
| **status** | `error`, `ok`, or `unset` |
| **start** / **end** | RFC3339 window |
| **limit** | Max rows, 1–1000 |

**Search** runs the query. **Reset** returns to defaults.

The API also supports an attribute filter (`attr=key=value`) and a trace id. The form is the usual path; arbitrary attributes are available over the [HTTP API](api.md).

The window cannot exceed the configured maximum (7 days by default). `end` must be after `start`.

## The table

Columns: **time**, **service**, **operation**, **spans**, **duration**, **id**.

Error traces are marked **ERR**. Click a row to open the waterfall.

While **end** is “now”, the table can **follow ingest**: new traces that match the current filters appear at the top. Scroll or hover to pause; **N new** counts what you missed. Historical windows do not tail.

## Waterfall

The waterfall is a timeline of every span in the trace:

- A **tree** of parent/child operations
- Horizontal **bars** for duration
- Error spans highlighted
- Truncated names expand on hover

Click a span to open the **inspector**. The waterfall stays; the inspector is extra context, not a replacement for the tree.

**Back** returns to the table without losing the search.

### Critical path and bottlenecks

On the trace, Rasat highlights the **critical path** (the chain of spans that explain end-to-end time) and **bottlenecks** (spans whose exclusive / self time dominates). Off-path work is visually quieter.

Click a path step or a bottleneck to select that span and **scroll** its row into view.

### Inspector

Header: operation name, service, timing, kind.

Tabs:

| Tab | Content |
|---|---|
| **details** | Span attributes, resource attributes, links to other traces |
| **events** | Events on the span (including exceptions) |
| **logs** | Structured logs with this **trace id** in the same window |

Service names and operations are jump targets: open the service dashboard or related logs. Linked traces open in place.

Related logs only appear if you [send logs](send-data.md) with `trace_id` set. Traces alone do not invent log lines.

## From other pages

Overview activity, service operations, error causes, map nodes, and log lines can all open this view with filters or a trace id already applied. You should not have to re-type the window.
