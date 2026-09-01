package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestServiceMapRequiresWindowAndLimit(t *testing.T) {
	t.Parallel()
	s := &Store{conn: &fakeConn{}, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	tests := []ServiceMapQuery{
		{},
		{Start: start, End: end},
		{Start: start, End: end, Limit: 0},
		{Start: end, End: start, Limit: 10},
	}
	for _, q := range tests {
		if _, err := s.ServiceMap(context.Background(), q); !errors.Is(err, ErrInvalidSearch) {
			t.Fatalf("got %v for %+v", err, q)
		}
	}
}

func TestServiceMapSQL(t *testing.T) {
	t.Parallel()
	fc := &fakeConn{
		mapNodes: []ServiceMapNode{{Service: "gateway", Spans: 4}},
		mapEdges: []ServiceMapEdge{{From: "gateway", To: "auth", Calls: 2, AvgDurationNs: 5_000_000}},
	}
	s := &Store{conn: fc, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	g, err := s.ServiceMap(context.Background(), ServiceMapQuery{
		Start: start,
		End:   start.Add(time.Hour),
		Limit: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Nodes) != 1 || g.Nodes[0].Service != "gateway" {
		t.Fatalf("nodes %+v", g.Nodes)
	}
	if len(g.Edges) != 1 || g.Edges[0].To != "auth" {
		t.Fatalf("edges %+v", g.Edges)
	}
	if len(fc.selects) != 2 {
		t.Fatalf("selects %d", len(fc.selects))
	}
	nodesSQL, edgesSQL := fc.selects[0], fc.selects[1]
	for _, want := range []string{"rasat.spans", "GROUP BY service_name", "LIMIT ?"} {
		if !strings.Contains(nodesSQL, want) {
			t.Fatalf("nodes missing %q in %s", want, nodesSQL)
		}
	}
	for _, want := range []string{
		"INNER JOIN",
		"parent.span_id = child.parent_span_id",
		"parent.service_name != child.service_name",
		"child.timestamp >= ?",
	} {
		if !strings.Contains(edgesSQL, want) {
			t.Fatalf("edges missing %q in %s", want, edgesSQL)
		}
	}
}

func TestServiceMapEmpty(t *testing.T) {
	t.Parallel()
	s := &Store{conn: &fakeConn{}, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	g, err := s.ServiceMap(context.Background(), ServiceMapQuery{
		Start: start,
		End:   start.Add(time.Hour),
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if g.Nodes == nil || g.Edges == nil {
		t.Fatalf("nil slices %#v", g)
	}
}

func TestServiceMapNilStore(t *testing.T) {
	t.Parallel()
	var s *Store
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	if _, err := s.ServiceMap(context.Background(), ServiceMapQuery{
		Start: start,
		End:   start.Add(time.Hour),
		Limit: 10,
	}); !errors.Is(err, ErrNotReady) {
		t.Fatalf("got %v", err)
	}
}
