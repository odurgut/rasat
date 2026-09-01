package store

import (
	"testing"
	"time"
)

func TestCriticalPathLinear(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	spans := []SpanDetail{
		{SpanID: "a", Service: "web-bff", Operation: "GET", Timestamp: t0, DurationNs: 100_000_000},
		{SpanID: "b", ParentSpanID: "a", Service: "gateway", Operation: "POST", Timestamp: t0.Add(2 * time.Millisecond), DurationNs: 80_000_000},
		{SpanID: "c", ParentSpanID: "b", Service: "postgres", Operation: "Query", Timestamp: t0.Add(4 * time.Millisecond), DurationNs: 50_000_000},
	}
	steps, ns := criticalPath(spans)
	if len(steps) != 3 || steps[0].Service != "web-bff" || steps[1].Service != "gateway" || steps[2].Service != "postgres" {
		t.Fatalf("steps %+v", steps)
	}
	if ns != 54_000_000 {
		t.Fatalf("path ns %d", ns)
	}
}

func TestCriticalPathPicksLatestChild(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	spans := []SpanDetail{
		{SpanID: "root", Service: "gateway", Timestamp: t0, DurationNs: 200_000_000},
		{SpanID: "fast", ParentSpanID: "root", Service: "cache", Timestamp: t0, DurationNs: 5_000_000},
		{SpanID: "slow", ParentSpanID: "root", Service: "payment", Timestamp: t0.Add(10 * time.Millisecond), DurationNs: 150_000_000},
	}
	steps, _ := criticalPath(spans)
	if len(steps) != 2 || steps[1].SpanID != "slow" {
		t.Fatalf("steps %+v", steps)
	}
}

func TestCriticalPathEmpty(t *testing.T) {
	t.Parallel()
	steps, ns := criticalPath(nil)
	if len(steps) != 0 || ns != 0 {
		t.Fatalf("got %+v %d", steps, ns)
	}
}

func TestCriticalPathCycleHasNoRoot(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	spans := []SpanDetail{
		{SpanID: "a", ParentSpanID: "b", Service: "a", Timestamp: t0, DurationNs: 10_000_000},
		{SpanID: "b", ParentSpanID: "a", Service: "b", Timestamp: t0.Add(time.Millisecond), DurationNs: 20_000_000},
	}
	steps, _ := criticalPath(spans)
	if len(steps) == 0 {
		t.Fatal("empty path on cycle")
	}
}
