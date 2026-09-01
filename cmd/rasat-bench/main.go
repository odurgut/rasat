// Command rasat-bench drives a spans/s firehose at a running Rasat and times search/detail.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/odurgut/rasat/internal/bench"
	"github.com/odurgut/rasat/internal/seed"
)

func main() {
	base := flag.String("url", getenv("RASAT_BENCH_URL", "http://127.0.0.1:8080"), "Rasat HTTP base (OTLP at /v1/traces)")
	spans := flag.Int("spans", 10000, "target spans per second")
	duration := flag.Duration("duration", 20*time.Second, "how long to load")
	workers := flag.Int("workers", 4, "concurrent OTLP exporters")
	batch := flag.Int("batch", 256, "spans packed per OTLP export")
	probeEvery := flag.Duration("probe-every", time.Second, "search/detail probe interval during load")
	timeout := flag.Duration("timeout", 15*time.Second, "HTTP timeout per call")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if *spans < 1 || *duration <= 0 || *workers < 1 || *batch < 1 || *probeEvery <= 0 {
		log.Error("invalid flags")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client := &http.Client{Timeout: *timeout}
	otlp := strings.TrimRight(*base, "/") + "/v1/traces"
	exp := seed.HTTPExporter{URL: otlp, Client: client}
	probes := bench.Client{Base: *base, Client: client}

	var mu sync.Mutex
	var report bench.Report
	probeCtx, probeStop := context.WithCancel(ctx)
	defer probeStop()
	go func() {
		tick := time.NewTicker(*probeEvery)
		defer tick.Stop()
		window := 2 * time.Minute
		for {
			select {
			case <-probeCtx.Done():
				return
			case <-tick.C:
				end := time.Now().UTC()
				start := end.Add(-window)
				s, ids := probes.Search(probeCtx, start, end, 20)
				mu.Lock()
				report.Search = append(report.Search, s)
				mu.Unlock()
				if len(ids) == 0 {
					continue
				}
				d := probes.Detail(probeCtx, ids[0], start, end)
				mu.Lock()
				report.Detail = append(report.Detail, d)
				mu.Unlock()
			}
		}
	}()

	log.Info("load start", "url", otlp, "spans_per_sec", *spans, "duration", duration.String())
	st, err := seed.RunLoad(ctx, exp, seed.LoadOptions{
		SpansPerSec: *spans,
		Duration:    *duration,
		Workers:     *workers,
		BatchSpans:  *batch,
		Log:         log,
	})
	probeStop()
	mu.Lock()
	out := report
	mu.Unlock()
	if err != nil {
		log.Error("load", "err", err)
		os.Exit(1)
	}

	fmt.Printf("load  spans/s=%.0f  target=%d  spans=%d  traces=%d  exports=%d  errors=%d  elapsed=%s\n",
		st.SpansPerSec(), *spans, st.Spans, st.Traces, st.Exports, st.Errors, st.Elapsed.Round(time.Millisecond))
	printSide("search", out.Search, bench.SearchBudget())
	printSide("detail", out.Detail, bench.DetailBudget())
}

func printSide(name string, samples []bench.Sample, budget time.Duration) {
	ok := len(samples) - bench.FailCount(samples)
	p50 := bench.Percentile(samples, 50)
	p99 := bench.Percentile(samples, 99)
	mark := "ok"
	if ok == 0 || p99 > budget {
		mark = "over"
	}
	fmt.Printf("%s  n=%d ok=%d  p50=%s  p99=%s  budget=%s  %s\n",
		name, len(samples), ok, p50.Round(time.Millisecond), p99.Round(time.Millisecond), budget, mark)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
