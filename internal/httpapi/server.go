// Package httpapi is the process HTTP surface.
//
// OTLP, structured logs, /api, and WebSocket mount on the same Server without
// changing cmd/rasat. Dependencies are injected; this package does not look up
// global state.
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/odurgut/rasat/internal/config"
)

const maxHeaderBytes = 1 << 20

// Handlers are the optional mounts. UI and Ready are required.
type Handlers struct {
	UI        http.Handler
	Ready     ReadyChecker
	Traces    http.Handler
	API       http.Handler
	Stream    http.Handler
	LogStream http.Handler
	Logs      http.Handler
}

// Server is the HTTP process: timeouts, middleware, and routes.
type Server struct {
	log   *slog.Logger
	httpd *http.Server
	ready ReadyChecker
}

// New builds a server. UI and Ready are required. Traces, API, Stream, and Logs may be nil.
func New(cfg config.Config, log *slog.Logger, h Handlers) (*Server, error) {
	if log == nil {
		return nil, errors.New("nil logger")
	}
	if h.UI == nil {
		return nil, errors.New("nil ui handler")
	}
	if h.Ready == nil {
		return nil, errors.New("nil ready checker")
	}

	s := &Server{log: log, ready: h.Ready}
	s.httpd = &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           s.routes(cfg, h),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelError),
	}
	return s, nil
}

func (s *Server) routes(cfg config.Config, h Handlers) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(recoverer(s.log))
	r.Use(maxBody(cfg.HTTPMaxBodyBytes))
	r.Use(accessLog(s.log))

	r.Get("/health", s.handleHealth)
	r.Get("/ready", s.handleReady)
	r.Get("/version", s.handleVersion)
	if h.Traces != nil {
		r.Method(http.MethodPost, "/v1/traces", h.Traces)
	}
	if h.Stream != nil {
		r.Handle("/api/stream/traces", h.Stream)
	}
	if h.LogStream != nil {
		r.Handle("/api/stream/logs", h.LogStream)
	}
	if h.Logs != nil {
		r.Method(http.MethodPost, "/api/logs", h.Logs)
	}
	if h.API != nil {
		r.Mount("/api", h.API)
	}
	r.Mount("/", h.UI)
	return r
}

// Serve accepts connections on ln until Shutdown or an error.
func (s *Server) Serve(ln net.Listener) error {
	return s.httpd.Serve(ln)
}

// Shutdown drains in-flight requests.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpd.Shutdown(ctx)
}

// Handler exposes the mux for tests.
func (s *Server) Handler() http.Handler {
	return s.httpd.Handler
}

// HTTPTimeouts reports the configured server timeouts (tests).
func (s *Server) HTTPTimeouts() (readHeader, read, write, idle bool) {
	return s.httpd.ReadHeaderTimeout > 0,
		s.httpd.ReadTimeout > 0,
		s.httpd.WriteTimeout > 0,
		s.httpd.IdleTimeout > 0
}
