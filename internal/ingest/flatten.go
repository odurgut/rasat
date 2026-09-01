// Package ingest receives OTLP traces and writes them through store.TraceWriter.
package ingest

import (
	"encoding/hex"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/odurgut/rasat/internal/store"
)

const (
	unknownService     = "unknown_service"
	maxAttrBytes       = 16 << 10
	maxSpansPerRequest = 100_000
)

// ErrTooManySpans is returned when an OTLP export exceeds the span cap.
var ErrTooManySpans = errors.New("too many spans in request")

// FlattenTraces maps OTLP pdata onto the ClickHouse schema. HTTP and gRPC share this.
func FlattenTraces(td ptrace.Traces, maxSpans int) (store.TraceBatch, error) {
	if maxSpans <= 0 {
		maxSpans = maxSpansPerRequest
	}

	var batch store.TraceBatch
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		resAttrs := mapFrom(rs.Resource().Attributes())
		service := serviceName(resAttrs)

		sss := rs.ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			ss := sss.At(j)
			scope := ss.Scope()
			spans := ss.Spans()
			for k := 0; k < spans.Len(); k++ {
				if len(batch.Spans) >= maxSpans {
					return store.TraceBatch{}, fmt.Errorf("%w: limit %d", ErrTooManySpans, maxSpans)
				}
				sp := spans.At(k)
				tid := sp.TraceID()
				sid := sp.SpanID()
				if tid.IsEmpty() || sid.IsEmpty() {
					continue
				}

				start := sp.StartTimestamp()
				end := sp.EndTimestamp()
				var duration uint64
				if end > start {
					duration = uint64(end - start)
				}

				parent := ""
				if ps := sp.ParentSpanID(); !ps.IsEmpty() {
					parent = hex.EncodeToString(ps[:])
				}

				ts := start.AsTime()
				row := store.SpanRow{
					Timestamp:          ts,
					TraceID:            hex.EncodeToString(tid[:]),
					SpanID:             hex.EncodeToString(sid[:]),
					ParentSpanID:       parent,
					ServiceName:        service,
					OperationName:      sp.Name(),
					Kind:               int32(sp.Kind()),
					DurationNs:         duration,
					StatusCode:         uint8(sp.Status().Code()),
					StatusMessage:      sp.Status().Message(),
					ScopeName:          scope.Name(),
					ScopeVersion:       scope.Version(),
					ResourceAttributes: resAttrs,
					SpanAttributes:     mapFrom(sp.Attributes()),
				}
				batch.Spans = append(batch.Spans, row)

				evs := sp.Events()
				for n := 0; n < evs.Len(); n++ {
					ev := evs.At(n)
					et := ev.Timestamp().AsTime()
					if ev.Timestamp() == 0 {
						et = ts
					}
					batch.Events = append(batch.Events, store.EventRow{
						Timestamp:       ts,
						TraceID:         row.TraceID,
						SpanID:          row.SpanID,
						EventTime:       et,
						EventName:       ev.Name(),
						EventAttributes: mapFrom(ev.Attributes()),
					})
				}

				lks := sp.Links()
				for n := 0; n < lks.Len(); n++ {
					lk := lks.At(n)
					lt := lk.TraceID()
					ls := lk.SpanID()
					batch.Links = append(batch.Links, store.LinkRow{
						Timestamp:      ts,
						TraceID:        row.TraceID,
						SpanID:         row.SpanID,
						LinkedTraceID:  hex.EncodeToString(lt[:]),
						LinkedSpanID:   hex.EncodeToString(ls[:]),
						LinkAttributes: mapFrom(lk.Attributes()),
					})
				}
			}
		}
	}
	return batch, nil
}

func serviceName(attrs map[string]string) string {
	if v := attrs["service.name"]; v != "" {
		return v
	}
	return unknownService
}

func mapFrom(m pcommon.Map) map[string]string {
	out := make(map[string]string, m.Len())
	m.Range(func(k string, v pcommon.Value) bool {
		if k == "" {
			return true
		}
		out[k] = clip(v.AsString())
		return true
	})
	return out
}

func clip(s string) string {
	if len(s) <= maxAttrBytes {
		return s
	}
	return s[:maxAttrBytes]
}
