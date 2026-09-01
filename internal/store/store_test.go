package store

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/odurgut/rasat/internal/config"
)

type fakeConn struct {
	ping          func(context.Context) error
	execErr       error
	execs         []string
	selectErr     error
	selects       []string
	selectArgs    [][]any
	selectFn      func(dest any, query string, args []any) error
	traceRows     []TraceListRow
	logRows       []LogRow
	serviceRows   []ServiceRow
	operationRows []OperationRow
	mapNodes      []ServiceMapNode
	mapEdges      []ServiceMapEdge
	metricRows    []ServiceMetrics
	metricBuckets []metricBucketRow
	causeRows     []ErrorCause
}

func (f *fakeConn) Ping(ctx context.Context) error {
	if f.ping != nil {
		return f.ping(ctx)
	}
	return nil
}

func (f *fakeConn) Exec(_ context.Context, query string, _ ...any) error {
	if f.execErr != nil {
		return f.execErr
	}
	f.execs = append(f.execs, query)
	return nil
}

func (f *fakeConn) Select(_ context.Context, dest any, query string, args ...any) error {
	if f.selectErr != nil {
		return f.selectErr
	}
	f.selects = append(f.selects, query)
	f.selectArgs = append(f.selectArgs, args)
	if f.selectFn != nil {
		return f.selectFn(dest, query, args)
	}
	if rows, ok := dest.(*[]TraceListRow); ok {
		*rows = f.traceRows
	}
	if rows, ok := dest.(*[]LogRow); ok {
		*rows = f.logRows
	}
	if rows, ok := dest.(*[]ServiceRow); ok {
		*rows = f.serviceRows
	}
	if rows, ok := dest.(*[]OperationRow); ok {
		*rows = f.operationRows
	}
	if rows, ok := dest.(*[]ServiceMapNode); ok {
		*rows = f.mapNodes
	}
	if rows, ok := dest.(*[]ServiceMapEdge); ok {
		*rows = f.mapEdges
	}
	if rows, ok := dest.(*[]ServiceMetrics); ok {
		*rows = f.metricRows
	}
	if rows, ok := dest.(*[]metricBucketRow); ok {
		*rows = f.metricBuckets
	}
	if rows, ok := dest.(*[]ErrorCause); ok {
		*rows = f.causeRows
	}
	return nil
}

func (f *fakeConn) Close() error { return nil }

func TestReadyOK(t *testing.T) {
	t.Parallel()
	s := &Store{conn: &fakeConn{}, pingTimeout: time.Second}
	if err := s.Ready(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestReadyWrapsPing(t *testing.T) {
	t.Parallel()
	s := &Store{
		conn: &fakeConn{ping: func(context.Context) error {
			return io.EOF
		}},
		pingTimeout: time.Second,
	}
	err := s.Ready(context.Background())
	if !errors.Is(err, ErrNotReady) {
		t.Fatalf("want ErrNotReady, got %v", err)
	}
	if !errors.Is(err, io.EOF) {
		t.Fatalf("want wrapped EOF, got %v", err)
	}
}

func TestReadyHonorsTimeout(t *testing.T) {
	t.Parallel()
	s := &Store{
		conn: &fakeConn{ping: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}},
		pingTimeout: 5 * time.Millisecond,
	}
	err := s.Ready(context.Background())
	if !errors.Is(err, ErrNotReady) {
		t.Fatalf("want ErrNotReady, got %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want deadline, got %v", err)
	}
}

func TestNewRejectsNilLogger(t *testing.T) {
	t.Parallel()
	cfg, err := config.LoadFrom(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(cfg, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewRejectsEmptyAddr(t *testing.T) {
	t.Parallel()
	cfg, err := config.LoadFrom(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	cfg.ClickHouseAddr = ""
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if _, err := New(cfg, log); err == nil {
		t.Fatal("expected error")
	}
}

func TestMigrateExecsStatements(t *testing.T) {
	t.Parallel()
	fc := &fakeConn{}
	s := &Store{
		conn:           fc,
		log:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		database:       "rasat",
		migrateTimeout: time.Second,
	}
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	want, err := Statements("rasat")
	if err != nil {
		t.Fatal(err)
	}
	if len(fc.execs) != len(want) {
		t.Fatalf("exec count %d want %d", len(fc.execs), len(want))
	}
}

func TestMigrateStopsOnExecError(t *testing.T) {
	t.Parallel()
	s := &Store{
		conn:           &fakeConn{execErr: io.ErrUnexpectedEOF},
		database:       "rasat",
		migrateTimeout: time.Second,
	}
	if err := s.Migrate(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}
