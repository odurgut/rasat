package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/odurgut/rasat/internal/store"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestRunHealthAndShutdown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	grpcLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	grpcAddr := grpcLn.Addr().String()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, Options{
			Getenv:       func(string) string { return "" },
			Stdout:       io.Discard,
			Listener:     ln,
			GRPCListener: grpcLn,
			Ready:        okReady{},
		})
	}()

	client := &http.Client{Timeout: time.Second}
	var healthOK bool
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr+"/health", nil)
		if reqErr != nil {
			t.Fatal(reqErr)
		}
		resp, getErr := client.Do(req)
		if getErr != nil {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		var body struct {
			OK bool `json:"ok"`
		}
		decErr := json.NewDecoder(resp.Body).Decode(&body)
		_ = resp.Body.Close()
		if decErr == nil && resp.StatusCode == http.StatusOK && body.OK {
			healthOK = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !healthOK {
		t.Fatal("health never became ready")
	}

	tracesReq := ptraceotlp.NewExportRequest()
	body, err := tracesReq.MarshalProto()
	if err != nil {
		t.Fatal(err)
	}
	post, postErr := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://"+addr+"/v1/traces", bytes.NewReader(body))
	if postErr != nil {
		t.Fatal(postErr)
	}
	post.Header.Set("Content-Type", "application/x-protobuf")
	resp, postDoErr := client.Do(post)
	if postDoErr != nil {
		t.Fatalf("otlp: %v", postDoErr)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("otlp status %d", resp.StatusCode)
	}

	conn, dialErr := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if dialErr != nil {
		t.Fatal(dialErr)
	}
	defer func() { _ = conn.Close() }()
	grpcClient := ptraceotlp.NewGRPCClient(conn)
	exportCtx, exportCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer exportCancel()
	if _, grpcErr := grpcClient.Export(exportCtx, ptraceotlp.NewExportRequest()); grpcErr != nil {
		t.Fatalf("otlp grpc: %v", grpcErr)
	}

	search, searchErr := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr+"/api/traces", nil)
	if searchErr != nil {
		t.Fatal(searchErr)
	}
	searchResp, searchDoErr := client.Do(search)
	if searchDoErr != nil {
		t.Fatalf("search: %v", searchDoErr)
	}
	_ = searchResp.Body.Close()
	if searchResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("search missing bounds status %d", searchResp.StatusCode)
	}

	okSearch, okErr := http.NewRequestWithContext(context.Background(), http.MethodGet,
		"http://"+addr+"/api/traces?start=2026-08-26T00:00:00Z&end=2026-08-26T01:00:00Z&limit=10&status=error", nil)
	if okErr != nil {
		t.Fatal(okErr)
	}
	okResp, okDoErr := client.Do(okSearch)
	if okDoErr != nil {
		t.Fatalf("search ok: %v", okDoErr)
	}
	_ = okResp.Body.Close()
	if okResp.StatusCode != http.StatusOK {
		t.Fatalf("search status %d", okResp.StatusCode)
	}

	detail, detailErr := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr+"/api/traces/aa", nil)
	if detailErr != nil {
		t.Fatal(detailErr)
	}
	detailResp, detailDoErr := client.Do(detail)
	if detailDoErr != nil {
		t.Fatalf("detail: %v", detailDoErr)
	}
	_ = detailResp.Body.Close()
	if detailResp.StatusCode != http.StatusNotFound {
		t.Fatalf("detail status %d", detailResp.StatusCode)
	}

	svc, svcErr := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr+"/api/services", nil)
	if svcErr != nil {
		t.Fatal(svcErr)
	}
	svcResp, svcDoErr := client.Do(svc)
	if svcDoErr != nil {
		t.Fatalf("services: %v", svcDoErr)
	}
	_ = svcResp.Body.Close()
	if svcResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("services missing limit status %d", svcResp.StatusCode)
	}

	okSvc, okSvcErr := http.NewRequestWithContext(context.Background(), http.MethodGet,
		"http://"+addr+"/api/services?limit=10", nil)
	if okSvcErr != nil {
		t.Fatal(okSvcErr)
	}
	okSvcResp, okSvcDoErr := client.Do(okSvc)
	if okSvcDoErr != nil {
		t.Fatalf("services ok: %v", okSvcDoErr)
	}
	_ = okSvcResp.Body.Close()
	if okSvcResp.StatusCode != http.StatusOK {
		t.Fatalf("services status %d", okSvcResp.StatusCode)
	}

	metricsMissing, metricsErr := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr+"/api/metrics", nil)
	if metricsErr != nil {
		t.Fatal(metricsErr)
	}
	metricsResp, metricsDoErr := client.Do(metricsMissing)
	if metricsDoErr != nil {
		t.Fatalf("metrics: %v", metricsDoErr)
	}
	_ = metricsResp.Body.Close()
	if metricsResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("metrics missing limit status %d", metricsResp.StatusCode)
	}

	okMetrics, okMetricsErr := http.NewRequestWithContext(context.Background(), http.MethodGet,
		"http://"+addr+"/api/metrics?limit=10", nil)
	if okMetricsErr != nil {
		t.Fatal(okMetricsErr)
	}
	okMetricsResp, okMetricsDoErr := client.Do(okMetrics)
	if okMetricsDoErr != nil {
		t.Fatalf("metrics ok: %v", okMetricsDoErr)
	}
	_ = okMetricsResp.Body.Close()
	if okMetricsResp.StatusCode != http.StatusOK {
		t.Fatalf("metrics status %d", okMetricsResp.StatusCode)
	}

	causesMissing, causesErr := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr+"/api/error-causes?limit=5", nil)
	if causesErr != nil {
		t.Fatal(causesErr)
	}
	causesResp, causesDoErr := client.Do(causesMissing)
	if causesDoErr != nil {
		t.Fatalf("error-causes: %v", causesDoErr)
	}
	_ = causesResp.Body.Close()
	if causesResp.StatusCode != http.StatusOK {
		t.Fatalf("error-causes fleet status %d", causesResp.StatusCode)
	}

	okCauses, okCausesErr := http.NewRequestWithContext(context.Background(), http.MethodGet,
		"http://"+addr+"/api/error-causes?limit=5&service=checkout", nil)
	if okCausesErr != nil {
		t.Fatal(okCausesErr)
	}
	okCausesResp, okCausesDoErr := client.Do(okCauses)
	if okCausesDoErr != nil {
		t.Fatalf("error-causes ok: %v", okCausesDoErr)
	}
	_ = okCausesResp.Body.Close()
	if okCausesResp.StatusCode != http.StatusOK {
		t.Fatalf("error-causes status %d", okCausesResp.StatusCode)
	}

	logsMissing, logsErr := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr+"/api/logs", nil)
	if logsErr != nil {
		t.Fatal(logsErr)
	}
	logsResp, logsDoErr := client.Do(logsMissing)
	if logsDoErr != nil {
		t.Fatalf("logs: %v", logsDoErr)
	}
	_ = logsResp.Body.Close()
	if logsResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("logs missing bounds status %d", logsResp.StatusCode)
	}

	okLogs, okLogsErr := http.NewRequestWithContext(context.Background(), http.MethodGet,
		"http://"+addr+"/api/logs?start=2026-08-26T00:00:00Z&end=2026-08-26T01:00:00Z&limit=10", nil)
	if okLogsErr != nil {
		t.Fatal(okLogsErr)
	}
	okLogsResp, okLogsDoErr := client.Do(okLogs)
	if okLogsDoErr != nil {
		t.Fatalf("logs ok: %v", okLogsDoErr)
	}
	_ = okLogsResp.Body.Close()
	if okLogsResp.StatusCode != http.StatusOK {
		t.Fatalf("logs status %d", okLogsResp.StatusCode)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown timeout")
	}
}

type okReady struct{}

func (okReady) Ready(context.Context) error { return nil }

func TestRunStreamAfterIngest(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	grpcLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, Options{
			Getenv:       func(string) string { return "" },
			Stdout:       io.Discard,
			Listener:     ln,
			GRPCListener: grpcLn,
			Ready:        okReady{},
		})
	}()

	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(2 * time.Second)
	for {
		req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr+"/health", nil)
		if reqErr != nil {
			t.Fatal(reqErr)
		}
		resp, getErr := client.Do(req)
		if getErr == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("health never became ready")
		}
		time.Sleep(20 * time.Millisecond)
	}

	wsCtx, wsCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer wsCancel()
	u := "ws://" + addr + "/api/stream/traces"
	conn, resp, err := websocket.Dial(wsCtx, u, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("ws: %v", err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "checkout")
	sp := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	sp.SetTraceID(pcommon.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	sp.SetSpanID(pcommon.SpanID{1, 2, 3, 4, 5, 6, 7, 8})
	sp.SetName("GET /pay")
	start := time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC)
	sp.SetStartTimestamp(pcommon.NewTimestampFromTime(start))
	sp.SetEndTimestamp(pcommon.NewTimestampFromTime(start.Add(5 * time.Millisecond)))

	body, err := ptraceotlp.NewExportRequestFromTraces(td).MarshalProto()
	if err != nil {
		t.Fatal(err)
	}
	post, err := http.NewRequestWithContext(wsCtx, http.MethodPost, "http://"+addr+"/v1/traces", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	post.Header.Set("Content-Type", "application/x-protobuf")
	resp, err = client.Do(post)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("otlp status %d", resp.StatusCode)
	}

	var row store.TraceListRow
	if err := wsjson.Read(wsCtx, conn, &row); err != nil {
		t.Fatal(err)
	}
	if row.Service != "checkout" || row.Operation != "GET /pay" || row.SpanCount != 1 {
		t.Fatalf("row %+v", row)
	}
	if row.TraceID == "" {
		t.Fatal("empty trace_id")
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown timeout")
	}
}

type captureLogs struct {
	mu   sync.Mutex
	rows []store.LogRow
}

func (c *captureLogs) WriteLogBatch(_ context.Context, rows []store.LogRow) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rows = append(c.rows, rows...)
	return nil
}

func TestRunLogsIngest(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	grpcLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	cap := &captureLogs{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, Options{
			Getenv:       func(string) string { return "" },
			Stdout:       io.Discard,
			Listener:     ln,
			GRPCListener: grpcLn,
			Ready:        okReady{},
			LogWriter:    cap,
		})
	}()

	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(2 * time.Second)
	for {
		req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+addr+"/health", nil)
		if reqErr != nil {
			t.Fatal(reqErr)
		}
		resp, getErr := client.Do(req)
		if getErr == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("health never became ready")
		}
		time.Sleep(20 * time.Millisecond)
	}

	body := `{"timestamp":"2026-08-01T12:00:00Z","service":"payment-service","level":"ERROR","message":"database timeout","trace_id":"abc123"}`
	post, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://"+addr+"/api/logs", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	post.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(post)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("logs status %d", resp.StatusCode)
	}
	var accepted struct {
		Accepted int `json:"accepted"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&accepted); err != nil {
		t.Fatal(err)
	}
	if accepted.Accepted != 1 {
		t.Fatalf("accepted %d", accepted.Accepted)
	}
	cap.mu.Lock()
	defer cap.mu.Unlock()
	if len(cap.rows) != 1 || cap.rows[0].ServiceName != "payment-service" || cap.rows[0].Level != "ERROR" || cap.rows[0].TraceID != "abc123" {
		t.Fatalf("rows %+v", cap.rows)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown timeout")
	}
}
