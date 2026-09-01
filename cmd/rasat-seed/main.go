// Command rasat-seed posts synthetic OTLP traces to a running Rasat process.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/odurgut/rasat/internal/seed"
)

func main() {
	url := flag.String("url", getenv("RASAT_SEED_URL", "http://127.0.0.1:8080/v1/traces"), "OTLP/HTTP traces URL")
	n := flag.Int("n", 80, "number of traces (ignored with -live)")
	timeout := flag.Duration("timeout", 15*time.Second, "HTTP timeout per export")
	live := flag.Bool("live", false, "post traces until interrupt")
	interval := flag.Duration("interval", 800*time.Millisecond, "mean delay between live traces")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	client := &http.Client{Timeout: *timeout}
	exp := seed.HTTPExporter{URL: *url, Client: client}
	lexp := seed.HTTPLogExporter{URL: logsURL(*url), Client: client}

	if *live {
		if *interval <= 0 {
			log.Error("interval must be > 0")
			os.Exit(1)
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		log.Info("live seed", "url", *url, "interval", interval.String())
		if err := seed.RunLive(ctx, exp, seed.LiveOptions{Interval: *interval, Log: log, Logs: lexp}); err != nil {
			log.Error("live", "err", err)
			os.Exit(1)
		}
		return
	}

	td := seed.Traces(seed.Options{
		Now:   time.Now().UTC(),
		Count: *n,
	})
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if err := exp.Export(ctx, td); err != nil {
		log.Error("seed", "err", err)
		os.Exit(1)
	}
	logs := seed.LogsFromTraces(td)
	if err := lexp.ExportLogs(ctx, logs); err != nil {
		log.Error("seed logs", "err", err)
		os.Exit(1)
	}
	log.Info("seeded", "traces", *n, "spans", td.SpanCount(), "logs", len(logs), "url", *url)
}

func logsURL(tracesURL string) string {
	u := strings.TrimSuffix(tracesURL, "/v1/traces")
	return strings.TrimRight(u, "/") + "/api/logs"
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
