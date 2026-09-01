---
title: Service map
description: Automatic dependency graph from parent and child spans.
---

# Service map

The map draws **who calls whom** from traces in the current window. You do not maintain a topology file. If service A has a child span on service B, there is an edge `A → B`.

Open **map** in the rail. The graph is built from the same span store as search.

## Reading the graph

- **Nodes** are services. Volume and error share come from spans on that service. When error rate is above zero, the node can show an **ERR** percentage. **P95** on a selected node comes from derived metrics for that service.
- **Edges** are directed calls. Thickness and meter reflect the selected mode.

Two modes:

| Mode | Emphasis |
|---|---|
| **calls** | Call volume |
| **errors** | Error share on the edge |

Hover an edge for a tip: call volume, average latency, error percentage.

Select an edge to pin latency for that path. Select a node to see its neighbors, error rate, and p95.

## Inspector

With a node selected:

- Incoming and outgoing neighbors
- Jump to that service’s [dashboard](services.md)
- Jump into [traces](traces.md) for the service

The map is a way to **orient**; dashboards and waterfalls are where you finish an investigation.

## What the map cannot show

- Services that never appear as a span `service.name`
- Calls that your instrumentation does not parent correctly (missing or wrong parent span id)
- Async edges that you only express as span **links** (links are visible on the waterfall, not as map edges)
- Traffic outside the selected time window

Fixing the graph usually means fixing tracing, not clicking “add node.”
