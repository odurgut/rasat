package store

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// MetricPoint is one step bucket for a service.
type MetricPoint struct {
	Time      time.Time `ch:"bucket" json:"t"`
	Spans     uint64    `ch:"spans" json:"spans"`
	Errors    uint64    `ch:"errors" json:"errors"`
	Rate      float64   `ch:"-" json:"rate"`
	ErrorRate float64   `ch:"-" json:"error_rate"`
	AvgNs     uint64    `ch:"avg_ns" json:"avg_ns"`
	P50Ns     uint64    `ch:"p50_ns" json:"p50_ns"`
	P95Ns     uint64    `ch:"p95_ns" json:"p95_ns"`
	P99Ns     uint64    `ch:"p99_ns" json:"p99_ns"`
}

// ServiceSeries is one service's bucketed metrics for a dashboard chart.
type ServiceSeries struct {
	Service string        `json:"service"`
	Points  []MetricPoint `json:"points"`
}

type metricBucketRow struct {
	Service string    `ch:"service_name"`
	Time    time.Time `ch:"bucket"`
	Spans   uint64    `ch:"spans"`
	Errors  uint64    `ch:"errors"`
	AvgNs   uint64    `ch:"avg_ns"`
	P50Ns   uint64    `ch:"p50_ns"`
	P95Ns   uint64    `ch:"p95_ns"`
	P99Ns   uint64    `ch:"p99_ns"`
}

// ListMetricsSeries aggregates span counts and latency percentiles per service per step.
func (s *Store) ListMetricsSeries(ctx context.Context, q MetricsQuery) ([]ServiceSeries, error) {
	if s == nil || s.conn == nil {
		return nil, ErrNotReady
	}
	if err := q.validateSeries(); err != nil {
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

	sql, args := buildListMetricsSeriesSQL(db, q)
	var rows []metricBucketRow
	if err := s.conn.Select(ctx, &rows, sql, args...); err != nil {
		return nil, fmt.Errorf("list metrics series: %w", err)
	}
	out := groupSeries(rows, q.Start, q.End, q.Step)
	if out == nil {
		out = []ServiceSeries{}
	}
	return out, nil
}

func buildListMetricsSeriesSQL(db string, q MetricsQuery) (string, []any) {
	stepSec := int64(q.Step / time.Second)
	if stepSec < 1 {
		stepSec = 1
	}
	args := []any{stepSec, q.Start.UTC(), q.End.UTC()}
	if q.Service == "" {
		// Fleet-wide buckets for the overview. One series named "*".
		sql := `SELECT
		'*' AS service_name,
		toStartOfInterval(timestamp, toIntervalSecond(?)) AS bucket,
		count() AS spans,
		countIf(status_code = 2) AS errors,
		toUInt64(avg(duration_ns)) AS avg_ns,
		toUInt64(quantileTDigest(0.5)(duration_ns)) AS p50_ns,
		toUInt64(quantileTDigest(0.95)(duration_ns)) AS p95_ns,
		toUInt64(quantileTDigest(0.99)(duration_ns)) AS p99_ns
	FROM ` + db + `.spans
	WHERE timestamp >= ? AND timestamp < ?
	  AND service_name != ''
	GROUP BY bucket
	ORDER BY bucket ASC
	LIMIT ?`
		args = append(args, maxMetricBuckets)
		return sql, args
	}
	sql := `SELECT
		service_name,
		toStartOfInterval(timestamp, toIntervalSecond(?)) AS bucket,
		count() AS spans,
		countIf(status_code = 2) AS errors,
		toUInt64(avg(duration_ns)) AS avg_ns,
		toUInt64(quantileTDigest(0.5)(duration_ns)) AS p50_ns,
		toUInt64(quantileTDigest(0.95)(duration_ns)) AS p95_ns,
		toUInt64(quantileTDigest(0.99)(duration_ns)) AS p99_ns
	FROM ` + db + `.spans
	WHERE timestamp >= ? AND timestamp < ?
	  AND service_name != ''
	  AND service_name = ?`
	args = append(args, q.Service)
	sql += `
	GROUP BY service_name, bucket
	ORDER BY service_name ASC, bucket ASC
	LIMIT ?`
	args = append(args, maxMetricBuckets)
	return sql, args
}

func groupSeries(rows []metricBucketRow, start, end time.Time, step time.Duration) []ServiceSeries {
	if len(rows) == 0 {
		return []ServiceSeries{}
	}
	by := map[string]map[int64]metricBucketRow{}
	names := make([]string, 0, 8)
	for _, r := range rows {
		if r.Service == "" {
			continue
		}
		m, ok := by[r.Service]
		if !ok {
			m = map[int64]metricBucketRow{}
			by[r.Service] = m
			names = append(names, r.Service)
		}
		m[r.Time.UTC().Unix()] = r
	}
	sort.Strings(names)
	out := make([]ServiceSeries, 0, len(names))
	for _, name := range names {
		out = append(out, ServiceSeries{
			Service: name,
			Points:  padPoints(by[name], start, end, step),
		})
	}
	return out
}

func padPoints(byUnix map[int64]metricBucketRow, start, end time.Time, step time.Duration) []MetricPoint {
	secs := step.Seconds()
	if secs <= 0 {
		return []MetricPoint{}
	}
	t0 := alignBucket(start, step)
	n := bucketCount(t0, end, step)
	if n > maxMetricBuckets {
		n = maxMetricBuckets
	}
	out := make([]MetricPoint, 0, n)
	for t := t0; t.Before(end) && len(out) < maxMetricBuckets; t = t.Add(step) {
		p := MetricPoint{Time: t.UTC()}
		if row, ok := byUnix[t.UTC().Unix()]; ok {
			p.Spans = row.Spans
			p.Errors = row.Errors
			p.AvgNs = row.AvgNs
			p.P50Ns = row.P50Ns
			p.P95Ns = row.P95Ns
			p.P99Ns = row.P99Ns
		}
		p.Rate = float64(p.Spans) / secs
		if p.Spans > 0 {
			p.ErrorRate = float64(p.Errors) / float64(p.Spans)
		}
		out = append(out, p)
	}
	return out
}

func alignBucket(t time.Time, step time.Duration) time.Time {
	sec := int64(step / time.Second)
	if sec < 1 {
		sec = 1
	}
	u := t.UTC().Unix()
	return time.Unix((u/sec)*sec, 0).UTC()
}
