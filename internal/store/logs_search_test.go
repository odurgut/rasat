package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSearchLogsRequiresWindowAndLimit(t *testing.T) {
	t.Parallel()
	s := &Store{conn: &fakeConn{}, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	tests := []LogSearch{
		{},
		{Start: start, End: end},
		{Start: start, End: end, Limit: 0},
		{Start: end, End: start, Limit: 10},
		{Start: start, End: end, Limit: maxSearchLimit + 1},
	}
	for _, q := range tests {
		if _, err := s.SearchLogs(context.Background(), q); !errors.Is(err, ErrInvalidSearch) {
			t.Fatalf("got %v", err)
		}
	}
}

func TestSearchLogsSQL(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	fc := &fakeConn{
		logRows: []LogRow{{
			Timestamp:   ts,
			ServiceName: "checkout",
			Level:       "ERROR",
			Message:     "database timeout",
			TraceID:     "abc123",
		}},
	}
	s := &Store{conn: fc, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	rows, err := s.SearchLogs(context.Background(), LogSearch{
		Start:   start,
		End:     start.Add(24 * time.Hour),
		Limit:   50,
		Service: "checkout",
		Level:   "ERROR",
		TraceID: "abc123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Level != "ERROR" || rows[0].ServiceName != "checkout" {
		t.Fatalf("rows %+v", rows)
	}
	if len(fc.selects) != 1 {
		t.Fatalf("selects %d", len(fc.selects))
	}
	q := fc.selects[0]
	for _, want := range []string{
		"rasat.logs",
		"timestamp >= ?",
		"service_name = ?",
		"level = ?",
		"trace_id = ?",
		"ORDER BY timestamp DESC",
		"LIMIT ?",
	} {
		if !strings.Contains(q, want) {
			t.Fatalf("missing %q in %s", want, q)
		}
	}
	if strings.Contains(q, "checkout") || strings.Contains(q, "ERROR") {
		t.Fatal("user values must be bound, not interpolated")
	}
}

func TestSearchLogsNilStore(t *testing.T) {
	t.Parallel()
	var s *Store
	if _, err := s.SearchLogs(context.Background(), LogSearch{
		Start: time.Now().Add(-time.Hour),
		End:   time.Now(),
		Limit: 10,
	}); !errors.Is(err, ErrNotReady) {
		t.Fatalf("got %v", err)
	}
}
