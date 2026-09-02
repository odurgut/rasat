package seed

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/ptrace"
)

var errBadLoadRate = errors.New("spans per second must be >= 1")

const (
	defaultLoadWorkers = 4
	defaultLoadBatch   = 256
)

// LoadOptions drive a spans/s firehose. Duration 0 runs until ctx is cancelled.
type LoadOptions struct {
	SpansPerSec int
	Duration    time.Duration
	Workers     int
	BatchSpans  int
	Now         func() time.Time
	Rand        *rand.Rand
	Log         *slog.Logger
}

// LoadStats is the result of one RunLoad.
type LoadStats struct {
	Exports int
	Traces  int
	Spans   int
	Errors  int
	Elapsed time.Duration
}

// SpansPerSec is achieved ingest rate over Elapsed.
func (s LoadStats) SpansPerSec() float64 {
	if s.Elapsed <= 0 {
		return 0
	}
	return float64(s.Spans) / s.Elapsed.Seconds()
}

// PackTraces appends shop scenarios until the payload has at least minSpans.
func PackTraces(rng *rand.Rand, t0 time.Time, minSpans int) ptrace.Traces {
	if rng == nil {
		rng = rand.New(rand.NewSource(t0.UnixNano()))
	}
	if t0.IsZero() {
		t0 = time.Now().UTC()
	}
	if minSpans < 1 {
		minSpans = 1
	}
	out := ptrace.NewTraces()
	for out.SpanCount() < minSpans {
		td := NextTrace(rng, t0)
		td.ResourceSpans().MoveAndAppendTo(out.ResourceSpans())
	}
	return out
}

// RunLoad posts packed OTLP batches to hit SpansPerSec. A failed export is
// counted and the loop continues; cancel is not an error.
func RunLoad(ctx context.Context, exp Exporter, opt LoadOptions) (LoadStats, error) {
	if exp == nil {
		return LoadStats{}, errNilExporter
	}
	if opt.SpansPerSec < 1 {
		return LoadStats{}, errBadLoadRate
	}
	workers := opt.Workers
	if workers < 1 {
		workers = defaultLoadWorkers
	}
	batchSpans := opt.BatchSpans
	if batchSpans < 1 {
		batchSpans = defaultLoadBatch
	}
	rng := opt.Rand
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	now := opt.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	log := opt.Log
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	jobs := make(chan ptrace.Traces, workers*2)
	var mu sync.Mutex
	var st LoadStats
	start := time.Now()

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for td := range jobs {
				err := exp.Export(ctx, td)
				mu.Lock()
				if err != nil {
					if ctx.Err() == nil {
						st.Errors++
						log.Error("load export", "err", err)
					}
				} else {
					st.Exports++
					st.Traces += countTraces(td)
					st.Spans += td.SpanCount()
				}
				mu.Unlock()
			}
		}()
	}

	committed := 0
producer:
	for ctx.Err() == nil {
		if opt.Duration > 0 && time.Since(start) >= opt.Duration {
			break
		}
		if err := waitBudget(ctx, start, committed, opt.SpansPerSec); err != nil {
			break
		}
		td := PackTraces(rng, now(), batchSpans)
		committed += td.SpanCount()
		select {
		case jobs <- td:
		case <-ctx.Done():
			break producer
		}
	}
	close(jobs)
	wg.Wait()
	st.Elapsed = time.Since(start)
	log.Info("load stopped",
		"exports", st.Exports,
		"traces", st.Traces,
		"spans", st.Spans,
		"errors", st.Errors,
		"spans_per_sec", st.SpansPerSec(),
	)
	return st, nil
}

func waitBudget(ctx context.Context, start time.Time, spans, spansPerSec int) error {
	if spansPerSec < 1 {
		return errBadLoadRate
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		elapsed := time.Since(start).Seconds()
		if elapsed < 0.02 {
			return nil
		}
		want := float64(spansPerSec) * elapsed
		if float64(spans) <= want {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Millisecond):
		}
	}
}
