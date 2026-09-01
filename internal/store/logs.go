package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// LogWriter persists a structured log batch. JSON ingest (and later OTLP logs)
// share this.
type LogWriter interface {
	WriteLogBatch(ctx context.Context, rows []LogRow) error
}

// LogRow is one logs table row. Search and the live stream share this shape.
type LogRow struct {
	Timestamp   time.Time `ch:"timestamp" json:"timestamp"`
	ServiceName string    `ch:"service_name" json:"service"`
	Level       string    `ch:"level" json:"level"`
	Message     string    `ch:"message" json:"message"`
	TraceID     string    `ch:"trace_id" json:"trace_id"`
	SpanID      string    `ch:"span_id" json:"span_id"`
}

// WriteLogBatch inserts log rows. Empty input is a no-op.
func (s *Store) WriteLogBatch(ctx context.Context, rows []LogRow) error {
	if s == nil || s.conn == nil {
		return ErrNotReady
	}
	if s.batches == nil {
		return fmt.Errorf("clickhouse batch insert not configured")
	}
	if len(rows) == 0 {
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

	q := fmt.Sprintf(`INSERT INTO %s.logs (
		timestamp, service_name, level, message, trace_id, span_id
	)`, db)
	out := make([][]any, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		out = append(out, []any{
			row.Timestamp,
			row.ServiceName,
			row.Level,
			row.Message,
			row.TraceID,
			row.SpanID,
		})
	}
	if err := s.send(ctx, q, out); err != nil {
		return fmt.Errorf("insert logs: %w", err)
	}
	return nil
}

// LogSearch is a bounded log list query. Start, End, and Limit are required.
type LogSearch struct {
	Start   time.Time
	End     time.Time
	Limit   int
	Service string
	Level   string
	TraceID string
	SpanID  string
}

// SearchLogs lists log rows in the window, newest first.
func (s *Store) SearchLogs(ctx context.Context, q LogSearch) ([]LogRow, error) {
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

	sql, args := buildLogSearchSQL(db, q)
	var rows []LogRow
	if err := s.conn.Select(ctx, &rows, sql, args...); err != nil {
		return nil, fmt.Errorf("search logs: %w", err)
	}
	if rows == nil {
		rows = []LogRow{}
	}
	return rows, nil
}

func (q LogSearch) validate() error {
	if q.Start.IsZero() || q.End.IsZero() || !q.End.After(q.Start) {
		return fmt.Errorf("%w: start and end are required and end must be after start", ErrInvalidSearch)
	}
	if q.Limit < 1 {
		return fmt.Errorf("%w: limit is required", ErrInvalidSearch)
	}
	if q.Limit > maxSearchLimit {
		return fmt.Errorf("%w: limit must be <= %d", ErrInvalidSearch, maxSearchLimit)
	}
	return nil
}

func buildLogSearchSQL(db string, q LogSearch) (string, []any) {
	var b strings.Builder
	args := make([]any, 0, 8)
	b.WriteString("SELECT timestamp, service_name, level, message, trace_id, span_id FROM ")
	b.WriteString(db)
	b.WriteString(".logs WHERE timestamp >= ? AND timestamp < ?")
	args = append(args, q.Start.UTC(), q.End.UTC())
	if q.Service != "" {
		b.WriteString(" AND service_name = ?")
		args = append(args, q.Service)
	}
	if q.Level != "" {
		b.WriteString(" AND level = ?")
		args = append(args, q.Level)
	}
	if q.TraceID != "" {
		b.WriteString(" AND trace_id = ?")
		args = append(args, q.TraceID)
	}
	if q.SpanID != "" {
		b.WriteString(" AND span_id = ?")
		args = append(args, q.SpanID)
	}
	b.WriteString(" ORDER BY timestamp DESC LIMIT ?")
	args = append(args, q.Limit)
	return b.String(), args
}
