package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/odurgut/rasat/internal/config"
	"github.com/odurgut/rasat/internal/httpapi"
	"github.com/odurgut/rasat/internal/ingest"
	"github.com/odurgut/rasat/internal/store"
	"github.com/odurgut/rasat/internal/stream"
)

// Fn is one unit Init or Close step.
type Fn func(ctx context.Context, app *App) error

// Unit is one named stage of process startup. InitFn runs in slice order;
// CloseFn is collected and run in reverse on failure and on shutdown.
type Unit struct {
	Name    string
	InitFn  Fn
	CloseFn Fn
}

type namedClose struct {
	name string
	fn   Fn
}

// App is the in-process bag of collaborators. Units write here.
type App struct {
	Opt          Options
	Cfg          config.Config
	Log          *slog.Logger
	Store        *store.Store
	Ready        httpapi.ReadyChecker
	Traces       http.Handler
	API          http.Handler
	Logs         http.Handler
	GRPC         *ingest.GRPC
	GRPCListener net.Listener
	Stream       *stream.Hub[store.TraceListRow]
	LogStream    *stream.Hub[store.LogRow]
	HTTP         *httpapi.Server
	Listener     net.Listener

	closeFns     []namedClose
	serveErr     chan error
	serveCount   int
	serveTaken   bool
	stopCh       chan struct{}
	stopOnce     sync.Once
	closeOnce    sync.Once
	waitOnce     sync.Once
	waitServeErr error
}

// InitApp runs units in order. On failure it closes already-started units
// in reverse (CloseFn is registered only after a successful InitFn).
func InitApp(ctx context.Context, units []Unit, opt Options) (*App, error) {
	if opt.Getenv == nil {
		opt.Getenv = getenvOr(opt)
	}
	if opt.Stdout == nil {
		opt.Stdout = stdoutOr(opt)
	}

	a := &App{
		Opt:      opt,
		serveErr: make(chan error, 4),
		stopCh:   make(chan struct{}),
	}

	for _, u := range units {
		if u.InitFn == nil {
			closeWithTimeout(a)
			return nil, fmt.Errorf("unit %s: nil InitFn", u.Name)
		}
		start := time.Now()
		err := u.InitFn(ctx, a)
		elapsed := time.Since(start)
		if err != nil {
			if a.Log != nil {
				a.Log.Error("unit init failed", "unit", u.Name, "err", err, "elapsed_ms", elapsed.Milliseconds())
			}
			closeWithTimeout(a)
			return nil, fmt.Errorf("%s: %w", u.Name, err)
		}
		if a.Log != nil {
			a.Log.Info("init", "unit", u.Name, "elapsed_ms", elapsed.Milliseconds())
		}
		if u.CloseFn != nil {
			a.closeFns = append(a.closeFns, namedClose{name: u.Name, fn: u.CloseFn})
		}
	}
	return a, nil
}

// Close runs collected CloseFns in reverse. Safe to call more than once.
func (a *App) Close(ctx context.Context) {
	if a == nil {
		return
	}
	a.closeOnce.Do(func() {
		for i := len(a.closeFns) - 1; i >= 0; i-- {
			c := a.closeFns[i]
			if a.Log != nil {
				a.Log.Info("stop", "unit", c.name)
			}
			if err := c.fn(ctx, a); err != nil && a.Log != nil {
				a.Log.Error("unit close", "unit", c.name, "err", err)
			}
		}
		a.closeFns = nil
	})
}

func closeWithTimeout(a *App) {
	d := fallbackShutdown
	if a != nil && a.Cfg.ShutdownTimeout > 0 {
		d = a.Cfg.ShutdownTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	a.Close(ctx)
}
