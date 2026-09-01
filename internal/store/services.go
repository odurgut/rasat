package store

import (
	"context"
	"fmt"
	"time"
)

// ServiceList is a bounded catalog query. Start, End, and Limit are required.
type ServiceList struct {
	Start time.Time
	End   time.Time
	Limit int
}

// ServiceRow is one discovered service in the window (from spans, same source as the map).
type ServiceRow struct {
	Service  string    `ch:"service_name" json:"service"`
	LastSeen time.Time `ch:"last_seen" json:"last_seen"`
	Spans    uint64    `ch:"spans" json:"spans"`
	Errors   uint64    `ch:"errors" json:"errors"`
}

// ListServices lists distinct services from spans in the window.
// The services ReplacingMergeTree stays for later high-ingest; the UI catalog
// must match the map, which already reads spans.
func (s *Store) ListServices(ctx context.Context, q ServiceList) ([]ServiceRow, error) {
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

	sql := buildListServicesSQL(db)
	args := []any{q.Start.UTC(), q.End.UTC(), q.Limit}
	var rows []ServiceRow
	if err := s.conn.Select(ctx, &rows, sql, args...); err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	if rows == nil {
		rows = []ServiceRow{}
	}
	return rows, nil
}

func (q ServiceList) validate() error {
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

func buildListServicesSQL(db string) string {
	return `SELECT
		sp.service_name AS service_name,
		max(sp.timestamp) AS last_seen,
		count() AS spans,
		countIf(sp.status_code = 2) AS errors
	FROM ` + db + `.spans AS sp
	WHERE sp.timestamp >= ? AND sp.timestamp < ?
	  AND sp.service_name != ''
	GROUP BY sp.service_name
	ORDER BY last_seen DESC, service_name ASC
	LIMIT ?`
}
