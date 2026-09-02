---
title: Current limits
description: What this version of Rasat does not include, and why it matters operationally.
---

# Current limits

Rasat is usable for tracing, live exploration, logs with correlation, service dashboards, and a map. This page is the product boundary — not a bug list.

## Security

There is **no login**, API token, RBAC, or audit log. Anyone who can reach the HTTP port can read telemetry and send OTLP or logs. Bind to a private network, VPN, or authenticating proxy until auth ships.

TLS is not provided by Rasat. Terminate TLS at a reverse proxy if you need it.

## Multi-tenancy

One deployment is one data set. There are no tenants, no per-team isolation, and no per-tenant retention.

## Alerting

Overview issues and regressions are **on-screen triage**. There are no alert rules, no Slack/email/webhooks, and no notification policy.

## Ingest

- **Traces:** OTLP/HTTP and OTLP/gRPC. Versions: [Compatibility](compatibility.md).
- **Logs:** JSON `POST /api/logs` only. No OTLP logs, no file tail, no syslog.
- **Metrics:** Not ingested. Dashboards aggregate **spans**.

## Deployment

The documented install is the **Rasat process** (image or binary) pointed at **ClickHouse you run**. There is no official Kubernetes chart, operator, or Helm release.

The live UI stream lives **inside one Rasat process**. Do not scale Rasat replicas expecting a shared live feed. Ingest and historical search can still work with more than one replica; live overview / traces / logs will not.

## Retention and HA

Rasat does not expire data. ClickHouse availability **is** Rasat availability for writes and queries. There is no built-in ClickHouse cluster recipe in this version.

## Product choices you will not find

- Side-by-side trace compare
- Custom dashboard builder
- SLO objects
- A Rasat-specific SDK

If a workflow needs one of the missing pieces, keep using your existing tool for that slice and Rasat for trace-centric investigation — or wait for a later release.
