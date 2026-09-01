package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestListOperationsRequiresServiceWindowLimit(t *testing.T) {
	t.Parallel()
	s := &Store{conn: &fakeConn{}, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	tests := []OperationList{
		{},
		{Start: start, End: end, Limit: 10},
		{Service: "checkout", Start: start, End: end},
		{Service: "checkout", Start: start, End: end, Limit: 0},
		{Service: "checkout", Start: end, End: start, Limit: 10},
		{Service: "checkout", Start: start, End: end, Limit: maxSearchLimit + 1},
	}
	for _, q := range tests {
		if _, err := s.ListOperations(context.Background(), q); !errors.Is(err, ErrInvalidSearch) {
			t.Fatalf("got %v for %+v", err, q)
		}
	}
}

func TestListOperationsSQL(t *testing.T) {
	t.Parallel()
	fc := &fakeConn{
		operationRows: []OperationRow{{Operation: "HTTP POST /checkout", Spans: 4, Errors: 1, P50Ns: 10_000_000, P95Ns: 40_000_000}},
	}
	s := &Store{conn: fc, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	rows, err := s.ListOperations(context.Background(), OperationList{
		Service: "checkout",
		Start:   start,
		End:     start.Add(24 * time.Hour),
		Limit:   50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Operation != "HTTP POST /checkout" {
		t.Fatalf("rows %+v", rows)
	}
	if rows[0].Errors != 1 || rows[0].ErrorRate != 0.25 || rows[0].P95Ns != 40_000_000 {
		t.Fatalf("stats %+v", rows[0])
	}
	if len(fc.selects) != 1 {
		t.Fatalf("selects %d", len(fc.selects))
	}
	q := fc.selects[0]
	for _, want := range []string{
		"rasat.spans",
		"sp.timestamp >= ?",
		"sp.service_name = ?",
		"sp.operation_name != ''",
		"countIf(status_code = 2)",
		"quantileTDigest(0.5)",
		"quantileTDigest(0.95)",
		"GROUP BY sp.operation_name",
		"ORDER BY spans DESC",
		"LIMIT ?",
	} {
		if !strings.Contains(q, want) {
			t.Fatalf("missing %q in %s", want, q)
		}
	}
	if strings.Contains(q, "checkout") || strings.Contains(q, "HTTP POST") {
		t.Fatal("user values must be bound, not interpolated")
	}
	args := fc.selectArgs[0]
	if args[2] != "checkout" || args[3] != 50 {
		t.Fatalf("args %v", args)
	}
}

func TestListOperationsEmpty(t *testing.T) {
	t.Parallel()
	s := &Store{conn: &fakeConn{}, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	rows, err := s.ListOperations(context.Background(), OperationList{
		Service: "checkout",
		Start:   start,
		End:     start.Add(time.Hour),
		Limit:   10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rows == nil || len(rows) != 0 {
		t.Fatalf("got %#v", rows)
	}
}

func TestListOperationsNilStore(t *testing.T) {
	t.Parallel()
	var s *Store
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	if _, err := s.ListOperations(context.Background(), OperationList{
		Service: "checkout",
		Start:   start,
		End:     start.Add(time.Hour),
		Limit:   10,
	}); !errors.Is(err, ErrNotReady) {
		t.Fatalf("got %v", err)
	}
}
