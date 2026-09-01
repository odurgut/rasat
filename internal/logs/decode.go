package logs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/odurgut/rasat/internal/store"
)

const (
	maxLogsPerRequest = 10_000
	maxServiceLen     = 256
	maxMessageLen     = 64 << 10
	maxTraceIDLen     = 128
	maxSpanIDLen      = 32
	maxLevelLen       = 16
)

var (
	errEmptyBody  = errors.New("empty body")
	errInvalidLog = errors.New("invalid log")
	errTooMany    = errors.New("too many logs")
)

type incoming struct {
	Timestamp string `json:"timestamp"`
	Service   string `json:"service"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	TraceID   string `json:"trace_id"`
	SpanID    string `json:"span_id"`
}

func decodeBody(body []byte, now time.Time, max int) ([]store.LogRow, error) {
	if max <= 0 {
		max = maxLogsPerRequest
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, errEmptyBody
	}
	switch trimmed[0] {
	case '[':
		var raw []incoming
		if err := json.Unmarshal(trimmed, &raw); err != nil {
			return nil, fmt.Errorf("%w: %w", errInvalidLog, err)
		}
		if len(raw) > max {
			return nil, errTooMany
		}
		out := make([]store.LogRow, 0, len(raw))
		for i := range raw {
			row, err := normalize(raw[i], now)
			if err != nil {
				return nil, fmt.Errorf("%w: item %d: %w", errInvalidLog, i, err)
			}
			out = append(out, row)
		}
		return out, nil
	case '{':
		var raw incoming
		if err := json.Unmarshal(trimmed, &raw); err != nil {
			return nil, fmt.Errorf("%w: %w", errInvalidLog, err)
		}
		row, err := normalize(raw, now)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errInvalidLog, err)
		}
		return []store.LogRow{row}, nil
	default:
		return nil, fmt.Errorf("%w: body must be a JSON object or array", errInvalidLog)
	}
}

func normalize(in incoming, now time.Time) (store.LogRow, error) {
	service := strings.TrimSpace(in.Service)
	if service == "" {
		return store.LogRow{}, fmt.Errorf("service is required")
	}
	if utf8.RuneCountInString(service) > maxServiceLen {
		return store.LogRow{}, fmt.Errorf("service is too long")
	}

	ts := now
	if strings.TrimSpace(in.Timestamp) != "" {
		parsed, err := parseTimestamp(strings.TrimSpace(in.Timestamp))
		if err != nil {
			return store.LogRow{}, fmt.Errorf("timestamp: %w", err)
		}
		ts = parsed
	}

	msg := in.Message
	if utf8.RuneCountInString(msg) > maxMessageLen {
		msg = truncateRunes(msg, maxMessageLen)
	}

	traceID := strings.TrimSpace(in.TraceID)
	if len(traceID) > maxTraceIDLen {
		return store.LogRow{}, fmt.Errorf("trace_id is too long")
	}
	spanID := strings.TrimSpace(in.SpanID)
	if len(spanID) > maxSpanIDLen {
		return store.LogRow{}, fmt.Errorf("span_id is too long")
	}

	return store.LogRow{
		Timestamp:   ts.UTC(),
		ServiceName: service,
		Level:       normalizeLevel(in.Level),
		Message:     msg,
		TraceID:     traceID,
		SpanID:      spanID,
	}, nil
}

func parseTimestamp(raw string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		t, err := time.Parse(layout, raw)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("must be RFC3339")
}

func normalizeLevel(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "INFO"
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(unicode.ToUpper(r))
	}
	s = b.String()
	switch s {
	case "TRACE":
		return "TRACE"
	case "DEBUG":
		return "DEBUG"
	case "INFO", "INFORMATION":
		return "INFO"
	case "WARN", "WARNING":
		return "WARN"
	case "ERR", "ERROR":
		return "ERROR"
	case "FATAL", "PANIC":
		return "FATAL"
	default:
		if utf8.RuneCountInString(s) > maxLevelLen {
			s = truncateRunes(s, maxLevelLen)
		}
		if s == "" {
			return "INFO"
		}
		return s
	}
}

func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	i := 0
	for idx := range s {
		if i == n {
			return s[:idx]
		}
		i++
	}
	return s
}
