package store

import (
	"context"
	"fmt"
	"time"
)

// TraceWriter persists a flattened OTLP batch. HTTP and gRPC ingest share this.
type TraceWriter interface {
	WriteTraceBatch(ctx context.Context, batch TraceBatch) error
}

// TraceBatch is one OTLP export flattened onto the schema (spans + children).
type TraceBatch struct {
	Spans  []SpanRow
	Events []EventRow
	Links  []LinkRow
}

// SpanRow is one spans table row. Resource and span attributes are maps.
type SpanRow struct {
	Timestamp          time.Time
	TraceID            string
	SpanID             string
	ParentSpanID       string
	ServiceName        string
	OperationName      string
	Kind               int32
	DurationNs         uint64
	StatusCode         uint8
	StatusMessage      string
	ScopeName          string
	ScopeVersion       string
	ResourceAttributes map[string]string
	SpanAttributes     map[string]string
}

// EventRow is one span_events row. Timestamp is the parent span start (partition).
type EventRow struct {
	Timestamp       time.Time
	TraceID         string
	SpanID          string
	EventTime       time.Time
	EventName       string
	EventAttributes map[string]string
}

// LinkRow is one span_links row. Timestamp is the parent span start (partition).
type LinkRow struct {
	Timestamp      time.Time
	TraceID        string
	SpanID         string
	LinkedTraceID  string
	LinkedSpanID   string
	LinkAttributes map[string]string
}

// WriteTraceBatch inserts spans, then events, then links. ClickHouse has no
// multi-table transaction; a failure after spans may leave children missing
// until the collector retries (duplicate spans are acceptable for MergeTree).
func (s *Store) WriteTraceBatch(ctx context.Context, batch TraceBatch) error {
	if s == nil || s.conn == nil {
		return ErrNotReady
	}
	if s.batches == nil {
		return fmt.Errorf("clickhouse batch insert not configured")
	}
	if len(batch.Spans) == 0 && len(batch.Events) == 0 && len(batch.Links) == 0 {
		return nil
	}

	timeout := s.insertTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	db, err := quoteIdent(s.database)
	if err != nil {
		return err
	}

	if len(batch.Spans) > 0 {
		q := fmt.Sprintf(`INSERT INTO %s.spans (
			timestamp, trace_id, span_id, parent_span_id, service_name, operation_name,
			kind, duration_ns, status_code, status_message, scope_name, scope_version,
			resource_attributes, span_attributes
		)`, db)
		rows := make([][]any, 0, len(batch.Spans))
		for i := range batch.Spans {
			sp := &batch.Spans[i]
			rows = append(rows, []any{
				sp.Timestamp,
				sp.TraceID,
				sp.SpanID,
				sp.ParentSpanID,
				sp.ServiceName,
				sp.OperationName,
				sp.Kind,
				sp.DurationNs,
				sp.StatusCode,
				sp.StatusMessage,
				sp.ScopeName,
				sp.ScopeVersion,
				attrMap(sp.ResourceAttributes),
				attrMap(sp.SpanAttributes),
			})
		}
		if err := s.send(ctx, q, rows); err != nil {
			return fmt.Errorf("insert spans: %w", err)
		}
	}

	if len(batch.Events) > 0 {
		q := fmt.Sprintf(`INSERT INTO %s.span_events (
			timestamp, trace_id, span_id, event_time, event_name, event_attributes
		)`, db)
		rows := make([][]any, 0, len(batch.Events))
		for i := range batch.Events {
			ev := &batch.Events[i]
			rows = append(rows, []any{
				ev.Timestamp,
				ev.TraceID,
				ev.SpanID,
				ev.EventTime,
				ev.EventName,
				attrMap(ev.EventAttributes),
			})
		}
		if err := s.send(ctx, q, rows); err != nil {
			return fmt.Errorf("insert span_events: %w", err)
		}
	}

	if len(batch.Links) > 0 {
		q := fmt.Sprintf(`INSERT INTO %s.span_links (
			timestamp, trace_id, span_id, linked_trace_id, linked_span_id, link_attributes
		)`, db)
		rows := make([][]any, 0, len(batch.Links))
		for i := range batch.Links {
			lk := &batch.Links[i]
			rows = append(rows, []any{
				lk.Timestamp,
				lk.TraceID,
				lk.SpanID,
				lk.LinkedTraceID,
				lk.LinkedSpanID,
				attrMap(lk.LinkAttributes),
			})
		}
		if err := s.send(ctx, q, rows); err != nil {
			return fmt.Errorf("insert span_links: %w", err)
		}
	}

	return nil
}

func (s *Store) send(ctx context.Context, query string, rows [][]any) error {
	b, err := s.batches.PrepareBatch(ctx, query)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}
	sent := false
	defer func() {
		if !sent {
			_ = b.Abort()
		}
	}()
	for _, row := range rows {
		if err := b.Append(row...); err != nil {
			return fmt.Errorf("append: %w", err)
		}
	}
	if err := b.Send(); err != nil {
		return fmt.Errorf("send: %w", err)
	}
	sent = true
	return nil
}

func attrMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}
