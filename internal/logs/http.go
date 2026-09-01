package logs

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

	"github.com/odurgut/rasat/internal/store"
)

const contentJSON = "application/json"

// Handler is the JSON logs receiver (POST /api/logs).
type Handler struct {
	log           *slog.Logger
	writer        store.LogWriter
	insertTimeout time.Duration
	maxDecoded    int64
	maxLogs       int
	now           func() time.Time
}

// Options configure the logs handler.
type Options struct {
	InsertTimeout time.Duration
	MaxDecoded    int64
	MaxLogs       int
	Now           func() time.Time
}

// NewHandler returns an HTTP handler for POST /api/logs.
func NewHandler(log *slog.Logger, writer store.LogWriter, opt Options) (*Handler, error) {
	if log == nil {
		return nil, errors.New("nil logger")
	}
	if writer == nil {
		return nil, errors.New("nil log writer")
	}
	if opt.InsertTimeout <= 0 {
		opt.InsertTimeout = 30 * time.Second
	}
	if opt.MaxDecoded <= 0 {
		opt.MaxDecoded = 16 << 20
	}
	if opt.MaxLogs <= 0 {
		opt.MaxLogs = maxLogsPerRequest
	}
	if opt.Now == nil {
		opt.Now = time.Now
	}
	return &Handler{
		log:           log,
		writer:        writer,
		insertTimeout: opt.InsertTimeout,
		maxDecoded:    opt.MaxDecoded,
		maxLogs:       opt.MaxLogs,
		now:           opt.Now,
	}, nil
}

// ServeHTTP implements http.Handler for structured log ingest.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if !isJSON(r.Header.Get("Content-Type")) {
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
			h.log.Warn("logs read", "err", err, "request_id", requestID(r))
			http.Error(w, "invalid request body", http.StatusBadRequest)
		}
		return
	}

	rows, err := decodeBody(body, h.now(), h.maxLogs)
	if err != nil {
		switch {
		case errors.Is(err, errTooMany):
			http.Error(w, "too many logs", http.StatusRequestEntityTooLarge)
		default:
			h.log.Warn("logs decode", "err", err, "request_id", requestID(r))
			http.Error(w, "invalid logs", http.StatusBadRequest)
		}
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.insertTimeout)
	defer cancel()
	if err := h.writer.WriteLogBatch(ctx, rows); err != nil {
		h.log.Error("logs write", "err", err, "request_id", requestID(r))
		http.Error(w, "storage unavailable", http.StatusServiceUnavailable)
		return
	}

	if err := writeJSON(w, http.StatusOK, acceptResponse{Accepted: len(rows)}); err != nil {
		h.log.Error("logs response", "err", err, "request_id", requestID(r))
	}
}

type acceptResponse struct {
	Accepted int `json:"accepted"`
}

func isJSON(raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return false
	}
	mt, _, err := mime.ParseMediaType(raw)
	if err != nil {
		return false
	}
	return strings.EqualFold(mt, contentJSON)
}

var (
	errTooLarge            = errors.New("decoded body too large")
	errUnsupportedEncoding = errors.New("unsupported content-encoding")
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

// Discard is a LogWriter that drops batches. Used when tests inject /ready
// without a Store.
type Discard struct{}

// WriteLogBatch implements store.LogWriter.
func (Discard) WriteLogBatch(context.Context, []store.LogRow) error { return nil }
