package seed

import (
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestLogsFromTracesCorrelates(t *testing.T) {
	t.Parallel()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "checkout")
	sp := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	var tid pcommon.TraceID
	tid[0] = 0xab
	var sid pcommon.SpanID
	sid[0] = 0xcd
	sp.SetTraceID(tid)
	sp.SetSpanID(sid)
	sp.SetName("HTTP POST /checkout")
	start := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	sp.SetStartTimestamp(pcommon.NewTimestampFromTime(start))
	sp.SetEndTimestamp(pcommon.NewTimestampFromTime(start.Add(time.Millisecond)))
	sp.Status().SetCode(ptrace.StatusCodeError)
	sp.Status().SetMessage("database timeout")

	rows := LogsFromTraces(td)
	if len(rows) != 2 {
		t.Fatalf("rows %d", len(rows))
	}
	info, errLog := rows[0], rows[1]
	if info.Level != "INFO" {
		info, errLog = rows[1], rows[0]
	}
	if info.Service != "checkout" || info.Level != "INFO" || info.Message != "HTTP POST /checkout" {
		t.Fatalf("info %+v", info)
	}
	if errLog.Level != "ERROR" || errLog.Message != "database timeout" || errLog.TraceID != info.TraceID {
		t.Fatalf("err %+v", errLog)
	}
	if info.TraceID == "" || info.SpanID == "" {
		t.Fatal("ids")
	}
}
