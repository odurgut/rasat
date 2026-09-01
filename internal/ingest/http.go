package ingest

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"

	"github.com/odurgut/rasat/internal/store"
)

const (
	contentProtobuf = "application/x-protobuf"
	contentJSON     = "application/json"
)

// Handler is the OTLP/HTTP traces receiver.
type Handler struct {
	log           *slog.Logger
	writer        store.TraceWriter
	insertTimeout time.Duration
	maxDecoded    int64
	maxSpans      int
}

// Options configure the OTLP/HTTP handler.
type Options struct {
	InsertTimeout time.Duration
	MaxDecoded    int64
	MaxSpans      int
}

// NewHandler returns an HTTP handler for POST /v1/traces.
func NewHandler(log *slog.Logger, writer store.TraceWriter, opt Options) (*Handler, error) {
	if log == nil {
		return nil, errors.New("nil logger")
	}
	if writer == nil {
		return nil, errors.New("nil trace writer")
	}
	if opt.InsertTimeout <= 0 {
		opt.InsertTimeout = 30 * time.Second
	}
	if opt.MaxDecoded <= 0 {
		opt.MaxDecoded = 16 << 20
	}
	if opt.MaxSpans <= 0 {
		opt.MaxSpans = maxSpansPerRequest
	}
	return &Handler{
		log:           log,
		writer:        writer,
		insertTimeout: opt.InsertTimeout,
		maxDecoded:    opt.MaxDecoded,
		maxSpans:      opt.MaxSpans,
	}, nil
}

// ServeHTTP implements http.Handler for OTLP/HTTP traces.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	kind, ok := tracesContent(r.Header.Get("Content-Type"))
	if !ok {
		http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
		return
	}

	body, err := readDecoded(r, h.maxDecoded)
	if err != nil {
		var maxErr *http.MaxBytesError
		switch {
		case errors.As(err, &maxErr), errors.Is(err, errTooLarge):
			http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
		case errors.Is(err, errUnsupportedEncoding):
			http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
		default:
			h.log.Warn("otlp read", "err", err, "request_id", requestID(r))
			http.Error(w, "invalid request body", http.StatusBadRequest)
		}
		return
	}

	req := ptraceotlp.NewExportRequest()
	switch kind {
	case contentJSON:
		err = req.UnmarshalJSON(body)
	default:
		err = req.UnmarshalProto(body)
	}
	if err != nil {
		h.log.Warn("otlp decode", "err", err, "request_id", requestID(r))
		http.Error(w, "invalid otlp traces", http.StatusBadRequest)
		return
	}

	if err := h.ExportTraces(r.Context(), req.Traces()); err != nil {
		switch {
		case errors.Is(err, ErrTooManySpans):
			http.Error(w, "too many spans", http.StatusRequestEntityTooLarge)
		case errors.Is(err, errWrite):
			h.log.Error("otlp write", "err", err, "request_id", requestID(r))
			http.Error(w, "storage unavailable", http.StatusServiceUnavailable)
		default:
			h.log.Error("otlp flatten", "err", err, "request_id", requestID(r))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	resp := ptraceotlp.NewExportResponse()
	var out []byte
	var ct string
	if kind == contentJSON {
		out, err = resp.MarshalJSON()
		ct = contentJSON
	} else {
		out, err = resp.MarshalProto()
		ct = contentProtobuf
	}
	if err != nil {
		h.log.Error("otlp response", "err", err, "request_id", requestID(r))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", ct)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(out); err != nil {
		h.log.Error("otlp write response", "err", err, "request_id", requestID(r))
	}
}

// ExportTraces flattens an OTLP payload and writes it. HTTP and gRPC share this.
func (h *Handler) ExportTraces(ctx context.Context, td ptrace.Traces) error {
	batch, err := FlattenTraces(td, h.maxSpans)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, h.insertTimeout)
	defer cancel()
	if err := h.writer.WriteTraceBatch(ctx, batch); err != nil {
		return fmt.Errorf("%w: %w", errWrite, err)
	}
	return nil
}

func tracesContent(raw string) (string, bool) {
	if strings.TrimSpace(raw) == "" {
		return contentProtobuf, true
	}
	mt, _, err := mime.ParseMediaType(raw)
	if err != nil {
		return "", false
	}
	switch strings.ToLower(mt) {
	case contentProtobuf, "application/protobuf":
		return contentProtobuf, true
	case contentJSON:
		return contentJSON, true
	default:
		return "", false
	}
}

var (
	errTooLarge            = errors.New("decoded body too large")
	errUnsupportedEncoding = errors.New("unsupported content-encoding")
	errWrite               = errors.New("otlp write")
)

func readDecoded(r *http.Request, max int64) ([]byte, error) {
	if r.Body == nil {
		return nil, io.ErrUnexpectedEOF
	}
	src := io.Reader(r.Body)
	switch strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Encoding"))) {
	case "", "identity":
	case "gzip", "x-gzip":
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			return nil, fmt.Errorf("gzip: %w", err)
		}
		defer func() { _ = gz.Close() }()
		src = gz
	default:
		return nil, errUnsupportedEncoding
	}

	limited := io.LimitReader(src, max+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > max {
		return nil, errTooLarge
	}
	return body, nil
}

func requestID(r *http.Request) string {
	return r.Header.Get("X-Request-Id")
}

// Discard is a TraceWriter that drops batches. Used when tests inject /ready
// without a Store.
type Discard struct{}

// WriteTraceBatch implements store.TraceWriter.
func (Discard) WriteTraceBatch(context.Context, store.TraceBatch) error { return nil }
