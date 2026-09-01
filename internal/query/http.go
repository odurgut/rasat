package query

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/odurgut/rasat/internal/store"
)

// Searcher lists traces, logs, services, operations, the service map, derived metrics, and error causes. The ClickHouse implementation lives in store; tests inject stubs.
type Searcher interface {
	SearchTraces(ctx context.Context, q store.TraceSearch) ([]store.TraceListRow, error)
	GetTrace(ctx context.Context, q store.TraceGet) (*store.TraceDetail, error)
	SearchLogs(ctx context.Context, q store.LogSearch) ([]store.LogRow, error)
	ListServices(ctx context.Context, q store.ServiceList) ([]store.ServiceRow, error)
	ListOperations(ctx context.Context, q store.OperationList) ([]store.OperationRow, error)
	ServiceMap(ctx context.Context, q store.ServiceMapQuery) (*store.ServiceMapGraph, error)
	ListMetrics(ctx context.Context, q store.MetricsQuery) ([]store.ServiceMetrics, error)
	ListMetricsSeries(ctx context.Context, q store.MetricsQuery) ([]store.ServiceSeries, error)
	ListErrorCauses(ctx context.Context, q store.ErrorCausesQuery) ([]store.ErrorCause, error)
}

// Empty is a Searcher that returns no rows. Used when tests inject /ready without a Store.
type Empty struct{}

// SearchTraces implements Searcher.
func (Empty) SearchTraces(context.Context, store.TraceSearch) ([]store.TraceListRow, error) {
	return []store.TraceListRow{}, nil
}

// GetTrace implements Searcher.
func (Empty) GetTrace(context.Context, store.TraceGet) (*store.TraceDetail, error) {
	return nil, store.ErrNotFound
}

// SearchLogs implements Searcher.
func (Empty) SearchLogs(context.Context, store.LogSearch) ([]store.LogRow, error) {
	return []store.LogRow{}, nil
}

// ListServices implements Searcher.
func (Empty) ListServices(context.Context, store.ServiceList) ([]store.ServiceRow, error) {
	return []store.ServiceRow{}, nil
}

// ListOperations implements Searcher.
func (Empty) ListOperations(context.Context, store.OperationList) ([]store.OperationRow, error) {
	return []store.OperationRow{}, nil
}

// ServiceMap implements Searcher.
func (Empty) ServiceMap(context.Context, store.ServiceMapQuery) (*store.ServiceMapGraph, error) {
	return &store.ServiceMapGraph{Nodes: []store.ServiceMapNode{}, Edges: []store.ServiceMapEdge{}}, nil
}

// ListMetrics implements Searcher.
func (Empty) ListMetrics(context.Context, store.MetricsQuery) ([]store.ServiceMetrics, error) {
	return []store.ServiceMetrics{}, nil
}

// ListMetricsSeries implements Searcher.
func (Empty) ListMetricsSeries(context.Context, store.MetricsQuery) ([]store.ServiceSeries, error) {
	return []store.ServiceSeries{}, nil
}

// ListErrorCauses implements Searcher.
func (Empty) ListErrorCauses(context.Context, store.ErrorCausesQuery) ([]store.ErrorCause, error) {
	return []store.ErrorCause{}, nil
}

// Limits bound search. Timeout applies per request; MaxWindow caps end-start.
type Limits struct {
	Timeout   time.Duration
	MaxWindow time.Duration
}

type handler struct {
	log    *slog.Logger
	search Searcher
	limits Limits
}

// NewHandler returns an HTTP handler mounted at /api
// (GET /traces, GET /traces/{id}, GET /logs, GET /services, GET /operations, GET /service-map, GET /metrics, GET /error-causes).
func NewHandler(log *slog.Logger, search Searcher, limits Limits) (http.Handler, error) {
	if log == nil {
		return nil, errors.New("nil logger")
	}
	if search == nil {
		return nil, errors.New("nil searcher")
	}
	if limits.Timeout <= 0 {
		limits.Timeout = 10 * time.Second
	}
	if limits.MaxWindow <= 0 {
		limits.MaxWindow = 168 * time.Hour
	}
	h := &handler{log: log, search: search, limits: limits}
	r := chi.NewRouter()
	r.Get("/traces", h.handleSearch)
	r.Get("/traces/{id}", h.handleGet)
	r.Get("/logs", h.handleLogs)
	r.Get("/services", h.handleServices)
	r.Get("/operations", h.handleOperations)
	r.Get("/service-map", h.handleServiceMap)
	r.Get("/metrics", h.handleMetrics)
	r.Get("/error-causes", h.handleErrorCauses)
	return r, nil
}

type searchResponse struct {
	Traces []store.TraceListRow `json:"traces"`
}

type logsResponse struct {
	Logs []store.LogRow `json:"logs"`
}

type servicesResponse struct {
	Services []store.ServiceRow `json:"services"`
}

type operationsResponse struct {
	Operations []store.OperationRow `json:"operations"`
}

type metricsResponse struct {
	WindowS float64                `json:"window_s"`
	StepS   float64                `json:"step_s"`
	Metrics []store.ServiceMetrics `json:"metrics"`
	Series  []store.ServiceSeries  `json:"series"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (h *handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	q, err := ParseSearch(r.URL.Query(), h.limits.MaxWindow)
	if err != nil {
		if werr := writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()}); werr != nil {
			h.log.Error("write search error", "err", werr)
		}
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.limits.Timeout)
	defer cancel()
	rows, err := h.search.SearchTraces(ctx, q)
	if err != nil {
		if errors.Is(err, store.ErrInvalidSearch) {
			if werr := writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()}); werr != nil {
				h.log.Error("write search error", "err", werr)
			}
			return
		}
		h.log.Error("search traces", "err", err)
		if werr := writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "storage unavailable"}); werr != nil {
			h.log.Error("write search error", "err", werr)
		}
		return
	}
	if rows == nil {
		rows = []store.TraceListRow{}
	}
	if err := writeJSON(w, http.StatusOK, searchResponse{Traces: rows}); err != nil {
		h.log.Error("write search", "err", err)
	}
}

func (h *handler) handleGet(w http.ResponseWriter, r *http.Request) {
	q, err := ParseGet(chi.URLParam(r, "id"), r.URL.Query(), h.limits.MaxWindow, time.Now().UTC())
	if err != nil {
		if werr := writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()}); werr != nil {
			h.log.Error("write get error", "err", werr)
		}
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.limits.Timeout)
	defer cancel()
	detail, err := h.search.GetTrace(ctx, q)
	if err != nil {
		h.writeGetError(w, err)
		return
	}
	if err := writeJSON(w, http.StatusOK, detail); err != nil {
		h.log.Error("write get", "err", err)
	}
}

func (h *handler) writeGetError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		if werr := writeJSON(w, http.StatusNotFound, errorResponse{Error: "trace not found"}); werr != nil {
			h.log.Error("write get error", "err", werr)
		}
	case errors.Is(err, store.ErrInvalidSearch), errors.Is(err, store.ErrTraceTooLarge):
		if werr := writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()}); werr != nil {
			h.log.Error("write get error", "err", werr)
		}
	default:
		h.log.Error("get trace", "err", err)
		if werr := writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "storage unavailable"}); werr != nil {
			h.log.Error("write get error", "err", werr)
		}
	}
}

func (h *handler) handleLogs(w http.ResponseWriter, r *http.Request) {
	q, err := ParseLogs(r.URL.Query(), h.limits.MaxWindow)
	if err != nil {
		if werr := writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()}); werr != nil {
			h.log.Error("write logs error", "err", werr)
		}
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.limits.Timeout)
	defer cancel()
	rows, err := h.search.SearchLogs(ctx, q)
	if err != nil {
		if errors.Is(err, store.ErrInvalidSearch) {
			if werr := writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()}); werr != nil {
				h.log.Error("write logs error", "err", werr)
			}
			return
		}
		h.log.Error("search logs", "err", err)
		if werr := writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "storage unavailable"}); werr != nil {
			h.log.Error("write logs error", "err", werr)
		}
		return
	}
	if rows == nil {
		rows = []store.LogRow{}
	}
	if err := writeJSON(w, http.StatusOK, logsResponse{Logs: rows}); err != nil {
		h.log.Error("write logs", "err", err)
	}
}

func (h *handler) handleServices(w http.ResponseWriter, r *http.Request) {
	q, err := ParseServices(r.URL.Query(), h.limits.MaxWindow, time.Now().UTC())
	if err != nil {
		if werr := writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()}); werr != nil {
			h.log.Error("write services error", "err", werr)
		}
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.limits.Timeout)
	defer cancel()
	rows, err := h.search.ListServices(ctx, q)
	if err != nil {
		if errors.Is(err, store.ErrInvalidSearch) {
			if werr := writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()}); werr != nil {
				h.log.Error("write services error", "err", werr)
			}
			return
		}
		h.log.Error("list services", "err", err)
		if werr := writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "storage unavailable"}); werr != nil {
			h.log.Error("write services error", "err", werr)
		}
		return
	}
	if rows == nil {
		rows = []store.ServiceRow{}
	}
	if err := writeJSON(w, http.StatusOK, servicesResponse{Services: rows}); err != nil {
		h.log.Error("write services", "err", err)
	}
}

func (h *handler) handleOperations(w http.ResponseWriter, r *http.Request) {
	q, err := ParseOperations(r.URL.Query(), h.limits.MaxWindow, time.Now().UTC())
	if err != nil {
		if werr := writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()}); werr != nil {
			h.log.Error("write operations error", "err", werr)
		}
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.limits.Timeout)
	defer cancel()
	rows, err := h.search.ListOperations(ctx, q)
	if err != nil {
		if errors.Is(err, store.ErrInvalidSearch) {
			if werr := writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()}); werr != nil {
				h.log.Error("write operations error", "err", werr)
			}
			return
		}
		h.log.Error("list operations", "err", err)
		if werr := writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "storage unavailable"}); werr != nil {
			h.log.Error("write operations error", "err", werr)
		}
		return
	}
	if rows == nil {
		rows = []store.OperationRow{}
	}
	if err := writeJSON(w, http.StatusOK, operationsResponse{Operations: rows}); err != nil {
		h.log.Error("write operations", "err", err)
	}
}

func (h *handler) handleServiceMap(w http.ResponseWriter, r *http.Request) {
	q, err := ParseServiceMap(r.URL.Query(), h.limits.MaxWindow, time.Now().UTC())
	if err != nil {
		if werr := writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()}); werr != nil {
			h.log.Error("write service map error", "err", werr)
		}
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.limits.Timeout)
	defer cancel()
	graph, err := h.search.ServiceMap(ctx, q)
	if err != nil {
		if errors.Is(err, store.ErrInvalidSearch) {
			if werr := writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()}); werr != nil {
				h.log.Error("write service map error", "err", werr)
			}
			return
		}
		h.log.Error("service map", "err", err)
		if werr := writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "storage unavailable"}); werr != nil {
			h.log.Error("write service map error", "err", werr)
		}
		return
	}
	if graph == nil {
		graph = &store.ServiceMapGraph{}
	}
	if graph.Nodes == nil {
		graph.Nodes = []store.ServiceMapNode{}
	}
	if graph.Edges == nil {
		graph.Edges = []store.ServiceMapEdge{}
	}
	if err := writeJSON(w, http.StatusOK, graph); err != nil {
		h.log.Error("write service map", "err", err)
	}
}

func (h *handler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	q, err := ParseMetrics(r.URL.Query(), h.limits.MaxWindow, time.Now().UTC())
	if err != nil {
		if werr := writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()}); werr != nil {
			h.log.Error("write metrics error", "err", werr)
		}
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.limits.Timeout)
	defer cancel()
	rows, err := h.search.ListMetrics(ctx, q)
	if err != nil {
		if errors.Is(err, store.ErrInvalidSearch) {
			if werr := writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()}); werr != nil {
				h.log.Error("write metrics error", "err", werr)
			}
			return
		}
		h.log.Error("list metrics", "err", err)
		if werr := writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "storage unavailable"}); werr != nil {
			h.log.Error("write metrics error", "err", werr)
		}
		return
	}
	if rows == nil {
		rows = []store.ServiceMetrics{}
	}
	series := []store.ServiceSeries{}
	if q.Step >= time.Second {
		series, err = h.search.ListMetricsSeries(ctx, q)
		if err != nil {
			if errors.Is(err, store.ErrInvalidSearch) {
				if werr := writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()}); werr != nil {
					h.log.Error("write metrics error", "err", werr)
				}
				return
			}
			h.log.Error("list metrics series", "err", err)
			if werr := writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "storage unavailable"}); werr != nil {
				h.log.Error("write metrics error", "err", werr)
			}
			return
		}
		if series == nil {
			series = []store.ServiceSeries{}
		}
	}
	stepS := 0.0
	if q.Step > 0 {
		stepS = q.Step.Seconds()
	}
	if err := writeJSON(w, http.StatusOK, metricsResponse{
		WindowS: q.End.Sub(q.Start).Seconds(),
		StepS:   stepS,
		Metrics: rows,
		Series:  series,
	}); err != nil {
		h.log.Error("write metrics", "err", err)
	}
}

type errorCausesResponse struct {
	Causes []store.ErrorCause `json:"causes"`
}

func (h *handler) handleErrorCauses(w http.ResponseWriter, r *http.Request) {
	q, err := ParseErrorCauses(r.URL.Query(), h.limits.MaxWindow, time.Now().UTC())
	if err != nil {
		if werr := writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()}); werr != nil {
			h.log.Error("write error causes error", "err", werr)
		}
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.limits.Timeout)
	defer cancel()
	rows, err := h.search.ListErrorCauses(ctx, q)
	if err != nil {
		if errors.Is(err, store.ErrInvalidSearch) {
			if werr := writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()}); werr != nil {
				h.log.Error("write error causes error", "err", werr)
			}
			return
		}
		h.log.Error("list error causes", "err", err)
		if werr := writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "storage unavailable"}); werr != nil {
			h.log.Error("write error causes error", "err", werr)
		}
		return
	}
	if rows == nil {
		rows = []store.ErrorCause{}
	}
	if err := writeJSON(w, http.StatusOK, errorCausesResponse{Causes: rows}); err != nil {
		h.log.Error("write error causes", "err", err)
	}
}
