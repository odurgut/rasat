package stream

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/odurgut/rasat/internal/store"
)

func TestServeHTTPMethod(t *testing.T) {
	t.Parallel()
	h := testHub(t, HubConfig{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/stream/traces", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestServeHTTPTooMany(t *testing.T) {
	t.Parallel()
	h := testHub(t, HubConfig{Buffer: 1, MaxClients: 1})
	sub, err := h.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/stream/traces", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestWebSocketReceivesPublish(t *testing.T) {
	t.Parallel()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := NewHub[store.TraceListRow](log, HubConfig{Buffer: 8, MaxClients: 8, WriteTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()

	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/stream/traces"
	conn, resp, err := websocket.Dial(ctx, u, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	want := store.TraceListRow{
		TraceID:    "aa",
		Service:    "checkout",
		Operation:  "GET /pay",
		DurationNs: 12_000_000,
		SpanCount:  1,
		Timestamp:  time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC),
		StatusCode: 1,
	}
	h.Publish(want)

	var got store.TraceListRow
	if err := wsjson.Read(ctx, conn, &got); err != nil {
		t.Fatal(err)
	}
	if got.TraceID != want.TraceID || got.Service != want.Service || got.Operation != want.Operation {
		t.Fatalf("got %+v", got)
	}
	if got.DurationNs != want.DurationNs || got.SpanCount != want.SpanCount || got.StatusCode != want.StatusCode {
		t.Fatalf("got %+v", got)
	}
	if !got.Timestamp.Equal(want.Timestamp) {
		t.Fatalf("timestamp %s", got.Timestamp)
	}

	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"trace_id":"aa"`) {
		t.Fatalf("json %s", raw)
	}
}
