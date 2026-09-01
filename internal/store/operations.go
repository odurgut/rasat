package store

import (
	"context"
	"fmt"
	"time"
)

// OperationList is a bounded operation catalog for one service.
// Start, End, Limit, and Service are required — same cut as Jaeger GetOperations.
type OperationList struct {
	Start   time.Time
	End     time.Time
	Limit   int
	Service string
}

// OperationRow is one distinct span name for a service in the window,
// with volume, error rate, and latency percentiles (same cut as metrics).
type OperationRow struct {
	Operation string  `ch:"operation_name" json:"operation"`
	Spans     uint64  `ch:"spans" json:"spans"`
	Errors    uint64  `ch:"errors" json:"errors"`
	ErrorRate float64 `ch:"-" json:"error_rate"`
	P50Ns     uint64  `ch:"p50_ns" json:"p50_ns"`
	P95Ns     uint64  `ch:"p95_ns" json:"p95_ns"`
}

// ListOperations lists distinct operations for one service from spans in the window,
// with volume, error rate, and p50/p95 (same aggregations as metrics).
func (s *Store) ListOperations(ctx context.Context, q OperationList) ([]OperationRow, error) {
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

	sql := buildListOperationsSQL(db)
	args := []any{q.Start.UTC(), q.End.UTC(), q.Service, q.Limit}
	var rows []OperationRow
	if err := s.conn.Select(ctx, &rows, sql, args...); err != nil {
		return nil, fmt.Errorf("list operations: %w", err)
	}
	if rows == nil {
		rows = []OperationRow{}
	}
	fillOpRates(rows)
	return rows, nil
}

func (q OperationList) validate() error {
	if q.Service == "" {
		return fmt.Errorf("%w: service is required", ErrInvalidSearch)
	}
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

func fillOpRates(rows []OperationRow) {
	for i := range rows {
		if rows[i].Spans > 0 {
			rows[i].ErrorRate = float64(rows[i].Errors) / float64(rows[i].Spans)
		}
	}
}

func buildListOperationsSQL(db string) string {
	return `SELECT
		sp.operation_name AS operation_name,
		count() AS spans,
		countIf(status_code = 2) AS errors,
		toUInt64(quantileTDigest(0.5)(duration_ns)) AS p50_ns,
		toUInt64(quantileTDigest(0.95)(duration_ns)) AS p95_ns
	FROM ` + db + `.spans AS sp
	WHERE sp.timestamp >= ? AND sp.timestamp < ?
	  AND sp.service_name = ?
	  AND sp.operation_name != ''
	GROUP BY sp.operation_name
	ORDER BY spans DESC, operation_name ASC
	LIMIT ?`
}
