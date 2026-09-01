package store

import (
	"testing"
	"time"
)

func TestRankBottlenecksSubtractsChildren(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	spans := []SpanDetail{
		{SpanID: "root", Service: "gateway", Operation: "GET", Timestamp: t0, DurationNs: 100_000_000},
		{SpanID: "db", ParentSpanID: "root", Service: "postgres", Operation: "Query", Timestamp: t0.Add(10 * time.Millisecond), DurationNs: 40_000_000},
	}
	got := rankBottlenecks(spans)
	if len(got) != 2 {
		t.Fatalf("len %d", len(got))
	}
	if got[0].SpanID != "root" || got[0].ExclusiveNs != 60_000_000 {
		t.Fatalf("first %+v", got[0])
	}
	if got[1].SpanID != "db" || got[1].ExclusiveNs != 40_000_000 {
		t.Fatalf("second %+v", got[1])
	}
}

func TestRankBottlenecksParallelOverlap(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	spans := []SpanDetail{
		{SpanID: "root", Service: "checkout", Operation: "pay", Timestamp: t0, DurationNs: 100_000_000},
		{SpanID: "a", ParentSpanID: "root", Service: "redis", Operation: "GET", Timestamp: t0, DurationNs: 80_000_000},
		{SpanID: "b", ParentSpanID: "root", Service: "postgres", Operation: "Query", Timestamp: t0, DurationNs: 80_000_000},
	}
	got := rankBottlenecks(spans)
	if len(got) != 3 {
		t.Fatalf("len %d %+v", len(got), got)
	}
	if got[0].SpanID != "b" || got[0].ExclusiveNs != 80_000_000 {
		t.Fatalf("first %+v", got[0])
	}
	if got[2].SpanID != "root" || got[2].ExclusiveNs != 20_000_000 {
		t.Fatalf("parent self %+v", got[2])
	}
}

func TestCoverNsMergesOverlapNotSum(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	parent := SpanDetail{Timestamp: t0, DurationNs: 100_000_000}
	kids := []SpanDetail{
		{Timestamp: t0, DurationNs: 80_000_000},
		{Timestamp: t0.Add(20 * time.Millisecond), DurationNs: 80_000_000},
	}
	got := coverNs(parent, kids)
	if got != 100_000_000 {
		t.Fatalf("cover %d", got)
	}
	if selfTimeNs(parent, kids) != 0 {
		t.Fatalf("self %d", selfTimeNs(parent, kids))
	}
}

func TestRankBottlenecksCapsTopN(t *testing.T) {
	t.Parallel()
	spans := make([]SpanDetail, maxBottlenecks+3)
	for i := range spans {
		spans[i] = SpanDetail{
			SpanID:     string(rune('a' + i)),
			Service:    "svc",
			Operation:  "op",
			DurationNs: uint64((i + 1) * 1_000_000),
		}
	}
	got := rankBottlenecks(spans)
	if len(got) != maxBottlenecks {
		t.Fatalf("len %d", len(got))
	}
	if got[0].ExclusiveNs != uint64((maxBottlenecks+3)*1_000_000) {
		t.Fatalf("top %+v", got[0])
	}
}

func TestRankBottlenecksEmpty(t *testing.T) {
	t.Parallel()
	if got := rankBottlenecks(nil); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}
