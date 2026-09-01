package ingest

import (
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func sampleTraces() ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "checkout")
	rs.Resource().Attributes().PutStr("deployment.environment", "prod")

	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("go.opentelemetry.io/contrib")
	ss.Scope().SetVersion("1.2.3")

	start := time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC)
	end := start.Add(12 * time.Millisecond)

	sp := ss.Spans().AppendEmpty()
	sp.SetTraceID(pcommon.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	sp.SetSpanID(pcommon.SpanID{1, 2, 3, 4, 5, 6, 7, 8})
	sp.SetParentSpanID(pcommon.SpanID{8, 7, 6, 5, 4, 3, 2, 1})
	sp.SetName("GET /pay")
	sp.SetKind(ptrace.SpanKindServer)
	sp.SetStartTimestamp(pcommon.NewTimestampFromTime(start))
	sp.SetEndTimestamp(pcommon.NewTimestampFromTime(end))
	sp.Status().SetCode(ptrace.StatusCodeOk)
	sp.Status().SetMessage("ok")
	sp.Attributes().PutStr("http.method", "GET")
	sp.Attributes().PutInt("http.status_code", 200)

	ev := sp.Events().AppendEmpty()
	ev.SetName("exception")
	ev.SetTimestamp(pcommon.NewTimestampFromTime(start.Add(time.Millisecond)))
	ev.Attributes().PutStr("exception.type", "timeout")

	lk := sp.Links().AppendEmpty()
	lk.SetTraceID(pcommon.TraceID{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1})
	lk.SetSpanID(pcommon.SpanID{8, 8, 8, 8, 8, 8, 8, 8})
	lk.Attributes().PutStr("link", "yes")

	return td
}

func TestFlattenTraces(t *testing.T) {
	t.Parallel()
	batch, err := FlattenTraces(sampleTraces(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Spans) != 1 {
		t.Fatalf("spans %d", len(batch.Spans))
	}
	sp := batch.Spans[0]
	if sp.ServiceName != "checkout" || sp.OperationName != "GET /pay" {
		t.Fatalf("identity: %+v", sp)
	}
	if sp.Kind != int32(ptrace.SpanKindServer) || sp.StatusCode != uint8(ptrace.StatusCodeOk) {
		t.Fatalf("kind/status: %+v", sp)
	}
	if sp.DurationNs != uint64(12*time.Millisecond) {
		t.Fatalf("duration %d", sp.DurationNs)
	}
	if sp.ResourceAttributes["deployment.environment"] != "prod" {
		t.Fatalf("resource: %v", sp.ResourceAttributes)
	}
	if sp.SpanAttributes["http.method"] != "GET" || sp.SpanAttributes["http.status_code"] != "200" {
		t.Fatalf("attrs: %v", sp.SpanAttributes)
	}
	wantTrace := hex.EncodeToString([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	if sp.TraceID != wantTrace {
		t.Fatalf("trace id %s", sp.TraceID)
	}
	if len(batch.Events) != 1 || batch.Events[0].EventName != "exception" {
		t.Fatalf("events: %+v", batch.Events)
	}
	if batch.Events[0].EventAttributes["exception.type"] != "timeout" {
		t.Fatal("event attr")
	}
	if len(batch.Links) != 1 || batch.Links[0].LinkAttributes["link"] != "yes" {
		t.Fatalf("links: %+v", batch.Links)
	}
}

func TestFlattenUnknownServiceAndSkipEmptyIDs(t *testing.T) {
	t.Parallel()
	td := ptrace.NewTraces()
	ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
	bad := ss.Spans().AppendEmpty()
	bad.SetName("missing ids")
	ok := ss.Spans().AppendEmpty()
	ok.SetTraceID(pcommon.TraceID{1})
	ok.SetSpanID(pcommon.SpanID{1})
	ok.SetName("ok")

	batch, err := FlattenTraces(td, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Spans) != 1 {
		t.Fatalf("spans %d", len(batch.Spans))
	}
	if batch.Spans[0].ServiceName != unknownService {
		t.Fatalf("service %s", batch.Spans[0].ServiceName)
	}
}

func TestFlattenTooManySpans(t *testing.T) {
	t.Parallel()
	td := ptrace.NewTraces()
	ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
	for i := 0; i < 3; i++ {
		sp := ss.Spans().AppendEmpty()
		sp.SetTraceID(pcommon.TraceID{1})
		sp.SetSpanID(pcommon.SpanID{byte(i + 1)})
	}
	_, err := FlattenTraces(td, 2)
	if !errors.Is(err, ErrTooManySpans) {
		t.Fatalf("got %v", err)
	}
}

func TestClipAttr(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", maxAttrBytes+8)
	if got := clip(long); len(got) != maxAttrBytes {
		t.Fatalf("len %d", len(got))
	}
}
