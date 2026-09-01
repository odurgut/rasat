package seed

import (
	"context"
	"io"
	"log/slog"
	"math/rand"
	"testing"
	"time"
)

func TestPackTracesMeetsMinSpans(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(3))
	t0 := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	td := PackTraces(rng, t0, 40)
	if td.SpanCount() < 40 {
		t.Fatalf("spans %d", td.SpanCount())
	}
}

func TestRunLoadNilExporter(t *testing.T) {
	t.Parallel()
	if _, err := RunLoad(context.Background(), nil, LoadOptions{SpansPerSec: 10}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunLoadBadRate(t *testing.T) {
	t.Parallel()
	if _, err := RunLoad(context.Background(), &countExporter{stopAt: 1}, LoadOptions{SpansPerSec: 0}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunLoadStopsOnCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	exp := &countExporter{stopAt: 2, cancel: cancel}
	st, err := RunLoad(ctx, exp, LoadOptions{
		SpansPerSec: 100000,
		Duration:    200 * time.Millisecond,
		Workers:     1,
		BatchSpans:  8,
		Rand:        rand.New(rand.NewSource(2)),
		Now:         func() time.Time { return time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC) },
		Log:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if st.Exports < 1 && st.Errors < 1 {
		t.Fatalf("stats %+v", st)
	}
}

func TestWaitBudgetRespectsRate(t *testing.T) {
	t.Parallel()
	start := time.Now().Add(-time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := waitBudget(ctx, start, 10_000, 1)
	if err == nil {
		t.Fatal("expected wait to hit ctx")
	}
}
