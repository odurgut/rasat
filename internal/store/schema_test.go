package store

import (
	"errors"
	"strings"
	"testing"
)

func TestStatementsRejectsBadDatabase(t *testing.T) {
	t.Parallel()
	_, err := Statements("rasat-prod")
	if !errors.Is(err, ErrInvalidIdent) {
		t.Fatalf("got %v", err)
	}
}

func TestStatementsShape(t *testing.T) {
	t.Parallel()
	stmts, err := Statements("rasat")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(stmts, "\n")
	for _, want := range []string{
		"CREATE DATABASE IF NOT EXISTS rasat",
		"CREATE TABLE IF NOT EXISTS rasat.spans",
		"CREATE TABLE IF NOT EXISTS rasat.span_events",
		"CREATE TABLE IF NOT EXISTS rasat.span_links",
		"CREATE TABLE IF NOT EXISTS rasat.logs",
		"ORDER BY (service_name, timestamp, trace_id)",
		"CREATE TABLE IF NOT EXISTS rasat.services",
		"CREATE MATERIALIZED VIEW IF NOT EXISTS rasat.services_mv",
		"ORDER BY (service_name, timestamp, trace_id, span_id)",
		"PARTITION BY toDate(timestamp)",
		"INDEX idx_trace",
		"bloom_filter",
		"resource_attributes Map",
		"span_attributes Map",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q", want)
		}
	}
	if strings.Contains(joined, ";") {
		t.Fatal("statements must be one Exec each, no bundled semicolons")
	}
}
