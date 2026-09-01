package seed

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math/rand"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var errNilExporter = errors.New("nil exporter")

const (
	defaultLiveInterval = 800 * time.Millisecond
	liveJitter          = 0.55
	liveBurstEvery      = 8
)

// LiveOptions control the interruptible traffic loop.
type LiveOptions struct {
	Interval time.Duration
	Now      func() time.Time
	Rand     *rand.Rand
	Log      *slog.Logger
	Logs     LogExporter
}

// NextTrace builds one shop scenario (sometimes a linked pair) starting at t0.
// Times are "now", not the historical 18h spread used by Traces.
func NextTrace(rng *rand.Rand, t0 time.Time) ptrace.Traces {
	if rng == nil {
		rng = rand.New(rand.NewSource(t0.UnixNano()))
	}
	if t0.IsZero() {
		t0 = time.Now().UTC()
	}
	e := &emitter{td: ptrace.NewTraces(), rng: rng}
	pickLive(rng)(e, 8, t0)
	return e.td
}

// RunLive posts traces until ctx is cancelled (SIGINT/SIGTERM in the CLI).
// A failed export is logged and the loop continues; cancel is not an error.
func RunLive(ctx context.Context, exp Exporter, opt LiveOptions) error {
	if exp == nil {
		return errNilExporter
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
	interval := opt.Interval
	if interval <= 0 {
		interval = defaultLiveInterval
	}

	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()

	var exports, traces, spans int
	for {
		if err := ctx.Err(); err != nil {
			log.Info("live stopped", "exports", exports, "traces", traces, "spans", spans)
			return nil
		}
		td := NextTrace(rng, now())
		if err := exp.Export(ctx, td); err != nil {
			if ctx.Err() != nil {
				log.Info("live stopped", "exports", exports, "traces", traces, "spans", spans)
				return nil
			}
			log.Error("live export", "err", err)
		} else {
			exports++
			n := countTraces(td)
			traces += n
			spans += td.SpanCount()
			if opt.Logs != nil {
				if lerr := opt.Logs.ExportLogs(ctx, LogsFromTraces(td)); lerr != nil && ctx.Err() == nil {
					log.Error("live logs", "err", lerr)
				}
			}
			log.Info("live", "traces", n, "spans", td.SpanCount(), "total_traces", traces)
		}

		d := liveDelay(rng, interval, exports)
		timer.Reset(d)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			log.Info("live stopped", "exports", exports, "traces", traces, "spans", spans)
			return nil
		case <-timer.C:
		}
	}
}

func liveDelay(rng *rand.Rand, mean time.Duration, n int) time.Duration {
	if mean <= 0 {
		return 0
	}
	if n > 0 && n%liveBurstEvery == 0 {
		return 50*time.Millisecond + time.Duration(rng.Intn(100))*time.Millisecond
	}
	f := 1 - liveJitter + rng.Float64()*2*liveJitter
	d := time.Duration(float64(mean) * f)
	if d < 40*time.Millisecond {
		return 40 * time.Millisecond
	}
	return d
}

func pickLive(rng *rand.Rand) scenario {
	book := playbook()
	// Traffic mix: more browse/checkout than rare paths. Same scenarios as Traces.
	weights := []int{8, 5, 4, 2, 3, 1, 1, 1, 1, 1, 1}
	if len(weights) != len(book) {
		return book[rng.Intn(len(book))]
	}
	total := 0
	for _, w := range weights {
		total += w
	}
	n := rng.Intn(total)
	for i, w := range weights {
		if n < w {
			return book[i]
		}
		n -= w
	}
	return book[0]
}

func countTraces(td ptrace.Traces) int {
	ids := make(map[pcommon.TraceID]struct{})
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		sss := rss.At(i).ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			spans := sss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				ids[spans.At(k).TraceID()] = struct{}{}
			}
		}
	}
	return len(ids)
}
