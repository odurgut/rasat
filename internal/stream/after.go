package stream

import (
	"context"
	"sort"

	"github.com/odurgut/rasat/internal/store"
)

// AfterWrite publishes summaries after a successful persist.
// Publish is non-blocking; a slow WebSocket does not delay ingest.
type AfterWrite struct {
	Writer store.TraceWriter
	Hub    *Hub[store.TraceListRow]
	Limit  *RateLimit
}

// WriteTraceBatch implements store.TraceWriter.
func (a AfterWrite) WriteTraceBatch(ctx context.Context, batch store.TraceBatch) error {
	if err := a.Writer.WriteTraceBatch(ctx, batch); err != nil {
		return err
	}
	if a.Hub != nil {
		rows := SummarizeBatch(batch)
		sortByTimestampAsc(rows)
		for _, row := range rows {
			if a.Limit != nil && !a.Limit.Allow() {
				continue
			}
			a.Hub.Publish(row)
		}
	}
	return nil
}

// AfterLogWrite publishes log rows after a successful persist.
type AfterLogWrite struct {
	Writer store.LogWriter
	Hub    *Hub[store.LogRow]
	Limit  *RateLimit
}

// WriteLogBatch implements store.LogWriter.
func (a AfterLogWrite) WriteLogBatch(ctx context.Context, rows []store.LogRow) error {
	if err := a.Writer.WriteLogBatch(ctx, rows); err != nil {
		return err
	}
	if a.Hub == nil || len(rows) == 0 {
		return nil
	}
	out := append([]store.LogRow(nil), rows...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	for _, row := range out {
		if a.Limit != nil && !a.Limit.Allow() {
			continue
		}
		a.Hub.Publish(row)
	}
	return nil
}
