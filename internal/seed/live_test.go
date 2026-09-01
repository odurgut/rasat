package seed

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"

	"github.com/odurgut/rasat/internal/ingest"
)

func TestNextTraceIsNowNotHistorical(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 8, 26, 15, 0, 0, 0, time.UTC)
	td := NextTrace(rand.New(rand.NewSource(4)), t0)
	batch, err := ingest.FlattenTraces(td, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Spans) < 2 {
		t.Fatalf("spans %d", len(batch.Spans))
	}
	for _, s := range batch.Spans {
		if s.Timestamp.Before(t0) || s.Timestamp.After(t0.Add(5*time.Second)) {
			t.Fatalf("span time %s want near %s", s.Timestamp, t0)
		}
	}
}

func TestNextTraceVaries(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(9))
	t0 := time.Date(2026, 8, 26, 15, 0, 0, 0, time.UTC)
	ops := map[string]int{}
	for i := 0; i < 40; i++ {
		td := NextTrace(rng, t0)
		batch, err := ingest.FlattenTraces(td, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range batch.Spans {
			if s.ParentSpanID == "" {
				ops[s.OperationName]++
			}
		}
	}
	if len(ops) < 3 {
		t.Fatalf("expected mixed roots, got %v", ops)
	}
}

func TestRunLiveStopsOnCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	exp := &countExporter{stopAt: 3, cancel: cancel}
	err := RunLive(ctx, exp, LiveOptions{
		Interval: time.Millisecond,
		Rand:     rand.New(rand.NewSource(5)),
		Now:      func() time.Time { return time.Date(2026, 8, 26, 15, 0, 0, 0, time.UTC) },
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if exp.n < 3 {
		t.Fatalf("exports %d", exp.n)
	}
}

func TestRunLiveNilExporter(t *testing.T) {
	t.Parallel()
	if err := RunLive(context.Background(), nil, LiveOptions{}); !errors.Is(err, errNilExporter) {
		t.Fatalf("err %v", err)
	}
}

func TestRunLiveContinuesAfterExportError(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	exp := &flakyExporter{failUntil: 1, stopAt: 3, cancel: cancel}
	err := RunLive(ctx, exp, LiveOptions{
		Interval: time.Millisecond,
		Rand:     rand.New(rand.NewSource(6)),
		Now:      func() time.Time { return time.Date(2026, 8, 26, 15, 0, 0, 0, time.UTC) },
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if exp.ok < 3 {
		t.Fatalf("ok exports %d", exp.ok)
	}
}

func TestLiveDelayBurst(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(7))
	d := liveDelay(rng, time.Second, liveBurstEvery)
	if d < 40*time.Millisecond || d > 160*time.Millisecond {
		t.Fatalf("burst %s", d)
	}
}

func TestHTTPExporter(t *testing.T) {
	t.Parallel()
	var gotCT string
	var gotLen int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		gotLen = len(body)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	td := NextTrace(rand.New(rand.NewSource(8)), time.Date(2026, 8, 26, 15, 0, 0, 0, time.UTC))
	err := HTTPExporter{URL: srv.URL, Client: srv.Client()}.Export(context.Background(), td)
	if err != nil {
		t.Fatal(err)
	}
	if gotCT != otlpProtobuf || gotLen < 10 {
		t.Fatalf("ct %s len %d", gotCT, gotLen)
	}
	req := ptraceotlp.NewExportRequestFromTraces(td)
	want, err := req.MarshalProto()
	if err != nil {
		t.Fatal(err)
	}
	if gotLen != len(want) {
		t.Fatalf("body %d want %d", gotLen, len(want))
	}
}

func TestHTTPExporterRejects(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	td := NextTrace(rand.New(rand.NewSource(8)), time.Date(2026, 8, 26, 15, 0, 0, 0, time.UTC))
	err := HTTPExporter{URL: srv.URL, Client: srv.Client()}.Export(context.Background(), td)
	if err == nil {
		t.Fatal("expected error")
	}
}

type countExporter struct {
	mu     sync.Mutex
	n      int
	stopAt int
	cancel context.CancelFunc
}

func (c *countExporter) Export(context.Context, ptrace.Traces) error {
	c.mu.Lock()
	c.n++
	n := c.n
	c.mu.Unlock()
	if n >= c.stopAt {
		c.cancel()
	}
	return nil
}

type flakyExporter struct {
	mu        sync.Mutex
	n         int
	ok        int
	failUntil int
	stopAt    int
	cancel    context.CancelFunc
}

func (f *flakyExporter) Export(context.Context, ptrace.Traces) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	if f.n <= f.failUntil {
		return errors.New("unavailable")
	}
	f.ok++
	if f.ok >= f.stopAt {
		f.cancel()
	}
	return nil
}
