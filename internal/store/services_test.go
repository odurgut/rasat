package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestListServicesRequiresWindowAndLimit(t *testing.T) {
	t.Parallel()
	s := &Store{conn: &fakeConn{}, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	tests := []ServiceList{
		{},
		{Start: start, End: end},
		{Start: start, End: end, Limit: 0},
		{Start: end, End: start, Limit: 10},
		{Start: start, End: end, Limit: maxSearchLimit + 1},
	}
	for _, q := range tests {
		if _, err := s.ListServices(context.Background(), q); !errors.Is(err, ErrInvalidSearch) {
			t.Fatalf("got %v for %+v", err, q)
		}
	}
}

func TestListServicesSQL(t *testing.T) {
	t.Parallel()
	seen := time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC)
	fc := &fakeConn{
		serviceRows: []ServiceRow{{Service: "checkout", LastSeen: seen}},
	}
	s := &Store{conn: fc, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	rows, err := s.ListServices(context.Background(), ServiceList{
		Start: start,
		End:   start.Add(24 * time.Hour),
		Limit: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Service != "checkout" {
		t.Fatalf("rows %+v", rows)
	}
	if len(fc.selects) != 1 {
		t.Fatalf("selects %d", len(fc.selects))
	}
	q := fc.selects[0]
	for _, want := range []string{
		"rasat.spans",
		"sp.timestamp >= ?",
		"sp.service_name != ''",
		"GROUP BY sp.service_name",
		"count() AS spans",
		"ORDER BY last_seen DESC",
		"LIMIT ?",
	} {
		if !strings.Contains(q, want) {
			t.Fatalf("missing %q in %s", want, q)
		}
	}
	if strings.Contains(q, "checkout") {
		t.Fatal("user values must be bound, not interpolated")
	}
	if fc.selectArgs[0][2] != 50 {
		t.Fatalf("limit arg %v", fc.selectArgs[0])
	}
}

func TestListServicesEmpty(t *testing.T) {
	t.Parallel()
	s := &Store{conn: &fakeConn{}, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	rows, err := s.ListServices(context.Background(), ServiceList{
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

func TestListServicesNilStore(t *testing.T) {
	t.Parallel()
	var s *Store
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	if _, err := s.ListServices(context.Background(), ServiceList{
		Start: start,
		End:   start.Add(time.Hour),
		Limit: 10,
	}); !errors.Is(err, ErrNotReady) {
		t.Fatalf("got %v", err)
	}
}

func TestListServicesBadDatabase(t *testing.T) {
	t.Parallel()
	s := &Store{conn: &fakeConn{}, database: "rasat-prod", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	_, err := s.ListServices(context.Background(), ServiceList{
		Start: start,
		End:   start.Add(time.Hour),
		Limit: 10,
	})
	if !errors.Is(err, ErrInvalidIdent) {
		t.Fatalf("got %v", err)
	}
}
