package seed

import (
	"fmt"
	"math/rand"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type event struct {
	name  string
	at    time.Time
	attrs map[string]string
}

type link struct {
	trace pcommon.TraceID
	span  pcommon.SpanID
	attrs map[string]string
}

type spanIn struct {
	tid     pcommon.TraceID
	service string
	name    string
	kind    ptrace.SpanKind
	parent  pcommon.SpanID
	start   time.Time
	end     time.Time
	code    ptrace.StatusCode
	msg     string
	attrs   map[string]string
	events  []event
	links   []link
}

type emitter struct {
	td  ptrace.Traces
	rng *rand.Rand
}

func (e *emitter) span(in spanIn) pcommon.SpanID {
	sid := randSpan(e.rng)
	meta := lookupService(in.service)
	rs := e.td.ResourceSpans().AppendEmpty()
	res := rs.Resource().Attributes()
	res.PutStr("service.name", meta.name)
	res.PutStr("service.version", meta.version)
	res.PutStr("service.namespace", meta.namespace)
	res.PutStr("deployment.environment", "dev")
	res.PutStr("telemetry.sdk.language", meta.lang)
	res.PutStr("host.name", fmt.Sprintf("%s-%04x", meta.name, e.rng.Intn(0xffff)))
	res.PutStr("k8s.namespace.name", "shop")
	res.PutStr("k8s.pod.name", fmt.Sprintf("%s-7d9c-%04x", meta.name, e.rng.Intn(0xffff)))
	res.PutStr("cloud.region", "eu-central-1")

	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName(meta.scope)
	ss.Scope().SetVersion(meta.scopeVer)
	sp := ss.Spans().AppendEmpty()
	sp.SetTraceID(in.tid)
	sp.SetSpanID(sid)
	if !in.parent.IsEmpty() {
		sp.SetParentSpanID(in.parent)
	}
	sp.SetName(in.name)
	sp.SetKind(in.kind)
	sp.SetStartTimestamp(pcommon.NewTimestampFromTime(in.start))
	sp.SetEndTimestamp(pcommon.NewTimestampFromTime(in.end))
	sp.Status().SetCode(in.code)
	sp.Status().SetMessage(in.msg)
	for k, v := range in.attrs {
		sp.Attributes().PutStr(k, v)
	}
	for _, ev := range in.events {
		evn := sp.Events().AppendEmpty()
		evn.SetName(ev.name)
		evn.SetTimestamp(pcommon.NewTimestampFromTime(ev.at))
		for k, v := range ev.attrs {
			evn.Attributes().PutStr(k, v)
		}
	}
	for _, lk := range in.links {
		l := sp.Links().AppendEmpty()
		l.SetTraceID(lk.trace)
		l.SetSpanID(lk.span)
		for k, v := range lk.attrs {
			l.Attributes().PutStr(k, v)
		}
	}
	return sid
}

func ms(n int) time.Duration {
	return time.Duration(n) * time.Millisecond
}
