package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestGetTraceRequiresWindowAndID(t *testing.T) {
	t.Parallel()
	s := &Store{conn: &fakeConn{}, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	tests := []TraceGet{
		{},
		{TraceID: "aa"},
		{TraceID: "aa", Start: start, End: start},
		{TraceID: "aa", Start: end, End: start},
	}
	for _, q := range tests {
		if _, err := s.GetTrace(context.Background(), q); !errors.Is(err, ErrInvalidSearch) {
			t.Fatalf("got %v for %+v", err, q)
		}
	}
}

func TestGetTraceNotFound(t *testing.T) {
	t.Parallel()
	s := &Store{conn: &fakeConn{}, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	_, err := s.GetTrace(context.Background(), TraceGet{
		TraceID: "aa",
		Start:   start,
		End:     start.Add(time.Hour),
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestGetTraceAssemblesTree(t *testing.T) {
	t.Parallel()
	root := time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC)
	child := root.Add(2 * time.Millisecond)
	fc := &fakeConn{
		selectFn: func(dest any, query string, _ []any) error {
			switch {
			case strings.Contains(query, ".spans"):
				rows := dest.(*[]spanScan)
				*rows = []spanScan{
					{
						Timestamp:          root,
						TraceID:            "aa",
						SpanID:             "01",
						ServiceName:        "gateway",
						OperationName:      "HTTP GET",
						Kind:               2,
						DurationNs:         uint64(10 * time.Millisecond),
						ResourceAttributes: nil,
					},
					{
						Timestamp:     child,
						TraceID:       "aa",
						SpanID:        "02",
						ParentSpanID:  "01",
						ServiceName:   "checkout",
						OperationName: "pay",
						DurationNs:    uint64(5 * time.Millisecond),
						StatusCode:    2,
						SpanAttributes: map[string]string{
							"http.status_code": "500",
						},
					},
				}
			case strings.Contains(query, "span_events"):
				rows := dest.(*[]eventScan)
				*rows = []eventScan{{
					SpanID:     "02",
					EventTime:  child.Add(time.Millisecond),
					EventName:  "exception",
					Attributes: map[string]string{"exception.type": "Error"},
				}}
			case strings.Contains(query, "span_links"):
				rows := dest.(*[]linkScan)
				*rows = []linkScan{{
					SpanID:        "01",
					LinkedTraceID: "bb",
					LinkedSpanID:  "99",
				}}
			}
			return nil
		},
	}
	s := &Store{conn: fc, database: "rasat", queryTimeout: time.Second}
	got, err := s.GetTrace(context.Background(), TraceGet{
		TraceID: "aa",
		Start:   root.Add(-time.Hour),
		End:     root.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.TraceID != "aa" || got.SpanCount != 2 || got.Timestamp != root {
		t.Fatalf("header %+v", got)
	}
	wantDur := uint64(10 * time.Millisecond)
	if got.DurationNs != wantDur {
		t.Fatalf("duration %d want %d", got.DurationNs, wantDur)
	}
	if got.Spans[0].SpanID != "01" || got.Spans[1].ParentSpanID != "01" {
		t.Fatalf("order %+v", got.Spans)
	}
	if got.Spans[0].ResourceAttributes == nil || got.Spans[1].Events[0].Name != "exception" {
		t.Fatalf("children %+v", got.Spans)
	}
	if got.Spans[0].Links[0].TraceID != "bb" || got.Spans[1].Events == nil {
		t.Fatalf("links %+v", got.Spans[0].Links)
	}
	if len(got.CriticalPath) != 2 || got.CriticalPath[0].SpanID != "01" || got.CriticalPath[1].SpanID != "02" {
		t.Fatalf("critical path %+v", got.CriticalPath)
	}
	if got.CriticalPathNs != uint64(7*time.Millisecond) {
		t.Fatalf("path ns %d", got.CriticalPathNs)
	}
	if len(got.Bottlenecks) != 2 || got.Bottlenecks[0].ExclusiveNs != uint64(5*time.Millisecond) {
		t.Fatalf("bottlenecks %+v", got.Bottlenecks)
	}
	if len(fc.selects) != 3 {
		t.Fatalf("selects %d", len(fc.selects))
	}
	for i, q := range fc.selects {
		if !strings.Contains(q, "trace_id = ?") || !strings.Contains(q, "timestamp >= ?") || !strings.Contains(q, "LIMIT ?") {
			t.Fatalf("unbounded or interpolated query %d: %s", i, q)
		}
		if strings.Contains(q, "aa") {
			t.Fatal("trace id must be bound, not interpolated")
		}
	}
	if fc.selectArgs[0][0] != "aa" {
		t.Fatalf("bound trace id %v", fc.selectArgs[0])
	}
}

func TestGetTraceTooLarge(t *testing.T) {
	t.Parallel()
	fc := &fakeConn{
		selectFn: func(dest any, query string, _ []any) error {
			if !strings.Contains(query, ".spans") {
				return nil
			}
			rows := dest.(*[]spanScan)
			*rows = make([]spanScan, maxTraceSpans+1)
			return nil
		},
	}
	s := &Store{conn: fc, database: "rasat", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	_, err := s.GetTrace(context.Background(), TraceGet{
		TraceID: "aa",
		Start:   start,
		End:     start.Add(time.Hour),
	})
	if !errors.Is(err, ErrTraceTooLarge) {
		t.Fatalf("got %v", err)
	}
}

func TestGetTraceNilStore(t *testing.T) {
	t.Parallel()
	var s *Store
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	if _, err := s.GetTrace(context.Background(), TraceGet{
		TraceID: "aa",
		Start:   start,
		End:     start.Add(time.Hour),
	}); !errors.Is(err, ErrNotReady) {
		t.Fatalf("got %v", err)
	}
}

func TestGetTraceBadDatabase(t *testing.T) {
	t.Parallel()
	s := &Store{conn: &fakeConn{}, database: "rasat-prod", queryTimeout: time.Second}
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	_, err := s.GetTrace(context.Background(), TraceGet{
		TraceID: "aa",
		Start:   start,
		End:     start.Add(time.Hour),
	})
	if !errors.Is(err, ErrInvalidIdent) {
		t.Fatalf("got %v", err)
	}
}
