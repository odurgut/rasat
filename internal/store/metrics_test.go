package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestListMetricsRequiresWindowAndLimit(t *testing.T) {
	t.Parallel()
	s := &Store{conn: &fakeConn{}, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	tests := []MetricsQuery{
		{},
		{Start: start, End: end},
		{Start: start, End: end, Limit: 0},
		{Start: end, End: start, Limit: 10},
		{Start: start, End: end, Limit: maxSearchLimit + 1},
	}
	for _, q := range tests {
		if _, err := s.ListMetrics(context.Background(), q); !errors.Is(err, ErrInvalidSearch) {
			t.Fatalf("got %v for %+v", err, q)
		}
	}
}

func TestListMetricsSQL(t *testing.T) {
	t.Parallel()
	fc := &fakeConn{
		metricRows: []ServiceMetrics{{
			Service: "checkout",
			Spans:   3600,
			Errors:  90,
			AvgNs:   12_000_000,
			P50Ns:   8_000_000,
			P95Ns:   45_000_000,
			P99Ns:   90_000_000,
		}},
	}
	s := &Store{conn: fc, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	rows, err := s.ListMetrics(context.Background(), MetricsQuery{
		Start: start,
		End:   start.Add(time.Hour),
		Limit: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Service != "checkout" {
		t.Fatalf("rows %+v", rows)
	}
	if rows[0].P95Ns != 45_000_000 {
		t.Fatalf("p95 %d", rows[0].P95Ns)
	}
	if rows[0].Rate != 1 || rows[0].ErrorRate != 0.025 {
		t.Fatalf("rates %+v", rows[0])
	}
	if len(fc.selects) != 1 {
		t.Fatalf("selects %d", len(fc.selects))
	}
	q := fc.selects[0]
	for _, want := range []string{
		"rasat.spans",
		"timestamp >= ?",
		"countIf(status_code = 2)",
		"quantileTDigest(0.5)",
		"quantileTDigest(0.95)",
		"quantileTDigest(0.99)",
		"GROUP BY service_name",
		"ORDER BY spans DESC",
		"LIMIT ?",
	} {
		if !strings.Contains(q, want) {
			t.Fatalf("missing %q in %s", want, q)
		}
	}
	if strings.Contains(q, "AND service_name = ?") {
		t.Fatal("no service filter unless requested")
	}
	args := fc.selectArgs[0]
	if len(args) != 3 || args[2] != 50 {
		t.Fatalf("args %v", args)
	}
}

func TestListMetricsSQLServiceFilter(t *testing.T) {
	t.Parallel()
	fc := &fakeConn{
		metricRows: []ServiceMetrics{{Service: "checkout", Spans: 10}},
	}
	s := &Store{conn: fc, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	_, err := s.ListMetrics(context.Background(), MetricsQuery{
		Start:   start,
		End:     start.Add(time.Hour),
		Limit:   10,
		Service: "checkout",
	})
	if err != nil {
		t.Fatal(err)
	}
	q := fc.selects[0]
	if !strings.Contains(q, "AND service_name = ?") {
		t.Fatalf("missing service bind in %s", q)
	}
	if strings.Contains(q, "checkout") {
		t.Fatal("user values must be bound, not interpolated")
	}
	args := fc.selectArgs[0]
	if len(args) != 4 || args[2] != "checkout" || args[3] != 10 {
		t.Fatalf("args %v", args)
	}
}

func TestListMetricsEmpty(t *testing.T) {
	t.Parallel()
	s := &Store{conn: &fakeConn{}, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	rows, err := s.ListMetrics(context.Background(), MetricsQuery{
		Start: start,
		End:   start.Add(time.Hour),
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rows == nil || len(rows) != 0 {
		t.Fatalf("got %#v", rows)
	}
}

func TestListMetricsNilStore(t *testing.T) {
	t.Parallel()
	var s *Store
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	if _, err := s.ListMetrics(context.Background(), MetricsQuery{
		Start: start,
		End:   start.Add(time.Hour),
		Limit: 10,
	}); !errors.Is(err, ErrNotReady) {
		t.Fatalf("got %v", err)
	}
}

func TestFillRatesZeroSpans(t *testing.T) {
	t.Parallel()
	rows := []ServiceMetrics{{Service: "idle", Spans: 0, Errors: 0}}
	fillRates(rows, time.Hour)
	if rows[0].Rate != 0 || rows[0].ErrorRate != 0 {
		t.Fatalf("%+v", rows[0])
	}
}
