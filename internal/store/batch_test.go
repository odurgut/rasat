package store

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

type recBatch struct {
	rows      [][]any
	appendErr error
	sendErr   error
	aborted   bool
	sent      bool
}

func (b *recBatch) Append(v ...any) error {
	if b.appendErr != nil {
		return b.appendErr
	}
	row := make([]any, len(v))
	copy(row, v)
	b.rows = append(b.rows, row)
	return nil
}

func (b *recBatch) Send() error {
	if b.sendErr != nil {
		return b.sendErr
	}
	b.sent = true
	return nil
}

func (b *recBatch) Abort() error {
	b.aborted = true
	return nil
}

type recFactory struct {
	queries []string
	next    []*recBatch
	err     error
}

func (f *recFactory) PrepareBatch(_ context.Context, query string) (rowBatch, error) {
	f.queries = append(f.queries, query)
	if f.err != nil {
		return nil, f.err
	}
	if len(f.next) == 0 {
		b := &recBatch{}
		f.next = append(f.next, b)
		return b, nil
	}
	b := f.next[0]
	f.next = f.next[1:]
	return b, nil
}

func TestWriteTraceBatchEmpty(t *testing.T) {
	t.Parallel()
	f := &recFactory{}
	s := &Store{
		conn:          &fakeConn{},
		batches:       f,
		database:      "rasat",
		insertTimeout: time.Second,
	}
	if err := s.WriteTraceBatch(context.Background(), TraceBatch{}); err != nil {
		t.Fatal(err)
	}
	if len(f.queries) != 0 {
		t.Fatalf("unexpected prepare: %v", f.queries)
	}
}

func TestWriteTraceBatchNilStore(t *testing.T) {
	t.Parallel()
	var s *Store
	if err := s.WriteTraceBatch(context.Background(), TraceBatch{Spans: []SpanRow{{}}}); !errors.Is(err, ErrNotReady) {
		t.Fatalf("got %v", err)
	}
}

func TestWriteTraceBatchRequiresFactory(t *testing.T) {
	t.Parallel()
	s := &Store{conn: &fakeConn{}, database: "rasat", insertTimeout: time.Second}
	err := s.WriteTraceBatch(context.Background(), TraceBatch{Spans: []SpanRow{{TraceID: "a"}}})
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("got %v", err)
	}
}

func TestWriteTraceBatchInserts(t *testing.T) {
	t.Parallel()
	spanBatch := &recBatch{}
	eventBatch := &recBatch{}
	linkBatch := &recBatch{}
	f := &recFactory{next: []*recBatch{spanBatch, eventBatch, linkBatch}}
	s := &Store{
		conn:          &fakeConn{},
		batches:       f,
		database:      "rasat",
		insertTimeout: time.Second,
	}
	ts := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	err := s.WriteTraceBatch(context.Background(), TraceBatch{
		Spans: []SpanRow{{
			Timestamp:          ts,
			TraceID:            "aa",
			SpanID:             "bb",
			ServiceName:        "checkout",
			OperationName:      "GET /pay",
			Kind:               2,
			DurationNs:         100,
			StatusCode:         1,
			ResourceAttributes: map[string]string{"service.name": "checkout"},
			SpanAttributes:     map[string]string{"http.method": "GET"},
		}},
		Events: []EventRow{{
			Timestamp: ts,
			TraceID:   "aa",
			SpanID:    "bb",
			EventTime: ts,
			EventName: "exception",
		}},
		Links: []LinkRow{{
			Timestamp:     ts,
			TraceID:       "aa",
			SpanID:        "bb",
			LinkedTraceID: "cc",
			LinkedSpanID:  "dd",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.queries) != 3 {
		t.Fatalf("queries %d", len(f.queries))
	}
	if !strings.Contains(f.queries[0], "rasat.spans") {
		t.Fatalf("spans sql: %s", f.queries[0])
	}
	if !strings.Contains(f.queries[1], "rasat.span_events") {
		t.Fatalf("events sql: %s", f.queries[1])
	}
	if !strings.Contains(f.queries[2], "rasat.span_links") {
		t.Fatalf("links sql: %s", f.queries[2])
	}
	if len(spanBatch.rows) != 1 || !spanBatch.sent || spanBatch.aborted {
		t.Fatalf("span batch: %+v", spanBatch)
	}
	if spanBatch.rows[0][4] != "checkout" {
		t.Fatalf("service: %v", spanBatch.rows[0][4])
	}
	if !eventBatch.sent || !linkBatch.sent {
		t.Fatal("child batches not sent")
	}
}

func TestWriteTraceBatchAppendErrorAborts(t *testing.T) {
	t.Parallel()
	b := &recBatch{appendErr: io.ErrUnexpectedEOF}
	f := &recFactory{next: []*recBatch{b}}
	s := &Store{
		conn:          &fakeConn{},
		batches:       f,
		database:      "rasat",
		insertTimeout: time.Second,
	}
	err := s.WriteTraceBatch(context.Background(), TraceBatch{
		Spans: []SpanRow{{TraceID: "aa", SpanID: "bb"}},
	})
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("got %v", err)
	}
	if b.sent || !b.aborted {
		t.Fatalf("sent=%v aborted=%v", b.sent, b.aborted)
	}
}

func TestWriteLogBatchEmpty(t *testing.T) {
	t.Parallel()
	f := &recFactory{}
	s := &Store{
		conn:          &fakeConn{},
		batches:       f,
		database:      "rasat",
		insertTimeout: time.Second,
	}
	if err := s.WriteLogBatch(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if len(f.queries) != 0 {
		t.Fatalf("unexpected prepare: %v", f.queries)
	}
}

func TestWriteLogBatchNilStore(t *testing.T) {
	t.Parallel()
	var s *Store
	if err := s.WriteLogBatch(context.Background(), []LogRow{{}}); !errors.Is(err, ErrNotReady) {
		t.Fatalf("got %v", err)
	}
}

func TestWriteLogBatchInserts(t *testing.T) {
	t.Parallel()
	b := &recBatch{}
	f := &recFactory{next: []*recBatch{b}}
	s := &Store{
		conn:          &fakeConn{},
		batches:       f,
		database:      "rasat",
		insertTimeout: time.Second,
	}
	ts := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	err := s.WriteLogBatch(context.Background(), []LogRow{{
		Timestamp:   ts,
		ServiceName: "checkout",
		Level:       "ERROR",
		Message:     "database timeout",
		TraceID:     "abc123",
		SpanID:      "def456",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.queries) != 1 || !strings.Contains(f.queries[0], "rasat.logs") {
		t.Fatalf("sql: %v", f.queries)
	}
	if len(b.rows) != 1 || !b.sent || b.aborted {
		t.Fatalf("batch: %+v", b)
	}
	if b.rows[0][1] != "checkout" || b.rows[0][2] != "ERROR" || b.rows[0][4] != "abc123" {
		t.Fatalf("row: %v", b.rows[0])
	}
}

func TestWriteTraceBatchBadDatabase(t *testing.T) {
	t.Parallel()
	s := &Store{
		conn:          &fakeConn{},
		batches:       &recFactory{},
		database:      "rasat-prod",
		insertTimeout: time.Second,
	}
	err := s.WriteTraceBatch(context.Background(), TraceBatch{
		Spans: []SpanRow{{TraceID: "aa"}},
	})
	if !errors.Is(err, ErrInvalidIdent) {
		t.Fatalf("got %v", err)
	}
}
