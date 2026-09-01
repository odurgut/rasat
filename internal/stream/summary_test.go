package stream

import (
	"testing"
	"time"

	"github.com/odurgut/rasat/internal/store"
)

func TestSummarizeBatchEmpty(t *testing.T) {
	t.Parallel()
	if rows := SummarizeBatch(store.TraceBatch{}); len(rows) != 0 {
		t.Fatalf("rows %d", len(rows))
	}
}

func TestSummarizeBatchRootAndEnvelope(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	childStart := t0.Add(-time.Millisecond)
	batch := store.TraceBatch{
		Spans: []store.SpanRow{
			{
				Timestamp:     childStart,
				TraceID:       "aa",
				SpanID:        "c1",
				ParentSpanID:  "root",
				ServiceName:   "postgres",
				OperationName: "SELECT",
				DurationNs:    uint64(2 * time.Millisecond),
				StatusCode:    0,
			},
			{
				Timestamp:     t0,
				TraceID:       "aa",
				SpanID:        "r1",
				ParentSpanID:  "",
				ServiceName:   "checkout",
				OperationName: "POST /pay",
				DurationNs:    uint64(5 * time.Millisecond),
				StatusCode:    2,
			},
			{
				Timestamp:     t0.Add(time.Second),
				TraceID:       "bb",
				SpanID:        "r2",
				ServiceName:   "gateway",
				OperationName: "GET /",
				DurationNs:    uint64(time.Millisecond),
				StatusCode:    1,
			},
			{
				TraceID: "",
			},
		},
	}
	rows := SummarizeBatch(batch)
	if len(rows) != 2 {
		t.Fatalf("rows %d", len(rows))
	}
	a := rows[0]
	if a.TraceID != "aa" {
		t.Fatalf("order %+v", rows)
	}
	if a.Service != "checkout" || a.Operation != "POST /pay" {
		t.Fatalf("root %+v", a)
	}
	if a.SpanCount != 2 {
		t.Fatalf("span_count %d", a.SpanCount)
	}
	if a.StatusCode != 2 {
		t.Fatalf("status %d", a.StatusCode)
	}
	if !a.Timestamp.Equal(childStart) {
		t.Fatalf("timestamp %s", a.Timestamp)
	}
	wantDur := uint64(t0.Add(5 * time.Millisecond).Sub(childStart))
	if a.DurationNs != wantDur {
		t.Fatalf("duration %d want %d", a.DurationNs, wantDur)
	}
	if rows[1].TraceID != "bb" || rows[1].Service != "gateway" {
		t.Fatalf("second %+v", rows[1])
	}
}

func TestSummarizeBatchEarliestAmongRoots(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 8, 26, 11, 0, 0, 0, time.UTC)
	batch := store.TraceBatch{
		Spans: []store.SpanRow{
			{
				Timestamp:     t0.Add(time.Millisecond),
				TraceID:       "aa",
				ParentSpanID:  "",
				ServiceName:   "later",
				OperationName: "b",
			},
			{
				Timestamp:     t0,
				TraceID:       "aa",
				ParentSpanID:  "",
				ServiceName:   "earlier",
				OperationName: "a",
			},
		},
	}
	rows := SummarizeBatch(batch)
	if len(rows) != 1 || rows[0].Service != "earlier" || rows[0].Operation != "a" {
		t.Fatalf("%+v", rows)
	}
}
