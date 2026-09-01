package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrInvalidSearch is returned when a search is missing a time window or limit.
var ErrInvalidSearch = errors.New("invalid search")

const maxSearchLimit = 1000

// TraceSearch is a bounded trace list query. Start, End, and Limit are required.
type TraceSearch struct {
	Start         time.Time
	End           time.Time
	Limit         int
	Service       string
	Operation     string
	TraceID       string
	MinDurationNs *uint64
	MaxDurationNs *uint64
	StatusCode    *uint8
	AttrKey       string
	AttrValue     string
}

// TraceListRow is one search hit: a trace summarized from its spans.
type TraceListRow struct {
	TraceID    string    `ch:"trace_id" json:"trace_id"`
	Service    string    `ch:"service_name" json:"service"`
	Operation  string    `ch:"operation_name" json:"operation"`
	DurationNs uint64    `ch:"duration_ns" json:"duration_ns"`
	SpanCount  uint64    `ch:"span_count" json:"span_count"`
	Timestamp  time.Time `ch:"timestamp" json:"timestamp"`
	StatusCode uint8     `ch:"status_code" json:"status_code"`
}

// SearchTraces lists traces that contain at least one matching span in the window.
// Aggregates (span count, duration, root service/op) use all spans of those traces
// in the same window so the list row is the whole trace, not the filtered subset.
func (s *Store) SearchTraces(ctx context.Context, q TraceSearch) ([]TraceListRow, error) {
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

	sql, args := buildSearchSQL(db, q)
	var rows []TraceListRow
	if err := s.conn.Select(ctx, &rows, sql, args...); err != nil {
		return nil, fmt.Errorf("search traces: %w", err)
	}
	if rows == nil {
		rows = []TraceListRow{}
	}
	return rows, nil
}

func (q TraceSearch) validate() error {
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

func buildSearchSQL(db string, q TraceSearch) (string, []any) {
	table := db + ".spans"
	var inner strings.Builder
	args := make([]any, 0, 16)

	inner.WriteString("SELECT c.trace_id FROM ")
	inner.WriteString(table)
	inner.WriteString(" AS c WHERE c.timestamp >= ? AND c.timestamp < ?")
	args = append(args, q.Start.UTC(), q.End.UTC())

	if q.Service != "" {
		inner.WriteString(" AND c.service_name = ?")
		args = append(args, q.Service)
	}
	if q.Operation != "" {
		inner.WriteString(" AND c.operation_name = ?")
		args = append(args, q.Operation)
	}
	if q.TraceID != "" {
		inner.WriteString(" AND c.trace_id = ?")
		args = append(args, q.TraceID)
	}
	if q.MinDurationNs != nil {
		inner.WriteString(" AND c.duration_ns >= ?")
		args = append(args, *q.MinDurationNs)
	}
	if q.MaxDurationNs != nil {
		inner.WriteString(" AND c.duration_ns <= ?")
		args = append(args, *q.MaxDurationNs)
	}
	if q.StatusCode != nil {
		inner.WriteString(" AND c.status_code = ?")
		args = append(args, *q.StatusCode)
	}
	if q.AttrKey != "" {
		inner.WriteString(" AND (c.span_attributes[?] = ? OR c.resource_attributes[?] = ?)")
		args = append(args, q.AttrKey, q.AttrValue, q.AttrKey, q.AttrValue)
	}

	inner.WriteString(" GROUP BY c.trace_id ORDER BY min(c.timestamp) DESC LIMIT ?")
	args = append(args, q.Limit)

	outerArgs := []any{q.Start.UTC(), q.End.UTC()}
	outerArgs = append(outerArgs, args...)

	sql := fmt.Sprintf(`SELECT
		s.trace_id AS trace_id,
		min(s.timestamp) AS timestamp,
		count() AS span_count,
		toUInt64(greatest(0, max(toUnixTimestamp64Nano(s.timestamp) + toInt64(s.duration_ns)) - min(toUnixTimestamp64Nano(s.timestamp)))) AS duration_ns,
		argMin(s.service_name, tuple(s.parent_span_id != '', s.timestamp)) AS service_name,
		argMin(s.operation_name, tuple(s.parent_span_id != '', s.timestamp)) AS operation_name,
		max(s.status_code) AS status_code
	FROM %s AS s
	WHERE s.timestamp >= ? AND s.timestamp < ?
	  AND s.trace_id IN (%s)
	GROUP BY s.trace_id
	ORDER BY timestamp DESC`, table, inner.String())

	return sql, outerArgs
}
