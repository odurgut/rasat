package ingest

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"

	"github.com/odurgut/rasat/internal/store"
)

type captureWriter struct {
	mu    sync.Mutex
	batch store.TraceBatch
	err   error
}

func (c *captureWriter) WriteTraceBatch(_ context.Context, batch store.TraceBatch) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.batch = batch
	return c.err
}

func testHandler(t *testing.T, w store.TraceWriter) *Handler {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := NewHandler(log, w, Options{InsertTimeout: time.Second, MaxDecoded: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestNewHandlerRejectsNil(t *testing.T) {
	t.Parallel()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if _, err := NewHandler(nil, Discard{}, Options{}); err == nil {
		t.Fatal("nil logger")
	}
	if _, err := NewHandler(log, nil, Options{}); err == nil {
		t.Fatal("nil writer")
	}
}

func TestHTTPProtobufIngest(t *testing.T) {
	t.Parallel()
	cap := &captureWriter{}
	h := testHandler(t, cap)

	req := ptraceotlp.NewExportRequestFromTraces(sampleTraces())
	body, err := req.MarshalProto()
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", contentProtobuf)
	h.ServeHTTP(rec, httpReq)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != contentProtobuf {
		t.Fatalf("content-type %s", ct)
	}
	cap.mu.Lock()
	defer cap.mu.Unlock()
	if len(cap.batch.Spans) != 1 {
		t.Fatalf("spans %+v", cap.batch.Spans)
	}
	if cap.batch.Spans[0].ServiceName != "checkout" {
		t.Fatalf("service %s", cap.batch.Spans[0].ServiceName)
	}
	if cap.batch.Spans[0].DurationNs == 0 {
		t.Fatal("duration")
	}
	if cap.batch.Spans[0].SpanAttributes["http.method"] != "GET" {
		t.Fatalf("attrs %v", cap.batch.Spans[0].SpanAttributes)
	}
}

func TestHTTPJSONIngest(t *testing.T) {
	t.Parallel()
	cap := &captureWriter{}
	h := testHandler(t, cap)

	req := ptraceotlp.NewExportRequestFromTraces(sampleTraces())
	body, err := req.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json; charset=utf-8")
	h.ServeHTTP(rec, httpReq)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != contentJSON {
		t.Fatalf("content-type %s", ct)
	}
	cap.mu.Lock()
	defer cap.mu.Unlock()
	if len(cap.batch.Spans) != 1 || cap.batch.Spans[0].ServiceName != "checkout" {
		t.Fatalf("batch %+v", cap.batch)
	}
}

func TestHTTPGzipProtobuf(t *testing.T) {
	t.Parallel()
	cap := &captureWriter{}
	h := testHandler(t, cap)

	req := ptraceotlp.NewExportRequestFromTraces(sampleTraces())
	raw, err := req.MarshalProto()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/traces", &buf)
	httpReq.Header.Set("Content-Type", contentProtobuf)
	httpReq.Header.Set("Content-Encoding", "gzip")
	h.ServeHTTP(rec, httpReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	cap.mu.Lock()
	defer cap.mu.Unlock()
	if len(cap.batch.Spans) != 1 {
		t.Fatal("expected span")
	}
}

func TestHTTPErrors(t *testing.T) {
	t.Parallel()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	fail := &captureWriter{err: io.ErrUnexpectedEOF}
	h, err := NewHandler(log, fail, Options{InsertTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}

	protoBody, err := ptraceotlp.NewExportRequestFromTraces(sampleTraces()).MarshalProto()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		method string
		ct     string
		enc    string
		body   []byte
		want   int
	}{
		{name: "method", method: http.MethodGet, ct: contentProtobuf, body: protoBody, want: http.StatusMethodNotAllowed},
		{name: "media", method: http.MethodPost, ct: "text/plain", body: protoBody, want: http.StatusUnsupportedMediaType},
		{name: "encoding", method: http.MethodPost, ct: contentProtobuf, enc: "br", body: protoBody, want: http.StatusUnsupportedMediaType},
		{name: "decode", method: http.MethodPost, ct: contentProtobuf, body: []byte("not-proto"), want: http.StatusBadRequest},
		{name: "write", method: http.MethodPost, ct: contentProtobuf, body: protoBody, want: http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, "/v1/traces", bytes.NewReader(tt.body))
			if tt.ct != "" {
				req.Header.Set("Content-Type", tt.ct)
			}
			if tt.enc != "" {
				req.Header.Set("Content-Encoding", tt.enc)
			}
			h.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("status %d want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestHTTPTooLargeDecoded(t *testing.T) {
	t.Parallel()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := NewHandler(log, Discard{}, Options{MaxDecoded: 4, InsertTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/traces", strings.NewReader("12345"))
	req.Header.Set("Content-Type", contentProtobuf)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestTracesContent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		kind string
		ok   bool
	}{
		{in: "", kind: contentProtobuf, ok: true},
		{in: "application/x-protobuf", kind: contentProtobuf, ok: true},
		{in: "application/json; charset=utf-8", kind: contentJSON, ok: true},
		{in: "text/html", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			kind, ok := tracesContent(tt.in)
			if ok != tt.ok || kind != tt.kind {
				t.Fatalf("got %q %v", kind, ok)
			}
		})
	}
}
