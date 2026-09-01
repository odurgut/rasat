package seed

import (
	"math/rand"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/odurgut/rasat/internal/ingest"
)

func TestTracesCoversTheProduct(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	td := Traces(Options{Now: now, Count: 16, Rand: rand.New(rand.NewSource(1))})
	batch, err := ingest.FlattenTraces(td, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Spans) < 40 {
		t.Fatalf("spans %d", len(batch.Spans))
	}

	services := map[string]int{}
	kinds := map[int32]int{}
	status := map[uint8]int{}
	var errors, children, slow int
	routes := 0
	for _, s := range batch.Spans {
		services[s.ServiceName]++
		kinds[s.Kind]++
		status[s.StatusCode]++
		if s.StatusCode == 2 {
			errors++
		}
		if s.ParentSpanID != "" {
			children++
		}
		if s.DurationNs >= 100_000_000 {
			slow++
		}
		if s.SpanAttributes["http.route"] != "" {
			routes++
		}
	}
	events, links := 0, len(batch.Links)
	for _, ev := range batch.Events {
		if ev.EventName == "exception" || ev.EventName == "cache.hit" || ev.EventName == "cache.miss" {
			events++
		}
	}

	for _, want := range []string{
		"web-bff", "gateway", "auth", "catalog", "cart", "checkout", "payment",
		"inventory", "search", "notify-worker", "fraud", "postgres", "redis",
		"elasticsearch", "kafka",
	} {
		if services[want] == 0 {
			t.Fatalf("missing service %s in %v", want, services)
		}
	}
	for _, k := range []int32{
		int32(ptrace.SpanKindInternal),
		int32(ptrace.SpanKindServer),
		int32(ptrace.SpanKindClient),
		int32(ptrace.SpanKindProducer),
		int32(ptrace.SpanKindConsumer),
	} {
		if kinds[k] == 0 {
			t.Fatalf("missing kind %d in %v", k, kinds)
		}
	}
	if errors == 0 || status[0] == 0 || status[1] == 0 {
		t.Fatalf("status mix errors=%d counts=%v", errors, status)
	}
	if children == 0 {
		t.Fatal("expected parent/child spans")
	}
	if events == 0 || links == 0 {
		t.Fatalf("events %d links %d", events, links)
	}
	if slow == 0 {
		t.Fatal("expected a slow span (>=100ms)")
	}
	if routes == 0 {
		t.Fatal("expected http.route attributes")
	}
	inWindow := false
	for _, s := range batch.Spans {
		if !s.Timestamp.After(now) && now.Sub(s.Timestamp) <= 18*time.Hour {
			inWindow = true
			break
		}
	}
	if !inWindow {
		t.Fatal("expected spans inside the last 18 hours")
	}
}

func TestTracesDefaultCount(t *testing.T) {
	t.Parallel()
	td := Traces(Options{Now: time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC), Rand: rand.New(rand.NewSource(2))})
	if td.SpanCount() < 40 {
		t.Fatalf("span count %d", td.SpanCount())
	}
}

func TestStatusCodes(t *testing.T) {
	t.Parallel()
	if ptrace.StatusCodeError != 2 {
		t.Fatalf("otlp error code %d", ptrace.StatusCodeError)
	}
}
