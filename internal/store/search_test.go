package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSearchTracesRequiresWindowAndLimit(t *testing.T) {
	t.Parallel()
	s := &Store{conn: &fakeConn{}, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	tests := []TraceSearch{
		{},
		{Start: start, End: end},
		{Start: start, End: end, Limit: 0},
		{Start: end, End: start, Limit: 10},
		{Start: start, End: end, Limit: maxSearchLimit + 1},
	}
	for _, q := range tests {
		if _, err := s.SearchTraces(context.Background(), q); !errors.Is(err, ErrInvalidSearch) {
			t.Fatalf("got %v", err)
		}
	}
}

func TestSearchTracesSQL(t *testing.T) {
	t.Parallel()
	fc := &fakeConn{
		traceRows: []TraceListRow{{
			TraceID:    "aa",
			Service:    "checkout",
			Operation:  "GET /pay",
			DurationNs: 12_000_000,
			SpanCount:  3,
			Timestamp:  time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC),
			StatusCode: 2,
		}},
	}
	s := &Store{conn: fc, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	minDur := uint64(500_000_000)
	status := uint8(2)
	rows, err := s.SearchTraces(context.Background(), TraceSearch{
		Start:         start,
		End:           end,
		Limit:         50,
		Service:       "checkout",
		Operation:     "GET /pay",
		TraceID:       "aa",
		MinDurationNs: &minDur,
		StatusCode:    &status,
		AttrKey:       "http.method",
		AttrValue:     "GET",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].StatusCode != 2 || rows[0].Service != "checkout" {
		t.Fatalf("rows %+v", rows)
	}
	if len(fc.selects) != 1 {
		t.Fatalf("selects %d", len(fc.selects))
	}
	q := fc.selects[0]
	for _, want := range []string{
		"rasat.spans",
		"timestamp >= ?",
		"service_name = ?",
		"operation_name = ?",
		"trace_id = ?",
		"duration_ns >= ?",
		"status_code = ?",
		"span_attributes[?]",
		"resource_attributes[?]",
		"GROUP BY s.trace_id",
		"LIMIT ?",
	} {
		if !strings.Contains(q, want) {
			t.Fatalf("missing %q in %s", want, q)
		}
	}
	if strings.Contains(q, "checkout") || strings.Contains(q, "GET /pay") {
		t.Fatal("user values must be bound, not interpolated")
	}
}

func TestSearchTracesNilStore(t *testing.T) {
	t.Parallel()
	var s *Store
	if _, err := s.SearchTraces(context.Background(), TraceSearch{
		Start: time.Now().Add(-time.Hour),
		End:   time.Now(),
		Limit: 10,
	}); !errors.Is(err, ErrNotReady) {
		t.Fatalf("got %v", err)
	}
}

func TestSearchTracesEmpty(t *testing.T) {
	t.Parallel()
	s := &Store{conn: &fakeConn{}, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	rows, err := s.SearchTraces(context.Background(), TraceSearch{
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

func TestSearchTracesBadDatabase(t *testing.T) {
	t.Parallel()
	s := &Store{conn: &fakeConn{}, database: "rasat-prod", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	_, err := s.SearchTraces(context.Background(), TraceSearch{
		Start: start,
		End:   start.Add(time.Hour),
		Limit: 10,
	})
	if !errors.Is(err, ErrInvalidIdent) {
		t.Fatalf("got %v", err)
	}
}
