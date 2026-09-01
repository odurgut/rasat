package store

import (
	"errors"
	"fmt"
	"regexp"
)

// ErrInvalidIdent is returned when a configured database name is not a safe identifier.
var ErrInvalidIdent = errors.New("invalid identifier")

// identRE is a ClickHouse identifier. Configured database names must match.
var identRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`)

func quoteIdent(name string) (string, error) {
	if !identRE.MatchString(name) {
		return "", fmt.Errorf("%w: clickhouse database name %q", ErrInvalidIdent, name)
	}
	return name, nil
}

// Statements returns idempotent DDL for the Rasat database.
//
// Layout (one row per span; a trace is a group of spans — no separate traces table):
//
//   - spans: search, waterfall, dashboards, service map. Resource is denormalized
//     (service_name + resource_attributes). Span attributes are a Map, not a child
//     table: 10k spans/s cannot afford an attributes join on every search.
//   - span_events / span_links: child rows keyed by (trace_id, span_id) for detail.
//   - logs: structured ingest. Same service_name + timestamp prefix as spans so
//     a time window + service or a bloom on trace_id can find related rows (5.2).
//   - services: ReplacingMergeTree fed by a materialized view — catalog without a scan.
//
// ORDER BY (service_name, timestamp, trace_id, span_id):
//
//	service + time range is the prefix (search, metrics, map window).
//	Get-by-trace-id uses INDEX idx_trace bloom_filter, not the primary key
//	(ORDER BY trace_id would destroy time-range scans).
//
// PARTITION BY toDate(timestamp): drop old days for retention later.
func Statements(database string) ([]string, error) {
	db, err := quoteIdent(database)
	if err != nil {
		return nil, err
	}
	t := func(name string) string { return db + "." + name }

	spans := t("spans")
	events := t("span_events")
	links := t("span_links")
	logs := t("logs")
	services := t("services")
	servicesMV := t("services_mv")

	return []string{
		fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", db),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			timestamp DateTime64(9, 'UTC'),
			trace_id String,
			span_id String,
			parent_span_id String,
			service_name LowCardinality(String),
			operation_name LowCardinality(String),
			kind Int32,
			duration_ns UInt64,
			status_code UInt8,
			status_message String,
			scope_name LowCardinality(String),
			scope_version LowCardinality(String),
			resource_attributes Map(LowCardinality(String), String),
			span_attributes Map(LowCardinality(String), String),
			ingested_at DateTime64(9, 'UTC') DEFAULT now64(9),
			INDEX idx_trace trace_id TYPE bloom_filter(0.01) GRANULARITY 1,
			INDEX idx_span span_id TYPE bloom_filter(0.01) GRANULARITY 1,
			INDEX idx_status status_code TYPE set(8) GRANULARITY 4,
			INDEX idx_op operation_name TYPE bloom_filter(0.01) GRANULARITY 4,
			INDEX idx_dur duration_ns TYPE minmax GRANULARITY 4,
			INDEX idx_res_keys mapKeys(resource_attributes) TYPE bloom_filter(0.01) GRANULARITY 4,
			INDEX idx_attr_keys mapKeys(span_attributes) TYPE bloom_filter(0.01) GRANULARITY 4
		)
		ENGINE = MergeTree
		PARTITION BY toDate(timestamp)
		ORDER BY (service_name, timestamp, trace_id, span_id)
		SETTINGS index_granularity = 8192`, spans),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			timestamp DateTime64(9, 'UTC'),
			trace_id String,
			span_id String,
			event_time DateTime64(9, 'UTC'),
			event_name LowCardinality(String),
			event_attributes Map(LowCardinality(String), String)
		)
		ENGINE = MergeTree
		PARTITION BY toDate(timestamp)
		ORDER BY (trace_id, span_id, event_time)
		SETTINGS index_granularity = 8192`, events),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			timestamp DateTime64(9, 'UTC'),
			trace_id String,
			span_id String,
			linked_trace_id String,
			linked_span_id String,
			link_attributes Map(LowCardinality(String), String)
		)
		ENGINE = MergeTree
		PARTITION BY toDate(timestamp)
		ORDER BY (trace_id, span_id)
		SETTINGS index_granularity = 8192`, links),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			timestamp DateTime64(9, 'UTC'),
			service_name LowCardinality(String),
			level LowCardinality(String),
			message String,
			trace_id String,
			span_id String,
			ingested_at DateTime64(9, 'UTC') DEFAULT now64(9),
			INDEX idx_trace trace_id TYPE bloom_filter(0.01) GRANULARITY 1,
			INDEX idx_level level TYPE set(16) GRANULARITY 4
		)
		ENGINE = MergeTree
		PARTITION BY toDate(timestamp)
		ORDER BY (service_name, timestamp, trace_id)
		SETTINGS index_granularity = 8192`, logs),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			service_name LowCardinality(String),
			last_seen DateTime64(9, 'UTC')
		)
		ENGINE = ReplacingMergeTree(last_seen)
		ORDER BY (service_name)`, services),
		fmt.Sprintf(`CREATE MATERIALIZED VIEW IF NOT EXISTS %s
		TO %s
		AS SELECT
			service_name,
			timestamp AS last_seen
		FROM %s`, servicesMV, services, spans),
	}, nil
}
