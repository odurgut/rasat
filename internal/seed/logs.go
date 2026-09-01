package seed

import (
	"encoding/hex"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// LogRecord is one structured log matching POST /api/logs.
type LogRecord struct {
	Timestamp time.Time
	Service   string
	Level     string
	Message   string
	TraceID   string
	SpanID    string
}

type traceLog struct {
	service string
	op      string
	start   time.Time
	span    pcommon.SpanID
	failed  bool
	msg     string
}

// LogsFromTraces builds correlated INFO / WARN / ERROR lines from a trace batch.
func LogsFromTraces(td ptrace.Traces) []LogRecord {
	roots := map[pcommon.TraceID]*traceLog{}
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		service := "unknown"
		if v, ok := rs.Resource().Attributes().Get("service.name"); ok {
			service = v.AsString()
		}
		sss := rs.ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			spans := sss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				sp := spans.At(k)
				tid := sp.TraceID()
				start := sp.StartTimestamp().AsTime()
				acc, ok := roots[tid]
				if !ok {
					acc = &traceLog{start: start}
					roots[tid] = acc
				}
				root := sp.ParentSpanID().IsEmpty()
				if root || start.Before(acc.start) {
					acc.service = service
					acc.op = sp.Name()
					acc.start = start
					acc.span = sp.SpanID()
				}
				if sp.Status().Code() == ptrace.StatusCodeError {
					acc.failed = true
					if acc.msg == "" {
						acc.msg = sp.Status().Message()
					}
				}
			}
		}
	}
	out := make([]LogRecord, 0, len(roots)*2)
	for tid, acc := range roots {
		hexTID := hex.EncodeToString(tid[:])
		hexSID := hex.EncodeToString(acc.span[:])
		out = append(out, LogRecord{
			Timestamp: acc.start,
			Service:   acc.service,
			Level:     "INFO",
			Message:   acc.op,
			TraceID:   hexTID,
			SpanID:    hexSID,
		})
		if acc.failed {
			msg := acc.msg
			if msg == "" {
				msg = "failed"
			}
			out = append(out, LogRecord{
				Timestamp: acc.start.Add(time.Millisecond),
				Service:   acc.service,
				Level:     "ERROR",
				Message:   msg,
				TraceID:   hexTID,
				SpanID:    hexSID,
			})
		}
	}
	return out
}
