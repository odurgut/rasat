package query

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/odurgut/rasat/internal/store"
)

type captureSearch struct {
	mu             sync.Mutex
	last           store.TraceSearch
	lastGet        store.TraceGet
	lastLogs       store.LogSearch
	lastServices   store.ServiceList
	lastOperations store.OperationList
	lastMap        store.ServiceMapQuery
	lastMetrics    store.MetricsQuery
	lastCauses     store.ErrorCausesQuery
	rows           []store.TraceListRow
	logs           []store.LogRow
	detail         *store.TraceDetail
	services       []store.ServiceRow
	operations     []store.OperationRow
	graph          *store.ServiceMapGraph
	metrics        []store.ServiceMetrics
	series         []store.ServiceSeries
	causes         []store.ErrorCause
	err            error
	getErr         error
	servicesErr    error
	operationsErr  error
	mapErr         error
	metricsErr     error
	seriesErr      error
	causesErr      error
}

func (c *captureSearch) SearchTraces(_ context.Context, q store.TraceSearch) ([]store.TraceListRow, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.last = q
	return c.rows, c.err
}

func (c *captureSearch) GetTrace(_ context.Context, q store.TraceGet) (*store.TraceDetail, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastGet = q
	if c.getErr != nil {
		return nil, c.getErr
	}
	if c.err != nil && c.detail == nil {
		return nil, c.err
	}
	return c.detail, nil
}

func (c *captureSearch) SearchLogs(_ context.Context, q store.LogSearch) ([]store.LogRow, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastLogs = q
	if c.err != nil && c.logs == nil {
		return nil, c.err
	}
	return c.logs, nil
}

func (c *captureSearch) ListServices(_ context.Context, q store.ServiceList) ([]store.ServiceRow, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastServices = q
	if c.servicesErr != nil {
		return nil, c.servicesErr
	}
	if c.err != nil && c.services == nil {
		return nil, c.err
	}
	return c.services, nil
}

func (c *captureSearch) ListOperations(_ context.Context, q store.OperationList) ([]store.OperationRow, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastOperations = q
	if c.operationsErr != nil {
		return nil, c.operationsErr
	}
	if c.err != nil && c.operations == nil {
		return nil, c.err
	}
	return c.operations, nil
}

func (c *captureSearch) ServiceMap(_ context.Context, q store.ServiceMapQuery) (*store.ServiceMapGraph, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastMap = q
	if c.mapErr != nil {
		return nil, c.mapErr
	}
	if c.err != nil && c.graph == nil {
		return nil, c.err
	}
	if c.graph == nil {
		return &store.ServiceMapGraph{Nodes: []store.ServiceMapNode{}, Edges: []store.ServiceMapEdge{}}, nil
	}
	return c.graph, nil
}

func (c *captureSearch) ListMetrics(_ context.Context, q store.MetricsQuery) ([]store.ServiceMetrics, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastMetrics = q
	if c.metricsErr != nil {
		return nil, c.metricsErr
	}
	if c.err != nil && c.metrics == nil {
		return nil, c.err
	}
	return c.metrics, nil
}

func (c *captureSearch) ListMetricsSeries(_ context.Context, q store.MetricsQuery) ([]store.ServiceSeries, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastMetrics = q
	if c.seriesErr != nil {
		return nil, c.seriesErr
	}
	if c.err != nil && c.series == nil {
		return nil, c.err
	}
	if c.series == nil {
		return []store.ServiceSeries{}, nil
	}
	return c.series, nil
}

func (c *captureSearch) ListErrorCauses(_ context.Context, q store.ErrorCausesQuery) ([]store.ErrorCause, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastCauses = q
	if c.causesErr != nil {
		return nil, c.causesErr
	}
	if c.err != nil && c.causes == nil {
		return nil, c.err
	}
	if c.causes == nil {
		return []store.ErrorCause{}, nil
	}
	return c.causes, nil
}

func testAPI(t *testing.T, s Searcher) http.Handler {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := NewHandler(log, s, Limits{Timeout: time.Second, MaxWindow: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestNewHandlerRejectsNil(t *testing.T) {
	t.Parallel()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if _, err := NewHandler(nil, Empty{}, Limits{}); err == nil {
		t.Fatal("nil logger")
	}
	if _, err := NewHandler(log, nil, Limits{}); err == nil {
		t.Fatal("nil searcher")
	}
}

func TestSearchErrorSpanByFilter(t *testing.T) {
	t.Parallel()
	cap := &captureSearch{rows: []store.TraceListRow{{
		TraceID:    "aa",
		Service:    "checkout",
		Operation:  "GET /pay",
		DurationNs: 12_000_000,
		SpanCount:  2,
		Timestamp:  time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC),
		StatusCode: 2,
	}}}
	h := testAPI(t, cap)

	req := httptest.NewRequest(http.MethodGet, "/traces?start=2026-08-26T00:00:00Z&end=2026-08-26T01:00:00Z&limit=20&service=checkout&status=error", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body searchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Traces) != 1 || body.Traces[0].StatusCode != 2 || body.Traces[0].Service != "checkout" {
		t.Fatalf("body %+v", body)
	}
	cap.mu.Lock()
	defer cap.mu.Unlock()
	if cap.last.Service != "checkout" || cap.last.StatusCode == nil || *cap.last.StatusCode != 2 {
		t.Fatalf("query %+v", cap.last)
	}
	if cap.last.Limit != 20 {
		t.Fatalf("limit %d", cap.last.Limit)
	}
}

func TestLogsSearch(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	cap := &captureSearch{logs: []store.LogRow{{
		Timestamp:   ts,
		ServiceName: "checkout",
		Level:       "ERROR",
		Message:     "database timeout",
		TraceID:     "abc123",
	}}}
	h := testAPI(t, cap)
	req := httptest.NewRequest(http.MethodGet, "/logs?start=2026-08-26T00:00:00Z&end=2026-08-26T01:00:00Z&limit=20&service=checkout&level=error&trace_id=abc123", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body logsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Logs) != 1 || body.Logs[0].Level != "ERROR" || body.Logs[0].ServiceName != "checkout" {
		t.Fatalf("body %+v", body)
	}
	cap.mu.Lock()
	defer cap.mu.Unlock()
	if cap.lastLogs.Service != "checkout" || cap.lastLogs.Level != "ERROR" || cap.lastLogs.TraceID != "abc123" {
		t.Fatalf("query %+v", cap.lastLogs)
	}
}

func TestLogsRequiresBounds(t *testing.T) {
	t.Parallel()
	h := testAPI(t, Empty{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/logs", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestSearchRequiresBounds(t *testing.T) {
	t.Parallel()
	h := testAPI(t, Empty{})
	tests := []string{
		"/traces",
		"/traces?start=2026-08-26T00:00:00Z&end=2026-08-26T01:00:00Z",
		"/traces?start=2026-08-26T00:00:00Z&end=2026-08-26T03:00:00Z&limit=10",
	}
	for _, path := range tests {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status %d", path, rec.Code)
		}
	}
}

func TestSearchStorageError(t *testing.T) {
	t.Parallel()
	h := testAPI(t, &captureSearch{err: io.ErrUnexpectedEOF})
	req := httptest.NewRequest(http.MethodGet, "/traces?start=2026-08-26T00:00:00Z&end=2026-08-26T01:00:00Z&limit=10", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestSearchInvalidFromStore(t *testing.T) {
	t.Parallel()
	h := testAPI(t, &captureSearch{err: store.ErrInvalidSearch})
	req := httptest.NewRequest(http.MethodGet, "/traces?start=2026-08-26T00:00:00Z&end=2026-08-26T01:00:00Z&limit=10", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestSearchEmptyList(t *testing.T) {
	t.Parallel()
	h := testAPI(t, Empty{})
	req := httptest.NewRequest(http.MethodGet, "/traces?start=2026-08-26T00:00:00Z&end=2026-08-26T01:00:00Z&limit=10", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body searchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Traces == nil || len(body.Traces) != 0 {
		t.Fatalf("traces %#v", body.Traces)
	}
}

func TestGetTraceDetail(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC)
	cap := &captureSearch{detail: &store.TraceDetail{
		TraceID:        "aa",
		Timestamp:      ts,
		DurationNs:     10_000_000,
		SpanCount:      2,
		CriticalPathNs: 10_000_000,
		CriticalPath: []store.CriticalPathStep{
			{SpanID: "01", Service: "gateway", Operation: "HTTP GET", DurationNs: 10_000_000},
			{SpanID: "02", Service: "checkout", Operation: "pay", DurationNs: 5_000_000},
		},
		Bottlenecks: []store.Bottleneck{
			{SpanID: "01", Service: "gateway", Operation: "HTTP GET", ExclusiveNs: 5_000_000},
			{SpanID: "02", Service: "checkout", Operation: "pay", ExclusiveNs: 5_000_000},
		},
		Spans: []store.SpanDetail{
			{
				Timestamp:          ts,
				SpanID:             "01",
				Service:            "gateway",
				Operation:          "HTTP GET",
				DurationNs:         10_000_000,
				Events:             []store.SpanEvent{},
				Links:              []store.SpanLink{},
				ResourceAttributes: map[string]string{},
				SpanAttributes:     map[string]string{},
			},
			{
				Timestamp:    ts.Add(time.Millisecond),
				SpanID:       "02",
				ParentSpanID: "01",
				Service:      "checkout",
				Operation:    "pay",
				DurationNs:   5_000_000,
				StatusCode:   2,
				Events: []store.SpanEvent{{
					Time:       ts.Add(2 * time.Millisecond),
					Name:       "exception",
					Attributes: map[string]string{"exception.type": "Error"},
				}},
				Links:              []store.SpanLink{},
				ResourceAttributes: map[string]string{},
				SpanAttributes:     map[string]string{"http.status_code": "500"},
			},
		},
	}}
	h := testAPI(t, cap)
	req := httptest.NewRequest(http.MethodGet, "/traces/AA?start=2026-08-26T00:00:00Z&end=2026-08-26T01:00:00Z", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body store.TraceDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.TraceID != "aa" || body.SpanCount != 2 || body.Spans[1].ParentSpanID != "01" {
		t.Fatalf("body %+v", body)
	}
	if body.Spans[1].Events[0].Name != "exception" {
		t.Fatalf("events %+v", body.Spans[1].Events)
	}
	if len(body.CriticalPath) != 2 || body.CriticalPath[1].Service != "checkout" || body.CriticalPathNs != 10_000_000 {
		t.Fatalf("critical_path %+v ns %d", body.CriticalPath, body.CriticalPathNs)
	}
	if len(body.Bottlenecks) != 2 || body.Bottlenecks[0].ExclusiveNs != 5_000_000 {
		t.Fatalf("bottlenecks %+v", body.Bottlenecks)
	}
	cap.mu.Lock()
	defer cap.mu.Unlock()
	if cap.lastGet.TraceID != "aa" {
		t.Fatalf("id %s", cap.lastGet.TraceID)
	}
	if cap.lastGet.Start.IsZero() || !cap.lastGet.End.After(cap.lastGet.Start) {
		t.Fatalf("window %+v", cap.lastGet)
	}
}

func TestGetTraceNotFound(t *testing.T) {
	t.Parallel()
	h := testAPI(t, Empty{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/traces/aa?start=2026-08-26T00:00:00Z&end=2026-08-26T01:00:00Z", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestGetTraceBadID(t *testing.T) {
	t.Parallel()
	h := testAPI(t, Empty{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/traces/zz", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestGetTracePartialWindow(t *testing.T) {
	t.Parallel()
	h := testAPI(t, Empty{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/traces/aa?start=2026-08-26T00:00:00Z", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestGetTraceStorageError(t *testing.T) {
	t.Parallel()
	h := testAPI(t, &captureSearch{getErr: io.ErrUnexpectedEOF})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/traces/aa?start=2026-08-26T00:00:00Z&end=2026-08-26T01:00:00Z", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestGetTraceTooLarge(t *testing.T) {
	t.Parallel()
	h := testAPI(t, &captureSearch{getErr: store.ErrTraceTooLarge})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/traces/aa?start=2026-08-26T00:00:00Z&end=2026-08-26T01:00:00Z", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestListServicesCatalog(t *testing.T) {
	t.Parallel()
	seen := time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC)
	cap := &captureSearch{services: []store.ServiceRow{
		{Service: "checkout", LastSeen: seen},
	}}
	h := testAPI(t, cap)
	req := httptest.NewRequest(http.MethodGet, "/services?start=2026-08-26T00:00:00Z&end=2026-08-26T01:00:00Z&limit=20", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body servicesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Services) != 1 || body.Services[0].Service != "checkout" {
		t.Fatalf("body %+v", body)
	}
	cap.mu.Lock()
	defer cap.mu.Unlock()
	if cap.lastServices.Limit != 20 || cap.lastServices.Start.IsZero() {
		t.Fatalf("query %+v", cap.lastServices)
	}
}

func TestListServicesRequiresLimit(t *testing.T) {
	t.Parallel()
	h := testAPI(t, Empty{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestListServicesEmpty(t *testing.T) {
	t.Parallel()
	h := testAPI(t, Empty{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services?limit=10", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body servicesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Services == nil || len(body.Services) != 0 {
		t.Fatalf("services %#v", body.Services)
	}
}

func TestListServicesStorageError(t *testing.T) {
	t.Parallel()
	h := testAPI(t, &captureSearch{servicesErr: io.ErrUnexpectedEOF})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/services?limit=10", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestListOperationsCatalog(t *testing.T) {
	t.Parallel()
	cap := &captureSearch{operations: []store.OperationRow{
		{Operation: "HTTP POST /checkout", Spans: 4, Errors: 1, P95Ns: 40_000_000},
	}}
	h := testAPI(t, cap)
	req := httptest.NewRequest(http.MethodGet, "/operations?start=2026-08-26T00:00:00Z&end=2026-08-26T01:00:00Z&limit=20&service=checkout", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body operationsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Operations) != 1 || body.Operations[0].Operation != "HTTP POST /checkout" {
		t.Fatalf("body %+v", body)
	}
	if body.Operations[0].Spans != 4 || body.Operations[0].P95Ns != 40_000_000 {
		t.Fatalf("stats %+v", body.Operations[0])
	}
	cap.mu.Lock()
	defer cap.mu.Unlock()
	if cap.lastOperations.Service != "checkout" || cap.lastOperations.Limit != 20 {
		t.Fatalf("query %+v", cap.lastOperations)
	}
}

func TestListOperationsRequiresService(t *testing.T) {
	t.Parallel()
	h := testAPI(t, Empty{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/operations?limit=10", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestListOperationsEmpty(t *testing.T) {
	t.Parallel()
	h := testAPI(t, Empty{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/operations?limit=10&service=checkout", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body operationsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Operations == nil || len(body.Operations) != 0 {
		t.Fatalf("operations %#v", body.Operations)
	}
}

func TestListOperationsStorageError(t *testing.T) {
	t.Parallel()
	h := testAPI(t, &captureSearch{operationsErr: io.ErrUnexpectedEOF})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/operations?limit=10&service=checkout", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestServiceMapGraph(t *testing.T) {
	t.Parallel()
	cap := &captureSearch{graph: &store.ServiceMapGraph{
		Nodes: []store.ServiceMapNode{{Service: "gateway", Spans: 4, Errors: 1}},
		Edges: []store.ServiceMapEdge{{From: "gateway", To: "auth", Calls: 2, AvgDurationNs: 5_000_000}},
	}}
	h := testAPI(t, cap)
	req := httptest.NewRequest(http.MethodGet, "/service-map?start=2026-08-26T00:00:00Z&end=2026-08-26T01:00:00Z&limit=20", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body store.ServiceMapGraph
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Nodes) != 1 || body.Nodes[0].Service != "gateway" {
		t.Fatalf("nodes %+v", body.Nodes)
	}
	if len(body.Edges) != 1 || body.Edges[0].To != "auth" {
		t.Fatalf("edges %+v", body.Edges)
	}
	cap.mu.Lock()
	defer cap.mu.Unlock()
	if cap.lastMap.Limit != 20 || cap.lastMap.Start.IsZero() {
		t.Fatalf("query %+v", cap.lastMap)
	}
}

func TestServiceMapRequiresLimit(t *testing.T) {
	t.Parallel()
	h := testAPI(t, Empty{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/service-map", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestServiceMapEmpty(t *testing.T) {
	t.Parallel()
	h := testAPI(t, Empty{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/service-map?limit=10", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body store.ServiceMapGraph
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Nodes == nil || body.Edges == nil {
		t.Fatalf("nil slices %#v", body)
	}
}

func TestServiceMapStorageError(t *testing.T) {
	t.Parallel()
	h := testAPI(t, &captureSearch{mapErr: io.ErrUnexpectedEOF})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/service-map?limit=10", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestListMetrics(t *testing.T) {
	t.Parallel()
	cap := &captureSearch{metrics: []store.ServiceMetrics{{
		Service:   "checkout",
		Spans:     120,
		Errors:    3,
		Rate:      0.0333,
		ErrorRate: 0.025,
		AvgNs:     12_000_000,
		P50Ns:     8_000_000,
		P95Ns:     45_000_000,
		P99Ns:     90_000_000,
	}}}
	h := testAPI(t, cap)
	req := httptest.NewRequest(http.MethodGet, "/metrics?start=2026-08-26T00:00:00Z&end=2026-08-26T01:00:00Z&limit=20&service=checkout", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body metricsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.WindowS != 3600 {
		t.Fatalf("window %v", body.WindowS)
	}
	if len(body.Metrics) != 1 || body.Metrics[0].Service != "checkout" || body.Metrics[0].P95Ns != 45_000_000 {
		t.Fatalf("metrics %+v", body.Metrics)
	}
	cap.mu.Lock()
	defer cap.mu.Unlock()
	if cap.lastMetrics.Limit != 20 || cap.lastMetrics.Service != "checkout" || cap.lastMetrics.Start.IsZero() {
		t.Fatalf("query %+v", cap.lastMetrics)
	}
}

func TestListMetricsSeries(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	cap := &captureSearch{
		metrics: []store.ServiceMetrics{{Service: "checkout", Spans: 120, P95Ns: 45_000_000}},
		series: []store.ServiceSeries{{
			Service: "checkout",
			Points:  []store.MetricPoint{{Time: start, Spans: 60, Rate: 1, P95Ns: 45_000_000}},
		}},
	}
	h := testAPI(t, cap)
	req := httptest.NewRequest(http.MethodGet, "/metrics?start=2026-08-26T00:00:00Z&end=2026-08-26T01:00:00Z&limit=20&service=checkout&step=1m", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body metricsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.StepS != 60 {
		t.Fatalf("step %v", body.StepS)
	}
	if len(body.Series) != 1 || body.Series[0].Service != "checkout" || len(body.Series[0].Points) != 1 {
		t.Fatalf("series %+v", body.Series)
	}
	if body.Series[0].Points[0].P95Ns != 45_000_000 {
		t.Fatalf("p95 %+v", body.Series[0].Points[0])
	}
	cap.mu.Lock()
	defer cap.mu.Unlock()
	if cap.lastMetrics.Step != time.Minute {
		t.Fatalf("step %+v", cap.lastMetrics.Step)
	}
}

func TestListMetricsRequiresLimit(t *testing.T) {
	t.Parallel()
	h := testAPI(t, Empty{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestListMetricsEmpty(t *testing.T) {
	t.Parallel()
	h := testAPI(t, Empty{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics?limit=10", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body metricsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Metrics == nil || len(body.Metrics) != 0 {
		t.Fatalf("metrics %#v", body.Metrics)
	}
	if body.Series == nil || len(body.Series) != 0 {
		t.Fatalf("series %#v", body.Series)
	}
}

func TestListMetricsStorageError(t *testing.T) {
	t.Parallel()
	h := testAPI(t, &captureSearch{metricsErr: io.ErrUnexpectedEOF})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics?limit=10", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestListMetricsSeriesStorageError(t *testing.T) {
	t.Parallel()
	h := testAPI(t, &captureSearch{seriesErr: io.ErrUnexpectedEOF})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics?limit=10&step=1m", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestListErrorCauses(t *testing.T) {
	t.Parallel()
	cap := &captureSearch{causes: []store.ErrorCause{{Cause: "AuthError", Count: 31, FirstSeen: time.Date(2026, 8, 26, 12, 1, 0, 0, time.UTC)}}}
	h := testAPI(t, cap)
	req := httptest.NewRequest(http.MethodGet, "/error-causes?start=2026-08-26T00:00:00Z&end=2026-08-26T01:00:00Z&limit=5&service=auth", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body errorCausesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Causes) != 1 || body.Causes[0].Cause != "AuthError" || body.Causes[0].Count != 31 || body.Causes[0].FirstSeen.IsZero() {
		t.Fatalf("causes %+v", body.Causes)
	}
	cap.mu.Lock()
	defer cap.mu.Unlock()
	if cap.lastCauses.Service != "auth" || cap.lastCauses.Limit != 5 {
		t.Fatalf("query %+v", cap.lastCauses)
	}
}

func TestListErrorCausesFleet(t *testing.T) {
	t.Parallel()
	cap := &captureSearch{causes: []store.ErrorCause{{Cause: "card declined", Count: 201}}}
	h := testAPI(t, cap)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/error-causes?limit=5", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	cap.mu.Lock()
	defer cap.mu.Unlock()
	if cap.lastCauses.Service != "" || cap.lastCauses.Limit != 5 {
		t.Fatalf("query %+v", cap.lastCauses)
	}
}

func TestListErrorCausesEmpty(t *testing.T) {
	t.Parallel()
	h := testAPI(t, &captureSearch{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/error-causes?limit=5&service=checkout", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body errorCausesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Causes == nil || len(body.Causes) != 0 {
		t.Fatalf("causes %+v", body.Causes)
	}
}

func TestListErrorCausesStorageError(t *testing.T) {
	t.Parallel()
	h := testAPI(t, &captureSearch{causesErr: io.ErrUnexpectedEOF})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/error-causes?limit=5&service=checkout", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}
}
