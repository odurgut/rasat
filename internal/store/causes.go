package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// maxErrorCauses caps ranked causes per query in a window.
const maxErrorCauses = 8

// ErrorCausesQuery is a bounded aggregation of error span causes.
// Start, End, and Limit are required. Service, if set, restricts the rank to that name.
type ErrorCausesQuery struct {
	Start   time.Time
	End     time.Time
	Limit   int
	Service string
}

// ErrorCause is one status_message / exception.type ranked by error-span count.
// FirstSeen is the earliest error timestamp for that cause in the window.
type ErrorCause struct {
	Cause     string    `ch:"cause" json:"cause"`
	Count     uint64    `ch:"n" json:"count"`
	FirstSeen time.Time `ch:"first_seen" json:"first_seen,omitempty"`
}

// ListErrorCauses ranks error spans: exception.type, else status_message, else "error".
func (s *Store) ListErrorCauses(ctx context.Context, q ErrorCausesQuery) ([]ErrorCause, error) {
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

	sql, args := buildListErrorCausesSQL(db, q)
	var rows []ErrorCause
	if err := s.conn.Select(ctx, &rows, sql, args...); err != nil {
		return nil, fmt.Errorf("list error causes: %w", err)
	}
	if rows == nil {
		rows = []ErrorCause{}
	}
	return rows, nil
}

func (q ErrorCausesQuery) validate() error {
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

func buildListErrorCausesSQL(db string, q ErrorCausesQuery) (string, []any) {
	n := q.Limit
	if n > maxErrorCauses {
		n = maxErrorCauses
	}
	var b strings.Builder
	b.WriteString("SELECT cause, count() AS n, min(ts) AS first_seen FROM (")
	b.WriteString("SELECT if(e.ex != '', e.ex, if(s.status_message != '', s.status_message, 'error')) AS cause, s.timestamp AS ts ")
	b.WriteString("FROM ")
	b.WriteString(db)
	b.WriteString(".spans AS s LEFT JOIN (")
	b.WriteString("SELECT trace_id, span_id, any(event_attributes['exception.type']) AS ex FROM ")
	b.WriteString(db)
	b.WriteString(".span_events WHERE timestamp >= ? AND timestamp < ? AND event_name = 'exception' GROUP BY trace_id, span_id")
	b.WriteString(") AS e ON e.trace_id = s.trace_id AND e.span_id = s.span_id ")
	b.WriteString("WHERE s.timestamp >= ? AND s.timestamp < ? AND s.status_code = 2")
	args := []any{q.Start, q.End, q.Start, q.End}
	if svc := strings.TrimSpace(q.Service); svc != "" {
		b.WriteString(" AND s.service_name = ?")
		args = append(args, svc)
	}
	b.WriteString(") GROUP BY cause ORDER BY n DESC, cause ASC LIMIT ?")
	args = append(args, n)
	return b.String(), args
}
