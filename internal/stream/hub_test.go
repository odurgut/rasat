package stream

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/odurgut/rasat/internal/store"
)

func testHub(t *testing.T, cfg HubConfig) *Hub[store.TraceListRow] {
	t.Helper()
	h, err := NewHub[store.TraceListRow](slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	return h
}

func TestNewHubRejectsNil(t *testing.T) {
	t.Parallel()
	if _, err := NewHub[store.TraceListRow](nil, HubConfig{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestPublishDoesNotBlock(t *testing.T) {
	t.Parallel()
	h := testHub(t, HubConfig{Buffer: 1, MaxClients: 1})
	sub, err := h.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	h.Publish(store.TraceListRow{TraceID: "a"})

	done := make(chan struct{})
	go func() {
		h.Publish(store.TraceListRow{TraceID: "b"})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Publish blocked")
	}

	select {
	case _, ok := <-sub.C():
		if !ok {
			return
		}
	case <-time.After(time.Second):
		t.Fatal("expected drop close or first event")
	}
	select {
	case _, ok := <-sub.C():
		if ok {
			t.Fatal("slow client should be dropped")
		}
	case <-time.After(time.Second):
		t.Fatal("channel not closed")
	}
}

func TestSlowClientDropLeavesFastClient(t *testing.T) {
	t.Parallel()
	h := testHub(t, HubConfig{Buffer: 1, MaxClients: 8})
	slow, err := h.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	fast, err := h.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer fast.Close()

	h.Publish(store.TraceListRow{TraceID: "1"})
	select {
	case row := <-fast.C():
		if row.TraceID != "1" {
			t.Fatalf("first %s", row.TraceID)
		}
	case <-time.After(time.Second):
		t.Fatal("fast missed first")
	}

	h.Publish(store.TraceListRow{TraceID: "2"})
	select {
	case row, ok := <-fast.C():
		if !ok || row.TraceID != "2" {
			t.Fatalf("second %v %v", ok, row)
		}
	case <-time.After(time.Second):
		t.Fatal("fast missed second")
	}

	select {
	case _, ok := <-slow.C():
		if ok {
			_, ok = <-slow.C()
			if ok {
				t.Fatal("slow client still subscribed")
			}
		}
	case <-time.After(time.Second):
		t.Fatal("slow not dropped")
	}
}

func TestSubscribeTooMany(t *testing.T) {
	t.Parallel()
	h := testHub(t, HubConfig{Buffer: 1, MaxClients: 1})
	sub, err := h.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	if _, err := h.Subscribe(); !errors.Is(err, ErrTooManyClients) {
		t.Fatalf("err %v", err)
	}
}

func TestSubscribeAfterClose(t *testing.T) {
	t.Parallel()
	h := testHub(t, HubConfig{})
	h.Close()
	if _, err := h.Subscribe(); !errors.Is(err, ErrClosed) {
		t.Fatalf("err %v", err)
	}
}

func TestPublishConcurrent(t *testing.T) {
	t.Parallel()
	h := testHub(t, HubConfig{Buffer: 64, MaxClients: 32})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		sub, err := h.Subscribe()
		if err != nil {
			t.Fatal(err)
		}
		wg.Add(1)
		go func(s *Sub[store.TraceListRow]) {
			defer wg.Done()
			defer s.Close()
			for range s.C() {
			}
		}(sub)
	}
	for i := 0; i < 32; i++ {
		h.Publish(store.TraceListRow{TraceID: "x"})
	}
	h.Close()
	wg.Wait()
}

func TestAfterWritePublishesOnSuccess(t *testing.T) {
	t.Parallel()
	h := testHub(t, HubConfig{Buffer: 4, MaxClients: 4})
	sub, err := h.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	w := AfterWrite{Writer: okWriter{}, Hub: h}
	err = w.WriteTraceBatch(context.Background(), store.TraceBatch{
		Spans: []store.SpanRow{{
			Timestamp:     time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC),
			TraceID:       "aa",
			ServiceName:   "checkout",
			OperationName: "GET /pay",
			DurationNs:    1000,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case row := <-sub.C():
		if row.TraceID != "aa" || row.Service != "checkout" {
			t.Fatalf("%+v", row)
		}
	case <-time.After(time.Second):
		t.Fatal("no event")
	}
}

func TestAfterWriteRateLimitDrops(t *testing.T) {
	t.Parallel()
	h := testHub(t, HubConfig{Buffer: 8, MaxClients: 4})
	sub, err := h.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	w := AfterWrite{Writer: okWriter{}, Hub: h, Limit: &RateLimit{n: 1, now: func() time.Time { return now }}}
	ts := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	err = w.WriteTraceBatch(context.Background(), store.TraceBatch{
		Spans: []store.SpanRow{
			{Timestamp: ts, TraceID: "aa", ServiceName: "a", OperationName: "x", DurationNs: 1},
			{Timestamp: ts, TraceID: "bb", ServiceName: "b", OperationName: "y", DurationNs: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case row := <-sub.C():
		if row.TraceID != "aa" {
			t.Fatalf("%+v", row)
		}
	case <-time.After(time.Second):
		t.Fatal("no event")
	}
	select {
	case row := <-sub.C():
		t.Fatalf("extra %+v", row)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestAfterWriteSkipsPublishOnError(t *testing.T) {
	t.Parallel()
	h := testHub(t, HubConfig{Buffer: 4, MaxClients: 4})
	sub, err := h.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	want := errors.New("write")
	w := AfterWrite{Writer: errWriter{err: want}, Hub: h}
	if err := w.WriteTraceBatch(context.Background(), store.TraceBatch{
		Spans: []store.SpanRow{{TraceID: "aa"}},
	}); !errors.Is(err, want) {
		t.Fatalf("err %v", err)
	}
	select {
	case row := <-sub.C():
		t.Fatalf("published %+v", row)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestAfterLogWritePublishesOnSuccess(t *testing.T) {
	t.Parallel()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := NewHub[store.LogRow](log, HubConfig{Buffer: 4, MaxClients: 4})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	sub, err := h.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	w := AfterLogWrite{Writer: okLogWriter{}, Hub: h}
	ts := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	err = w.WriteLogBatch(context.Background(), []store.LogRow{{
		Timestamp:   ts,
		ServiceName: "checkout",
		Level:       "ERROR",
		Message:     "database timeout",
		TraceID:     "abc123",
	}})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case row := <-sub.C():
		if row.TraceID != "abc123" || row.Level != "ERROR" {
			t.Fatalf("%+v", row)
		}
	case <-time.After(time.Second):
		t.Fatal("no event")
	}
}

type okLogWriter struct{}

func (okLogWriter) WriteLogBatch(context.Context, []store.LogRow) error { return nil }

type okWriter struct{}

func (okWriter) WriteTraceBatch(context.Context, store.TraceBatch) error { return nil }

type errWriter struct{ err error }

func (e errWriter) WriteTraceBatch(context.Context, store.TraceBatch) error { return e.err }
