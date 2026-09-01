package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestListMetricsSeriesRequiresStep(t *testing.T) {
	t.Parallel()
	s := &Store{conn: &fakeConn{}, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	tests := []MetricsQuery{
		{Start: start, End: end, Limit: 10},
		{Start: start, End: end, Limit: 10, Step: time.Millisecond},
		{Start: start, End: start.Add(24 * time.Hour), Limit: 10, Step: time.Second},
	}
	for _, q := range tests {
		if _, err := s.ListMetricsSeries(context.Background(), q); !errors.Is(err, ErrInvalidSearch) {
			t.Fatalf("got %v for %+v", err, q)
		}
	}
}

func TestListMetricsSeriesSQL(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	fc := &fakeConn{
		metricBuckets: []metricBucketRow{{
			Service: "checkout",
			Time:    start,
			Spans:   60,
			Errors:  3,
			AvgNs:   12_000_000,
			P50Ns:   8_000_000,
			P95Ns:   45_000_000,
			P99Ns:   90_000_000,
		}},
	}
	s := &Store{conn: fc, database: "rasat", queryTimeout: time.Second}
	rows, err := s.ListMetricsSeries(context.Background(), MetricsQuery{
		Start:   start,
		End:     start.Add(time.Hour),
		Limit:   10,
		Service: "checkout",
		Step:    time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Service != "checkout" {
		t.Fatalf("series %+v", rows)
	}
	if len(rows[0].Points) != 60 {
		t.Fatalf("points %d", len(rows[0].Points))
	}
	p0 := rows[0].Points[0]
	if p0.P95Ns != 45_000_000 || p0.Rate != 1 || p0.ErrorRate != 0.05 {
		t.Fatalf("point %+v", p0)
	}
	if rows[0].Points[1].Spans != 0 || rows[0].Points[1].Rate != 0 {
		t.Fatalf("padded %+v", rows[0].Points[1])
	}
	q := fc.selects[0]
	for _, want := range []string{
		"rasat.spans",
		"toStartOfInterval(timestamp, toIntervalSecond(?))",
		"quantileTDigest(0.95)",
		"GROUP BY service_name, bucket",
		"AND service_name = ?",
		"LIMIT ?",
	} {
		if !strings.Contains(q, want) {
			t.Fatalf("missing %q in %s", want, q)
		}
	}
	if strings.Contains(q, "checkout") {
		t.Fatal("user values must be bound, not interpolated")
	}
	args := fc.selectArgs[0]
	if len(args) != 5 || args[0] != int64(60) || args[3] != "checkout" || args[4] != maxMetricBuckets {
		t.Fatalf("args %v", args)
	}
}

func TestListMetricsSeriesFleetSQL(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	fc := &fakeConn{
		metricBuckets: []metricBucketRow{{
			Service: "*",
			Time:    start,
			Spans:   120,
			Errors:  6,
			P95Ns:   40_000_000,
		}},
	}
	s := &Store{conn: fc, database: "rasat", queryTimeout: time.Second}
	rows, err := s.ListMetricsSeries(context.Background(), MetricsQuery{
		Start: start,
		End:   start.Add(time.Hour),
		Limit: 10,
		Step:  time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Service != "*" {
		t.Fatalf("series %+v", rows)
	}
	q := fc.selects[0]
	for _, want := range []string{
		"'*' AS service_name",
		"GROUP BY bucket",
		"LIMIT ?",
	} {
		if !strings.Contains(q, want) {
			t.Fatalf("missing %q in %s", want, q)
		}
	}
	if strings.Contains(q, "GROUP BY service_name, bucket") || strings.Contains(q, "AND service_name = ?") {
		t.Fatalf("fleet query must not filter a service: %s", q)
	}
}

func TestListMetricsSeriesEmpty(t *testing.T) {
	t.Parallel()
	s := &Store{conn: &fakeConn{}, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	rows, err := s.ListMetricsSeries(context.Background(), MetricsQuery{
		Start: start,
		End:   start.Add(time.Hour),
		Limit: 10,
		Step:  time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rows == nil || len(rows) != 0 {
		t.Fatalf("got %#v", rows)
	}
}

func TestListMetricsSeriesNilStore(t *testing.T) {
	t.Parallel()
	var s *Store
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	if _, err := s.ListMetricsSeries(context.Background(), MetricsQuery{
		Start: start,
		End:   start.Add(time.Hour),
		Limit: 10,
		Step:  time.Minute,
	}); !errors.Is(err, ErrNotReady) {
		t.Fatalf("got %v", err)
	}
}

func TestAlignBucket(t *testing.T) {
	t.Parallel()
	got := alignBucket(time.Date(2026, 8, 26, 12, 7, 30, 0, time.UTC), 5*time.Minute)
	want := time.Date(2026, 8, 26, 12, 5, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}
