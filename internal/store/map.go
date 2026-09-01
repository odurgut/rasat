package store

import (
	"context"
	"fmt"
	"time"
)

// ServiceMapQuery is a bounded graph query. Start, End, and Limit are required.
type ServiceMapQuery struct {
	Start time.Time
	End   time.Time
	Limit int
}

// ServiceMapNode is one service in the window.
type ServiceMapNode struct {
	Service string `ch:"service_name" json:"service"`
	Spans   uint64 `ch:"spans" json:"spans"`
	Errors  uint64 `ch:"errors" json:"errors"`
}

// ServiceMapEdge is parent→child calls between different services.
type ServiceMapEdge struct {
	From          string `ch:"parent_service" json:"from"`
	To            string `ch:"child_service" json:"to"`
	Calls         uint64 `ch:"calls" json:"calls"`
	Errors        uint64 `ch:"errors" json:"errors"`
	AvgDurationNs uint64 `ch:"avg_duration_ns" json:"avg_duration_ns"`
}

// ServiceMapGraph is nodes plus edges for the map UI.
type ServiceMapGraph struct {
	Nodes []ServiceMapNode `json:"nodes"`
	Edges []ServiceMapEdge `json:"edges"`
}

// ServiceMap reads cross-service parent/child edges from spans in the window.
func (s *Store) ServiceMap(ctx context.Context, q ServiceMapQuery) (*ServiceMapGraph, error) {
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

	start, end := q.Start.UTC(), q.End.UTC()
	var nodes []ServiceMapNode
	if err := s.conn.Select(ctx, &nodes, buildServiceMapNodesSQL(db), start, end, q.Limit); err != nil {
		return nil, fmt.Errorf("service map nodes: %w", err)
	}
	if nodes == nil {
		nodes = []ServiceMapNode{}
	}

	edgeLimit := q.Limit * 4
	if edgeLimit > maxSearchLimit {
		edgeLimit = maxSearchLimit
	}
	var edges []ServiceMapEdge
	if err := s.conn.Select(ctx, &edges, buildServiceMapEdgesSQL(db), start, end, start, end, edgeLimit); err != nil {
		return nil, fmt.Errorf("service map edges: %w", err)
	}
	if edges == nil {
		edges = []ServiceMapEdge{}
	}
	return &ServiceMapGraph{Nodes: nodes, Edges: edges}, nil
}

func (q ServiceMapQuery) validate() error {
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

func buildServiceMapNodesSQL(db string) string {
	return `SELECT
		service_name,
		count() AS spans,
		countIf(status_code = 2) AS errors
	FROM ` + db + `.spans
	WHERE timestamp >= ? AND timestamp < ?
	  AND service_name != ''
	GROUP BY service_name
	ORDER BY spans DESC, service_name ASC
	LIMIT ?`
}

func buildServiceMapEdgesSQL(db string) string {
	return `SELECT
		parent.service_name AS parent_service,
		child.service_name AS child_service,
		count() AS calls,
		countIf(child.status_code = 2) AS errors,
		toUInt64(avg(child.duration_ns)) AS avg_duration_ns
	FROM ` + db + `.spans AS child
	INNER JOIN ` + db + `.spans AS parent
		ON parent.trace_id = child.trace_id
	   AND parent.span_id = child.parent_span_id
	WHERE child.timestamp >= ? AND child.timestamp < ?
	  AND parent.timestamp >= ? AND parent.timestamp < ?
	  AND parent.service_name != ''
	  AND child.service_name != ''
	  AND parent.service_name != child.service_name
	GROUP BY parent.service_name, child.service_name
	ORDER BY calls DESC, parent_service ASC, child_service ASC
	LIMIT ?`
}
