package seed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

const otlpProtobuf = "application/x-protobuf"

// Exporter sends one OTLP traces payload. Tests inject a fake.
type Exporter interface {
	Export(ctx context.Context, td ptrace.Traces) error
}

// HTTPExporter posts protobuf OTLP/HTTP to URL. Client timeout bounds each call.
type HTTPExporter struct {
	URL    string
	Client *http.Client
}

// Export implements Exporter.
func (h HTTPExporter) Export(ctx context.Context, td ptrace.Traces) error {
	client := h.Client
	if client == nil {
		client = http.DefaultClient
	}
	req := ptraceotlp.NewExportRequestFromTraces(td)
	body, err := req.MarshalProto()
	if err != nil {
		return fmt.Errorf("marshal otlp: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, h.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("otlp request: %w", err)
	}
	httpReq.Header.Set("Content-Type", otlpProtobuf)
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("otlp post: %w", err)
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("otlp post: status %d", resp.StatusCode)
	}
	return nil
}

// LogExporter sends structured logs. Tests inject a fake.
type LogExporter interface {
	ExportLogs(ctx context.Context, rows []LogRecord) error
}

// HTTPLogExporter posts a JSON array to URL (POST /api/logs).
type HTTPLogExporter struct {
	URL    string
	Client *http.Client
}

type jsonLog struct {
	Timestamp string `json:"timestamp"`
	Service   string `json:"service"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	TraceID   string `json:"trace_id"`
	SpanID    string `json:"span_id,omitempty"`
}

// ExportLogs implements LogExporter.
func (h HTTPLogExporter) ExportLogs(ctx context.Context, rows []LogRecord) error {
	if len(rows) == 0 {
		return nil
	}
	client := h.Client
	if client == nil {
		client = http.DefaultClient
	}
	body := make([]jsonLog, 0, len(rows))
	for _, r := range rows {
		body = append(body, jsonLog{
			Timestamp: r.Timestamp.UTC().Format(time.RFC3339Nano),
			Service:   r.Service,
			Level:     r.Level,
			Message:   r.Message,
			TraceID:   r.TraceID,
			SpanID:    r.SpanID,
		})
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal logs: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, h.URL, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("logs request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("logs post: %w", err)
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("logs post: status %d", resp.StatusCode)
	}
	return nil
}
