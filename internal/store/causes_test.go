package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestListErrorCausesRequiresWindowLimit(t *testing.T) {
	t.Parallel()
	s := &Store{conn: &fakeConn{}, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	tests := []ErrorCausesQuery{
		{},
		{Start: start, End: end, Limit: 5},
		{Start: start, End: end, Limit: 5, Service: "checkout"},
		{Start: start, End: end, Limit: 0, Service: "checkout"},
		{Start: end, End: start, Limit: 5, Service: "checkout"},
		{Start: start, End: end, Limit: maxSearchLimit + 1, Service: "checkout"},
	}
	for i, q := range tests {
		if i == 1 || i == 2 {
			continue
		}
		if _, err := s.ListErrorCauses(context.Background(), q); !errors.Is(err, ErrInvalidSearch) {
			t.Fatalf("got %v for %+v", err, q)
		}
	}
	if _, err := s.ListErrorCauses(context.Background(), tests[1]); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ListErrorCauses(context.Background(), tests[2]); err != nil {
		t.Fatal(err)
	}
}

func TestListErrorCausesSQL(t *testing.T) {
	t.Parallel()
	seen := time.Date(2026, 8, 26, 12, 1, 0, 0, time.UTC)
	fc := &fakeConn{
		causeRows: []ErrorCause{{Cause: "AuthError", Count: 31, FirstSeen: seen}, {Cause: "unauthorized", Count: 8, FirstSeen: seen}},
	}
	s := &Store{conn: fc, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	rows, err := s.ListErrorCauses(context.Background(), ErrorCausesQuery{
		Start:   start,
		End:     start.Add(time.Hour),
		Limit:   50,
		Service: "auth",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Cause != "AuthError" || rows[0].Count != 31 || !rows[0].FirstSeen.Equal(seen) {
		t.Fatalf("rows %+v", rows)
	}
	if len(fc.selects) != 1 {
		t.Fatalf("selects %d", len(fc.selects))
	}
	q := fc.selects[0]
	for _, want := range []string{
		"rasat.spans",
		"rasat.span_events",
		"status_code = 2",
		"event_name = 'exception'",
		"exception.type",
		"status_message",
		"min(ts) AS first_seen",
		"service_name = ?",
		"ORDER BY n DESC",
		"LIMIT ?",
	} {
		if !strings.Contains(q, want) {
			t.Fatalf("missing %q in %s", want, q)
		}
	}
	args := fc.selectArgs[0]
	if len(args) != 6 {
		t.Fatalf("args %v", args)
	}
	if args[4] != "auth" {
		t.Fatalf("service %v", args[4])
	}
	if args[5] != maxErrorCauses {
		t.Fatalf("limit clamped %v", args[5])
	}
}

func TestListErrorCausesFleetSQL(t *testing.T) {
	t.Parallel()
	fc := &fakeConn{}
	s := &Store{conn: fc, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	if _, err := s.ListErrorCauses(context.Background(), ErrorCausesQuery{
		Start: start,
		End:   start.Add(time.Hour),
		Limit: 5,
	}); err != nil {
		t.Fatal(err)
	}
	q := fc.selects[0]
	if strings.Contains(q, "service_name = ?") {
		t.Fatalf("fleet query still scoped: %s", q)
	}
	if !strings.Contains(q, "min(ts) AS first_seen") {
		t.Fatalf("missing first_seen in %s", q)
	}
	args := fc.selectArgs[0]
	if len(args) != 5 {
		t.Fatalf("args %v", args)
	}
	if args[4] != 5 {
		t.Fatalf("limit %v", args[4])
	}
}

func TestListErrorCausesEmpty(t *testing.T) {
	t.Parallel()
	s := &Store{conn: &fakeConn{}, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	rows, err := s.ListErrorCauses(context.Background(), ErrorCausesQuery{
		Start:   start,
		End:     start.Add(time.Hour),
		Limit:   5,
		Service: "checkout",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rows == nil || len(rows) != 0 {
		t.Fatalf("rows %+v", rows)
	}
}

func TestListErrorCausesNilStore(t *testing.T) {
	t.Parallel()
	s := &Store{}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	if _, err := s.ListErrorCauses(context.Background(), ErrorCausesQuery{
		Start:   start,
		End:     start.Add(time.Hour),
		Limit:   5,
		Service: "checkout",
	}); !errors.Is(err, ErrNotReady) {
		t.Fatalf("got %v", err)
	}
}
