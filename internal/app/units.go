package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/odurgut/rasat/internal/config"
	"github.com/odurgut/rasat/internal/httpapi"
	"github.com/odurgut/rasat/internal/ingest"
	"github.com/odurgut/rasat/internal/logging"
	"github.com/odurgut/rasat/internal/logs"
	"github.com/odurgut/rasat/internal/query"
	"github.com/odurgut/rasat/internal/store"
	"github.com/odurgut/rasat/internal/stream"
	"github.com/odurgut/rasat/internal/ui"
	"github.com/odurgut/rasat/internal/version"
)

// Named units for cmd/rasat. Same App, reverse CloseFn.
var (
	UnitConfig = Unit{Name: "config", InitFn: InitConfig}
	UnitLogger = Unit{Name: "logger", InitFn: InitLogger}
	UnitStore  = Unit{Name: "store", InitFn: InitStore, CloseFn: CloseStore}
	UnitStream = Unit{Name: "stream", InitFn: InitStream, CloseFn: CloseStream}
	UnitOTLP   = Unit{Name: "otlp", InitFn: InitOTLP, CloseFn: CloseOTLP}
	UnitQuery  = Unit{Name: "query", InitFn: InitQuery}
	UnitLogs   = Unit{Name: "logs", InitFn: InitLogs}
	UnitHTTP   = Unit{Name: "http", InitFn: InitHTTP, CloseFn: CloseHTTP}
)

// DefaultUnits is the production process order. Stream is before OTLP so
// ingest can publish after a successful write.
func DefaultUnits() []Unit {
	return []Unit{UnitConfig, UnitLogger, UnitStore, UnitStream, UnitOTLP, UnitQuery, UnitLogs, UnitHTTP}
}

// InitConfig loads RASAT_* into App.Cfg.
func InitConfig(_ context.Context, a *App) error {
	getenv := a.Opt.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	cfg, err := config.LoadFrom(getenv)
	if err != nil {
		return err
	}
	a.Cfg = cfg
	return nil
}

// InitLogger installs slog on App.Log.
func InitLogger(_ context.Context, a *App) error {
	out := a.Opt.Stdout
	if out == nil {
		out = os.Stdout
	}
	log, err := logging.New(out, a.Cfg.LogFormat, a.Cfg.LogLevel)
	if err != nil {
		return err
	}
	a.Log = log
	return nil
}

// InitStore opens ClickHouse, migrates, and sets Ready. Tests may inject Ready.
func InitStore(ctx context.Context, a *App) error {
	if a.Opt.Ready != nil {
		a.Ready = a.Opt.Ready
		return nil
	}
	st, err := store.New(a.Cfg, a.Log)
	if err != nil {
		return err
	}
	if err := st.Migrate(ctx); err != nil {
		_ = st.Close()
		return fmt.Errorf("migrate: %w", err)
	}
	a.Store = st
	a.Ready = st
	return nil
}

// CloseStore closes ClickHouse (or a test ReadyCloser).
func CloseStore(_ context.Context, a *App) error {
	var err error
	if a.Opt.ReadyCloser != nil {
		err = a.Opt.ReadyCloser.Close()
	} else if a.Store != nil {
		err = a.Store.Close()
	}
	a.Store = nil
	a.Ready = nil
	return err
}

// InitStream starts the in-process trace and log fan-out.
func InitStream(_ context.Context, a *App) error {
	cfg := stream.HubConfig{
		Buffer:       a.Cfg.StreamBuffer,
		MaxClients:   a.Cfg.StreamMaxClients,
		WriteTimeout: a.Cfg.StreamWriteTimeout,
	}
	th, err := stream.NewHub[store.TraceListRow](a.Log, cfg)
	if err != nil {
		return err
	}
	lh, err := stream.NewHub[store.LogRow](a.Log, cfg)
	if err != nil {
		th.Close()
		return err
	}
	a.Stream = th
	a.LogStream = lh
	return nil
}

// CloseStream drops every WebSocket subscriber.
func CloseStream(_ context.Context, a *App) error {
	if a.Stream != nil {
		a.Stream.Close()
		a.Stream = nil
	}
	if a.LogStream != nil {
		a.LogStream.Close()
		a.LogStream = nil
	}
	return nil
}

// InitOTLP mounts OTLP/HTTP + OTLP/gRPC ingest.
func InitOTLP(_ context.Context, a *App) error {
	w := a.Opt.TraceWriter
	if w == nil {
		if a.Store != nil {
			w = a.Store
		} else {
			w = ingest.Discard{}
		}
	}
	if a.Stream != nil {
		w = stream.AfterWrite{Writer: w, Hub: a.Stream, Limit: stream.NewRateLimit(a.Cfg.StreamMaxPerSec)}
	}
	h, err := ingest.NewHandler(a.Log, w, ingest.Options{
		InsertTimeout: a.Cfg.ClickHouseInsertTimeout,
		MaxDecoded:    a.Cfg.HTTPMaxBodyBytes,
	})
	if err != nil {
		return err
	}
	a.Traces = h

	g, err := ingest.NewGRPC(h, recvSize(a.Cfg.HTTPMaxBodyBytes))
	if err != nil {
		return err
	}
	ln := a.Opt.GRPCListener
	if ln == nil {
		ln, err = net.Listen("tcp", a.Cfg.GRPCAddr)
		if err != nil {
			g.Stop()
			return fmt.Errorf("listen grpc %s: %w", a.Cfg.GRPCAddr, err)
		}
	}
	a.GRPC = g
	a.GRPCListener = ln
	startServe(a, g.Serve, ln)
	return nil
}

// CloseOTLP stops the OTLP/gRPC server.
func CloseOTLP(ctx context.Context, a *App) error {
	if a.GRPC == nil {
		return nil
	}
	err := a.GRPC.Shutdown(ctx)
	a.GRPC = nil
	return err
}

// InitQuery mounts GET /api/traces, GET /api/traces/{id}, GET /api/logs, GET /api/services, GET /api/operations, GET /api/service-map, GET /api/metrics, and GET /api/error-causes.
func InitQuery(_ context.Context, a *App) error {
	s := a.Opt.TraceSearcher
	if s == nil {
		if a.Store != nil {
			s = a.Store
		} else {
			s = query.Empty{}
		}
	}
	h, err := query.NewHandler(a.Log, s, query.Limits{
		Timeout:   a.Cfg.ClickHouseQueryTimeout,
		MaxWindow: a.Cfg.QueryMaxWindow,
	})
	if err != nil {
		return err
	}
	a.API = h
	return nil
}

// InitLogs mounts POST /api/logs.
func InitLogs(_ context.Context, a *App) error {
	w := a.Opt.LogWriter
	if w == nil {
		if a.Store != nil {
			w = a.Store
		} else {
			w = logs.Discard{}
		}
	}
	if a.LogStream != nil {
		w = stream.AfterLogWrite{Writer: w, Hub: a.LogStream, Limit: stream.NewRateLimit(a.Cfg.StreamMaxPerSec)}
	}
	h, err := logs.NewHandler(a.Log, w, logs.Options{
		InsertTimeout: a.Cfg.ClickHouseInsertTimeout,
		MaxDecoded:    a.Cfg.HTTPMaxBodyBytes,
	})
	if err != nil {
		return err
	}
	a.Logs = h
	return nil
}

func recvSize(n int64) int {
	const capBytes = 64 << 20
	if n <= 0 {
		return 16 << 20
	}
	if n > capBytes {
		return capBytes
	}
	return int(n)
}

func startServe(a *App, serve func(net.Listener) error, ln net.Listener) {
	if a.serveErr == nil {
		a.serveErr = make(chan error, 4)
	}
	a.serveCount++
	go func() {
		a.serveErr <- serve(ln)
	}()
}

// InitHTTP starts the HTTP server (UI, health, OTLP/HTTP, logs, /api).
func InitHTTP(_ context.Context, a *App) error {
	uih, err := ui.New()
	if err != nil {
		return fmt.Errorf("ui: %w", err)
	}
	var streamH http.Handler
	if a.Stream != nil {
		streamH = a.Stream
	}
	var logStreamH http.Handler
	if a.LogStream != nil {
		logStreamH = a.LogStream
	}
	srv, err := httpapi.New(a.Cfg, a.Log, httpapi.Handlers{
		UI:        uih,
		Ready:     a.Ready,
		Traces:    a.Traces,
		API:       a.API,
		Stream:    streamH,
		LogStream: logStreamH,
		Logs:      a.Logs,
	})
	if err != nil {
		return err
	}
	ln := a.Opt.Listener
	if ln == nil {
		ln, err = net.Listen("tcp", a.Cfg.HTTPAddr)
		if err != nil {
			return fmt.Errorf("listen %s: %w", a.Cfg.HTTPAddr, err)
		}
	}
	a.HTTP = srv
	a.Listener = ln
	startServe(a, func(l net.Listener) error {
		err := srv.Serve(l)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}, ln)
	if a.Log != nil {
		attrs := []any{
			"version", version.Version,
			"commit", version.Commit,
			"addr", ln.Addr().String(),
		}
		if a.GRPCListener != nil {
			attrs = append(attrs, "grpc", a.GRPCListener.Addr().String())
		}
		a.Log.Info("rasat starting", attrs...)
	}
	return nil
}

// CloseHTTP drains the HTTP server.
func CloseHTTP(ctx context.Context, a *App) error {
	if a.HTTP == nil {
		return nil
	}
	err := a.HTTP.Shutdown(ctx)
	a.HTTP = nil
	return err
}
