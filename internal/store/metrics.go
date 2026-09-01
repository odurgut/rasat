package store

import (
	"context"
	"fmt"
	"time"
)

// maxMetricBuckets caps time-series points per query (window / step).
const maxMetricBuckets = 360

// MetricsQuery is a bounded per-service aggregation. Start, End, and Limit are required.
// Service, if set, restricts the result to that name.
// Step, if set, requests a bucketed series (dashboards); it must yield at most maxMetricBuckets.
type MetricsQuery struct {
	Start   time.Time
	End     time.Time
	Limit   int
	Service string
	Step    time.Duration
}

// ServiceMetrics is one service's derived rates and latency from spans in the window.
// Rate is spans per second. ErrorRate is errors / spans (0 when spans is 0).
type ServiceMetrics struct {
	Service   string  `ch:"service_name" json:"service"`
	Spans     uint64  `ch:"spans" json:"spans"`
	Errors    uint64  `ch:"errors" json:"errors"`
	Rate      float64 `ch:"-" json:"rate"`
	ErrorRate float64 `ch:"-" json:"error_rate"`
	AvgNs     uint64  `ch:"avg_ns" json:"avg_ns"`
	P50Ns     uint64  `ch:"p50_ns" json:"p50_ns"`
	P95Ns     uint64  `ch:"p95_ns" json:"p95_ns"`
	P99Ns     uint64  `ch:"p99_ns" json:"p99_ns"`
}

// ListMetrics aggregates span counts and latency percentiles per service in the window.
func (s *Store) ListMetrics(ctx context.Context, q MetricsQuery) ([]ServiceMetrics, error) {
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

	sql, args := buildListMetricsSQL(db, q)
	var rows []ServiceMetrics
	if err := s.conn.Select(ctx, &rows, sql, args...); err != nil {
		return nil, fmt.Errorf("list metrics: %w", err)
	}
	if rows == nil {
		rows = []ServiceMetrics{}
	}
	fillRates(rows, q.End.Sub(q.Start))
	return rows, nil
}

func (q MetricsQuery) validate() error {
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

func (q MetricsQuery) validateSeries() error {
	if err := q.validate(); err != nil {
		return err
	}
	if q.Step < time.Second {
		return fmt.Errorf("%w: step must be >= 1s", ErrInvalidSearch)
	}
	n := bucketCount(q.Start, q.End, q.Step)
	if n > maxMetricBuckets {
		return fmt.Errorf("%w: step is too small for the window (max %d buckets)", ErrInvalidSearch, maxMetricBuckets)
	}
	return nil
}

func bucketCount(start, end time.Time, step time.Duration) int {
	if step <= 0 || !end.After(start) {
		return 0
	}
	n := int(end.Sub(start) / step)
	if end.Sub(start)%step != 0 {
		n++
	}
	if n < 1 {
		return 1
	}
	return n
}

func fillRates(rows []ServiceMetrics, window time.Duration) {
	secs := window.Seconds()
	if secs <= 0 {
		return
	}
	for i := range rows {
		rows[i].Rate = float64(rows[i].Spans) / secs
		if rows[i].Spans > 0 {
			rows[i].ErrorRate = float64(rows[i].Errors) / float64(rows[i].Spans)
		}
	}
}

func buildListMetricsSQL(db string, q MetricsQuery) (string, []any) {
	args := []any{q.Start.UTC(), q.End.UTC()}
	sql := `SELECT
		service_name,
		count() AS spans,
		countIf(status_code = 2) AS errors,
		toUInt64(avg(duration_ns)) AS avg_ns,
		toUInt64(quantileTDigest(0.5)(duration_ns)) AS p50_ns,
		toUInt64(quantileTDigest(0.95)(duration_ns)) AS p95_ns,
		toUInt64(quantileTDigest(0.99)(duration_ns)) AS p99_ns
	FROM ` + db + `.spans
	WHERE timestamp >= ? AND timestamp < ?
	  AND service_name != ''`
	if q.Service != "" {
		sql += `
	  AND service_name = ?`
		args = append(args, q.Service)
	}
	sql += `
	GROUP BY service_name
	ORDER BY spans DESC, service_name ASC
	LIMIT ?`
	args = append(args, q.Limit)
	return sql, args
}
