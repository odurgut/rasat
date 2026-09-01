package stream

import (
	"time"

	"github.com/odurgut/rasat/internal/store"
)

// SummarizeBatch builds list-shaped events from an in-memory ingest batch.
// It does not query ClickHouse. A later export of the same trace_id may
// produce a fuller row; the UI replaces by id.
func SummarizeBatch(batch store.TraceBatch) []store.TraceListRow {
	type acc struct {
		minTs     time.Time
		maxEnd    time.Time
		n         uint64
		maxStatus uint8
		root      store.SpanRow
		hasRoot   bool
	}

	by := make(map[string]*acc)
	order := make([]string, 0)
	for _, sp := range batch.Spans {
		if sp.TraceID == "" {
			continue
		}
		a, ok := by[sp.TraceID]
		if !ok {
			a = &acc{}
			by[sp.TraceID] = a
			order = append(order, sp.TraceID)
		}
		a.n++
		if a.minTs.IsZero() || sp.Timestamp.Before(a.minTs) {
			a.minTs = sp.Timestamp
		}
		end := sp.Timestamp.Add(time.Duration(sp.DurationNs))
		if a.maxEnd.IsZero() || end.After(a.maxEnd) {
			a.maxEnd = end
		}
		if sp.StatusCode > a.maxStatus {
			a.maxStatus = sp.StatusCode
		}
		if betterRoot(a.root, sp, a.hasRoot) {
			a.root = sp
			a.hasRoot = true
		}
	}

	rows := make([]store.TraceListRow, 0, len(order))
	for _, id := range order {
		a := by[id]
		var dur uint64
		if !a.minTs.IsZero() && a.maxEnd.After(a.minTs) {
			dur = uint64(a.maxEnd.Sub(a.minTs))
		}
		rows = append(rows, store.TraceListRow{
			TraceID:    id,
			Service:    a.root.ServiceName,
			Operation:  a.root.OperationName,
			DurationNs: dur,
			SpanCount:  a.n,
			Timestamp:  a.minTs,
			StatusCode: a.maxStatus,
		})
	}
	return rows
}

// betterRoot matches search SQL:
// argMin(col, tuple(parent_span_id != ”, timestamp))
// empty parent wins; then earliest timestamp.
func betterRoot(cur, cand store.SpanRow, has bool) bool {
	if !has {
		return true
	}
	curChild := cur.ParentSpanID != ""
	candChild := cand.ParentSpanID != ""
	if curChild != candChild {
		return !candChild
	}
	if cand.Timestamp.Before(cur.Timestamp) {
		return true
	}
	if cand.Timestamp.After(cur.Timestamp) {
		return false
	}
	return false
}
