package logs

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/odurgut/rasat/internal/store"
)

type captureWriter struct {
	mu   sync.Mutex
	rows []store.LogRow
	err  error
}

func (c *captureWriter) WriteLogBatch(_ context.Context, rows []store.LogRow) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rows = append(c.rows, rows...)
	return c.err
}

func testHandler(t *testing.T, w store.LogWriter) *Handler {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	h, err := NewHandler(log, w, Options{InsertTimeout: time.Second, MaxDecoded: 1 << 20, Now: func() time.Time { return now }})
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

func TestHTTPJSONObject(t *testing.T) {
	t.Parallel()
	cap := &captureWriter{}
	h := testHandler(t, cap)

	body := `{"timestamp":"2026-08-01T12:00:00Z","service":"payment-service","level":"ERROR","message":"database timeout","trace_id":"abc123"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/logs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var resp acceptResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Accepted != 1 {
		t.Fatalf("accepted %d", resp.Accepted)
	}
	cap.mu.Lock()
	defer cap.mu.Unlock()
	if len(cap.rows) != 1 || cap.rows[0].ServiceName != "payment-service" || cap.rows[0].Level != "ERROR" {
		t.Fatalf("rows %+v", cap.rows)
	}
}

func TestHTTPJSONArray(t *testing.T) {
	t.Parallel()
	cap := &captureWriter{}
	h := testHandler(t, cap)
	body := `[{"service":"a","message":"one"},{"service":"b","level":"warn","message":"two"}]`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/logs", strings.NewReader(body))
	req.Header.Set("Content-Type", contentJSON)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	cap.mu.Lock()
	defer cap.mu.Unlock()
	if len(cap.rows) != 2 || cap.rows[1].Level != "WARN" {
		t.Fatalf("rows %+v", cap.rows)
	}
}

func TestHTTPGzip(t *testing.T) {
	t.Parallel()
	cap := &captureWriter{}
	h := testHandler(t, cap)
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte(`{"service":"gzip","message":"ok"}`)); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/logs", &buf)
	req.Header.Set("Content-Type", contentJSON)
	req.Header.Set("Content-Encoding", "gzip")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	cap.mu.Lock()
	defer cap.mu.Unlock()
	if len(cap.rows) != 1 || cap.rows[0].ServiceName != "gzip" {
		t.Fatalf("rows %+v", cap.rows)
	}
}

func TestHTTPRejects(t *testing.T) {
	t.Parallel()
	h := testHandler(t, &captureWriter{})
	cases := []struct {
		name   string
		method string
		ct     string
		body   string
		want   int
	}{
		{name: "get", method: http.MethodGet, ct: contentJSON, body: `{"service":"a"}`, want: http.StatusMethodNotAllowed},
		{name: "ct", method: http.MethodPost, ct: "text/plain", body: `{"service":"a"}`, want: http.StatusUnsupportedMediaType},
		{name: "bad json", method: http.MethodPost, ct: contentJSON, body: `{`, want: http.StatusBadRequest},
		{name: "no service", method: http.MethodPost, ct: contentJSON, body: `{"message":"x"}`, want: http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, "/api/logs", strings.NewReader(tc.body))
			if tc.ct != "" {
				req.Header.Set("Content-Type", tc.ct)
			}
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status %d want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestHTTPWriteError(t *testing.T) {
	t.Parallel()
	h := testHandler(t, &captureWriter{err: errors.New("clickhouse down")})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/logs", strings.NewReader(`{"service":"a","message":"x"}`))
	req.Header.Set("Content-Type", contentJSON)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestHTTPTooMany(t *testing.T) {
	t.Parallel()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := NewHandler(log, Discard{}, Options{MaxLogs: 1, InsertTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/logs", strings.NewReader(`[{"service":"a"},{"service":"b"}]`))
	req.Header.Set("Content-Type", contentJSON)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d", rec.Code)
	}
}
