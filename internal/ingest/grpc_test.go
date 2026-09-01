package ingest

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/odurgut/rasat/internal/store"
)

func TestNewGRPCRejectsNil(t *testing.T) {
	t.Parallel()
	if _, err := NewGRPC(nil, 0); err == nil {
		t.Fatal("expected error")
	}
}

func TestGRPCExport(t *testing.T) {
	t.Parallel()
	cap := &captureWriter{}
	client, stop := startGRPC(t, cap, 0)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := client.Export(ctx, ptraceotlp.NewExportRequestFromTraces(sampleTraces()))
	if err != nil {
		t.Fatal(err)
	}
	cap.mu.Lock()
	defer cap.mu.Unlock()
	if len(cap.batch.Spans) != 1 || cap.batch.Spans[0].ServiceName != "checkout" {
		t.Fatalf("batch %+v", cap.batch)
	}
	if cap.batch.Spans[0].DurationNs == 0 {
		t.Fatal("duration")
	}
	if len(cap.batch.Events) != 1 || len(cap.batch.Links) != 1 {
		t.Fatalf("children events=%d links=%d", len(cap.batch.Events), len(cap.batch.Links))
	}
}

func TestGRPCWriteUnavailable(t *testing.T) {
	t.Parallel()
	cap := &captureWriter{err: io.ErrUnexpectedEOF}
	client, stop := startGRPC(t, cap, 0)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := client.Export(ctx, ptraceotlp.NewExportRequestFromTraces(sampleTraces()))
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("code %v err %v", status.Code(err), err)
	}
}

func TestGRPCTooManySpans(t *testing.T) {
	t.Parallel()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := NewHandler(log, Discard{}, Options{InsertTimeout: time.Second, MaxSpans: 1})
	if err != nil {
		t.Fatal(err)
	}
	client, stop := startGRPCHandler(t, h)
	defer stop()

	td := sampleTraces()
	sp := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().AppendEmpty()
	sp.SetTraceID(pcommon.TraceID{2})
	sp.SetSpanID(pcommon.SpanID{2})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = client.Export(ctx, ptraceotlp.NewExportRequestFromTraces(td))
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("code %v err %v", status.Code(err), err)
	}
}

func TestGRPCEmptyExport(t *testing.T) {
	t.Parallel()
	cap := &captureWriter{}
	client, stop := startGRPC(t, cap, 0)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := client.Export(ctx, ptraceotlp.NewExportRequest()); err != nil {
		t.Fatal(err)
	}
}

func startGRPC(t *testing.T, w store.TraceWriter, maxSpans int) (ptraceotlp.GRPCClient, func()) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := NewHandler(log, w, Options{InsertTimeout: time.Second, MaxSpans: maxSpans})
	if err != nil {
		t.Fatal(err)
	}
	return startGRPCHandler(t, h)
}

func startGRPCHandler(t *testing.T, h *Handler) (ptraceotlp.GRPCClient, func()) {
	t.Helper()
	g, err := NewGRPC(h, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- g.Serve(ln) }()

	conn, err := grpc.NewClient(ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	client := ptraceotlp.NewGRPCClient(conn)
	return client, func() {
		_ = conn.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := g.Shutdown(ctx); err != nil {
			t.Errorf("shutdown: %v", err)
		}
		if err := <-errCh; err != nil {
			t.Errorf("serve: %v", err)
		}
	}
}
