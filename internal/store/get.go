package store

import (
	"context"
	"fmt"
	"time"
)

const (
	maxTraceSpans  = 10_000
	maxTraceEvents = 50_000
	maxTraceLinks  = 50_000
)

// TraceGet is a bounded fetch of one trace. Start, End, and TraceID are required.
type TraceGet struct {
	TraceID string
	Start   time.Time
	End     time.Time
}

// TraceDetail is one trace as a flat span list in timestamp, span_id order.
// parent_span_id is enough for the client to draw a waterfall.
type TraceDetail struct {
	TraceID        string             `json:"trace_id"`
	Timestamp      time.Time          `json:"timestamp"`
	DurationNs     uint64             `json:"duration_ns"`
	SpanCount      int                `json:"span_count"`
	CriticalPath   []CriticalPathStep `json:"critical_path"`
	CriticalPathNs uint64             `json:"critical_path_ns"`
	Bottlenecks    []Bottleneck       `json:"bottlenecks"`
	Spans          []SpanDetail       `json:"spans"`
}

// SpanDetail is one span in a trace, including children for the inspector.
type SpanDetail struct {
	Timestamp          time.Time         `json:"timestamp"`
	SpanID             string            `json:"span_id"`
	ParentSpanID       string            `json:"parent_span_id"`
	Service            string            `json:"service"`
	Operation          string            `json:"operation"`
	Kind               int32             `json:"kind"`
	DurationNs         uint64            `json:"duration_ns"`
	StartOffsetNs      uint64            `json:"start_offset_ns"`
	StatusCode         uint8             `json:"status_code"`
	StatusMessage      string            `json:"status_message"`
	ScopeName          string            `json:"scope_name"`
	ScopeVersion       string            `json:"scope_version"`
	ResourceAttributes map[string]string `json:"resource_attributes"`
	SpanAttributes     map[string]string `json:"span_attributes"`
	Events             []SpanEvent       `json:"events"`
	Links              []SpanLink        `json:"links"`
}

// SpanEvent is a timed annotation on a span.
type SpanEvent struct {
	Time       time.Time         `json:"time"`
	Name       string            `json:"name"`
	Attributes map[string]string `json:"attributes"`
}

// SpanLink points at another span (possibly in another trace).
type SpanLink struct {
	TraceID    string            `json:"trace_id"`
	SpanID     string            `json:"span_id"`
	Attributes map[string]string `json:"attributes"`
}

type spanScan struct {
	Timestamp          time.Time         `ch:"timestamp"`
	TraceID            string            `ch:"trace_id"`
	SpanID             string            `ch:"span_id"`
	ParentSpanID       string            `ch:"parent_span_id"`
	ServiceName        string            `ch:"service_name"`
	OperationName      string            `ch:"operation_name"`
	Kind               int32             `ch:"kind"`
	DurationNs         uint64            `ch:"duration_ns"`
	StatusCode         uint8             `ch:"status_code"`
	StatusMessage      string            `ch:"status_message"`
	ScopeName          string            `ch:"scope_name"`
	ScopeVersion       string            `ch:"scope_version"`
	ResourceAttributes map[string]string `ch:"resource_attributes"`
	SpanAttributes     map[string]string `ch:"span_attributes"`
}

type eventScan struct {
	SpanID     string            `ch:"span_id"`
	EventTime  time.Time         `ch:"event_time"`
	EventName  string            `ch:"event_name"`
	Attributes map[string]string `ch:"event_attributes"`
}

type linkScan struct {
	SpanID         string            `ch:"span_id"`
	LinkedTraceID  string            `ch:"linked_trace_id"`
	LinkedSpanID   string            `ch:"linked_span_id"`
	LinkAttributes map[string]string `ch:"link_attributes"`
}

// GetTrace loads one trace in a time window. The window prunes partitions;
// the bloom index on trace_id is not a substitute for that bound.
func (s *Store) GetTrace(ctx context.Context, q TraceGet) (*TraceDetail, error) {
	if s == nil || s.conn == nil {
		return nil, ErrNotReady
	}
	if err := q.validate(); err != nil {
		return nil, err
	}
	db, err := quoteIdent(s.database)
	if err != nil {
		return nil, err
	}

	timeout := s.queryTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := []any{q.TraceID, q.Start.UTC(), q.End.UTC(), maxTraceSpans + 1}
	var spans []spanScan
	if err := s.conn.Select(ctx, &spans, buildGetSpansSQL(db), args...); err != nil {
		return nil, fmt.Errorf("get trace spans: %w", err)
	}
	if len(spans) == 0 {
		return nil, ErrNotFound
	}
	if len(spans) > maxTraceSpans {
		return nil, fmt.Errorf("%w: more than %d spans in this window", ErrTraceTooLarge, maxTraceSpans)
	}

	var events []eventScan
	eventArgs := []any{q.TraceID, q.Start.UTC(), q.End.UTC(), maxTraceEvents + 1}
	if err := s.conn.Select(ctx, &events, buildGetEventsSQL(db), eventArgs...); err != nil {
		return nil, fmt.Errorf("get trace events: %w", err)
	}
	if len(events) > maxTraceEvents {
		return nil, fmt.Errorf("%w: more than %d events in this window", ErrTraceTooLarge, maxTraceEvents)
	}

	var links []linkScan
	linkArgs := []any{q.TraceID, q.Start.UTC(), q.End.UTC(), maxTraceLinks + 1}
	if err := s.conn.Select(ctx, &links, buildGetLinksSQL(db), linkArgs...); err != nil {
		return nil, fmt.Errorf("get trace links: %w", err)
	}
	if len(links) > maxTraceLinks {
		return nil, fmt.Errorf("%w: more than %d links in this window", ErrTraceTooLarge, maxTraceLinks)
	}

	return assembleTrace(q.TraceID, spans, events, links), nil
}

func (q TraceGet) validate() error {
	if q.TraceID == "" {
		return fmt.Errorf("%w: trace id is required", ErrInvalidSearch)
	}
	if q.Start.IsZero() || q.End.IsZero() || !q.End.After(q.Start) {
		return fmt.Errorf("%w: start and end are required and end must be after start", ErrInvalidSearch)
	}
	return nil
}

func buildGetSpansSQL(db string) string {
	return `SELECT
		timestamp, trace_id, span_id, parent_span_id,
		service_name, operation_name, kind, duration_ns,
		status_code, status_message, scope_name, scope_version,
		resource_attributes, span_attributes
	FROM ` + db + `.spans
	WHERE trace_id = ? AND timestamp >= ? AND timestamp < ?
	ORDER BY timestamp, span_id
	LIMIT ?`
}

func buildGetEventsSQL(db string) string {
	return `SELECT
		span_id, event_time, event_name, event_attributes
	FROM ` + db + `.span_events
	WHERE trace_id = ? AND timestamp >= ? AND timestamp < ?
	ORDER BY span_id, event_time
	LIMIT ?`
}

func buildGetLinksSQL(db string) string {
	return `SELECT
		span_id, linked_trace_id, linked_span_id, link_attributes
	FROM ` + db + `.span_links
	WHERE trace_id = ? AND timestamp >= ? AND timestamp < ?
	ORDER BY span_id
	LIMIT ?`
}

func assembleTrace(traceID string, spans []spanScan, events []eventScan, links []linkScan) *TraceDetail {
	bySpanEvents := make(map[string][]SpanEvent, len(spans))
	for _, e := range events {
		bySpanEvents[e.SpanID] = append(bySpanEvents[e.SpanID], SpanEvent{
			Time:       e.EventTime,
			Name:       e.EventName,
			Attributes: nonNilMap(e.Attributes),
		})
	}
	bySpanLinks := make(map[string][]SpanLink, len(spans))
	for _, l := range links {
		bySpanLinks[l.SpanID] = append(bySpanLinks[l.SpanID], SpanLink{
			TraceID:    l.LinkedTraceID,
			SpanID:     l.LinkedSpanID,
			Attributes: nonNilMap(l.LinkAttributes),
		})
	}

	out := make([]SpanDetail, 0, len(spans))
	minTs := spans[0].Timestamp
	maxEnd := spanEnd(spans[0])
	for _, sp := range spans {
		if sp.Timestamp.Before(minTs) {
			minTs = sp.Timestamp
		}
		if end := spanEnd(sp); end.After(maxEnd) {
			maxEnd = end
		}
		ev := bySpanEvents[sp.SpanID]
		if ev == nil {
			ev = []SpanEvent{}
		}
		ln := bySpanLinks[sp.SpanID]
		if ln == nil {
			ln = []SpanLink{}
		}
		out = append(out, SpanDetail{
			Timestamp:          sp.Timestamp,
			SpanID:             sp.SpanID,
			ParentSpanID:       sp.ParentSpanID,
			Service:            sp.ServiceName,
			Operation:          sp.OperationName,
			Kind:               sp.Kind,
			DurationNs:         sp.DurationNs,
			StatusCode:         sp.StatusCode,
			StatusMessage:      sp.StatusMessage,
			ScopeName:          sp.ScopeName,
			ScopeVersion:       sp.ScopeVersion,
			ResourceAttributes: nonNilMap(sp.ResourceAttributes),
			SpanAttributes:     nonNilMap(sp.SpanAttributes),
			Events:             ev,
			Links:              ln,
		})
	}

	dur := uint64(0)
	if maxEnd.After(minTs) {
		dur = uint64(maxEnd.Sub(minTs))
	}
	for i := range out {
		d := out[i].Timestamp.Sub(minTs)
		if d < 0 {
			d = 0
		}
		out[i].StartOffsetNs = uint64(d)
	}
	d := &TraceDetail{
		TraceID:    traceID,
		Timestamp:  minTs,
		DurationNs: dur,
		SpanCount:  len(out),
		Spans:      out,
	}
	attachCriticalPath(d)
	attachBottlenecks(d)
	return d
}

func spanEnd(sp spanScan) time.Time {
	return sp.Timestamp.Add(time.Duration(sp.DurationNs))
}

func nonNilMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}
