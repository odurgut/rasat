// Package query is the read API: bounded trace search, detail, logs, service catalog, operations, service map, derived metrics, and error causes.
package query

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/odurgut/rasat/internal/store"
)

const (
	maxLimit       = 1000
	maxAttrKey     = 256
	maxAttrValue   = 4096
	maxServiceName = 256
)

// ErrInvalid is returned when search query parameters cannot be used.
var ErrInvalid = errors.New("invalid query")

// ParseSearch reads GET /api/traces query parameters.
// start, end, and limit are required. The time window is capped by maxWindow.
func ParseSearch(q url.Values, maxWindow time.Duration) (store.TraceSearch, error) {
	if maxWindow <= 0 {
		maxWindow = 168 * time.Hour
	}

	start, err := parseTime(q.Get("start"))
	if err != nil {
		return store.TraceSearch{}, fmt.Errorf("%w: start: %w", ErrInvalid, err)
	}
	end, err := parseTime(q.Get("end"))
	if err != nil {
		return store.TraceSearch{}, fmt.Errorf("%w: end: %w", ErrInvalid, err)
	}
	if !end.After(start) {
		return store.TraceSearch{}, fmt.Errorf("%w: end must be after start", ErrInvalid)
	}
	if end.Sub(start) > maxWindow {
		return store.TraceSearch{}, fmt.Errorf("%w: time window must be <= %s", ErrInvalid, maxWindow)
	}

	limit, err := parseLimit(q.Get("limit"))
	if err != nil {
		return store.TraceSearch{}, fmt.Errorf("%w: limit: %w", ErrInvalid, err)
	}

	minDur, err := parseDurationParam(q, "min_duration", "min_duration_ns")
	if err != nil {
		return store.TraceSearch{}, fmt.Errorf("%w: min duration: %w", ErrInvalid, err)
	}
	maxDur, err := parseDurationParam(q, "max_duration", "max_duration_ns")
	if err != nil {
		return store.TraceSearch{}, fmt.Errorf("%w: max duration: %w", ErrInvalid, err)
	}
	if minDur != nil && maxDur != nil && *minDur > *maxDur {
		return store.TraceSearch{}, fmt.Errorf("%w: min duration must be <= max duration", ErrInvalid)
	}

	status, err := parseStatus(first(q, "status", "status_code"))
	if err != nil {
		return store.TraceSearch{}, fmt.Errorf("%w: status: %w", ErrInvalid, err)
	}

	attrKey, attrVal, err := parseAttr(q.Get("attr"))
	if err != nil {
		return store.TraceSearch{}, fmt.Errorf("%w: attr: %w", ErrInvalid, err)
	}

	traceID := strings.ToLower(strings.TrimSpace(first(q, "trace_id", "trace")))
	if traceID != "" && !isHex(traceID) {
		return store.TraceSearch{}, fmt.Errorf("%w: trace_id must be hex", ErrInvalid)
	}

	return store.TraceSearch{
		Start:         start,
		End:           end,
		Limit:         limit,
		Service:       strings.TrimSpace(q.Get("service")),
		Operation:     strings.TrimSpace(first(q, "operation", "op")),
		TraceID:       traceID,
		MinDurationNs: minDur,
		MaxDurationNs: maxDur,
		StatusCode:    status,
		AttrKey:       attrKey,
		AttrValue:     attrVal,
	}, nil
}

const maxLogTraceID = 128
const maxLogSpanID = 32

// ParseLogs reads GET /api/logs query parameters.
// start, end, and limit are required. The time window is capped by maxWindow.
func ParseLogs(q url.Values, maxWindow time.Duration) (store.LogSearch, error) {
	if maxWindow <= 0 {
		maxWindow = 168 * time.Hour
	}

	start, err := parseTime(q.Get("start"))
	if err != nil {
		return store.LogSearch{}, fmt.Errorf("%w: start: %w", ErrInvalid, err)
	}
	end, err := parseTime(q.Get("end"))
	if err != nil {
		return store.LogSearch{}, fmt.Errorf("%w: end: %w", ErrInvalid, err)
	}
	if !end.After(start) {
		return store.LogSearch{}, fmt.Errorf("%w: end must be after start", ErrInvalid)
	}
	if end.Sub(start) > maxWindow {
		return store.LogSearch{}, fmt.Errorf("%w: time window must be <= %s", ErrInvalid, maxWindow)
	}

	limit, err := parseLimit(q.Get("limit"))
	if err != nil {
		return store.LogSearch{}, fmt.Errorf("%w: limit: %w", ErrInvalid, err)
	}

	traceID := strings.TrimSpace(first(q, "trace_id", "trace"))
	if len(traceID) > maxLogTraceID {
		return store.LogSearch{}, fmt.Errorf("%w: trace_id is too long", ErrInvalid)
	}
	spanID := strings.TrimSpace(q.Get("span_id"))
	if len(spanID) > maxLogSpanID {
		return store.LogSearch{}, fmt.Errorf("%w: span_id is too long", ErrInvalid)
	}

	level := strings.ToUpper(strings.TrimSpace(q.Get("level")))
	if level == "WARNING" {
		level = "WARN"
	}
	if level == "ERR" {
		level = "ERROR"
	}

	return store.LogSearch{
		Start:   start,
		End:     end,
		Limit:   limit,
		Service: strings.TrimSpace(q.Get("service")),
		Level:   level,
		TraceID: traceID,
		SpanID:  spanID,
	}, nil
}

const maxTraceIDHex = 32

// ParseGet reads GET /api/traces/{id}. start and end are optional together;
// if both are omitted the window is [now-maxWindow, now].
func ParseGet(id string, q url.Values, maxWindow time.Duration, now time.Time) (store.TraceGet, error) {
	id, err := parseHexTraceID(id)
	if err != nil {
		return store.TraceGet{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	start, end, err := parseWindowBounds(q, maxWindow, now)
	if err != nil {
		return store.TraceGet{}, err
	}
	return store.TraceGet{TraceID: id, Start: start, End: end}, nil
}

func parseHexTraceID(id string) (string, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return "", errors.New("trace id is required")
	}
	if len(id) > maxTraceIDHex || !isHex(id) {
		return "", fmt.Errorf("trace id must be 1-%d hex characters", maxTraceIDHex)
	}
	return id, nil
}

// ParseServices reads GET /api/services. limit is required. start and end are
// optional together; if both are omitted the window is [now-maxWindow, now].
func ParseServices(q url.Values, maxWindow time.Duration, now time.Time) (store.ServiceList, error) {
	limit, err := parseLimit(q.Get("limit"))
	if err != nil {
		return store.ServiceList{}, fmt.Errorf("%w: limit: %w", ErrInvalid, err)
	}
	start, end, err := parseWindowBounds(q, maxWindow, now)
	if err != nil {
		return store.ServiceList{}, err
	}
	return store.ServiceList{Start: start, End: end, Limit: limit}, nil
}

// ParseOperations reads GET /api/operations. service and limit are required.
// start and end are optional together; if both are omitted the window is [now-maxWindow, now].
func ParseOperations(q url.Values, maxWindow time.Duration, now time.Time) (store.OperationList, error) {
	service := strings.TrimSpace(q.Get("service"))
	if service == "" {
		return store.OperationList{}, fmt.Errorf("%w: service is required", ErrInvalid)
	}
	list, err := ParseServices(q, maxWindow, now)
	if err != nil {
		return store.OperationList{}, err
	}
	return store.OperationList{Start: list.Start, End: list.End, Limit: list.Limit, Service: service}, nil
}

// ParseServiceMap reads GET /api/service-map. Same bounds as GET /api/services:
// limit is required; start and end default to [now-maxWindow, now].
func ParseServiceMap(q url.Values, maxWindow time.Duration, now time.Time) (store.ServiceMapQuery, error) {
	list, err := ParseServices(q, maxWindow, now)
	if err != nil {
		return store.ServiceMapQuery{}, err
	}
	return store.ServiceMapQuery(list), nil
}

// ParseMetrics reads GET /api/metrics. Same bounds as GET /api/services:
// limit is required; start and end default to [now-maxWindow, now].
// Optional service restricts aggregation to one name.
// Optional step (duration like 1m) adds a bucketed series for dashboards.
func ParseMetrics(q url.Values, maxWindow time.Duration, now time.Time) (store.MetricsQuery, error) {
	list, err := ParseServices(q, maxWindow, now)
	if err != nil {
		return store.MetricsQuery{}, err
	}
	service := strings.TrimSpace(q.Get("service"))
	if len(service) > maxServiceName {
		return store.MetricsQuery{}, fmt.Errorf("%w: service is too long", ErrInvalid)
	}
	step, err := parseStep(q.Get("step"))
	if err != nil {
		return store.MetricsQuery{}, fmt.Errorf("%w: step: %w", ErrInvalid, err)
	}
	return store.MetricsQuery{
		Start:   list.Start,
		End:     list.End,
		Limit:   list.Limit,
		Service: service,
		Step:    step,
	}, nil
}

// ParseErrorCauses reads GET /api/error-causes. Same bounds as GET /api/metrics:
// limit is required; start and end default to [now-maxWindow, now]. Service is optional.
func ParseErrorCauses(q url.Values, maxWindow time.Duration, now time.Time) (store.ErrorCausesQuery, error) {
	service := strings.TrimSpace(q.Get("service"))
	if len(service) > maxServiceName {
		return store.ErrorCausesQuery{}, fmt.Errorf("%w: service is too long", ErrInvalid)
	}
	list, err := ParseServices(q, maxWindow, now)
	if err != nil {
		return store.ErrorCausesQuery{}, err
	}
	return store.ErrorCausesQuery{Start: list.Start, End: list.End, Limit: list.Limit, Service: service}, nil
}

func parseStep(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		return 0, errors.New("must be a duration like 1m or 5m")
	}
	d = d.Truncate(time.Second)
	if d == 0 {
		return 0, errors.New("must be >= 1s")
	}
	return d, nil
}

func parseWindowBounds(q url.Values, maxWindow time.Duration, now time.Time) (time.Time, time.Time, error) {
	if maxWindow <= 0 {
		maxWindow = 168 * time.Hour
	}
	startRaw := strings.TrimSpace(q.Get("start"))
	endRaw := strings.TrimSpace(q.Get("end"))
	switch {
	case startRaw == "" && endRaw == "":
		if now.IsZero() {
			now = time.Now().UTC()
		} else {
			now = now.UTC()
		}
		return now.Add(-maxWindow), now, nil
	case startRaw == "" || endRaw == "":
		return time.Time{}, time.Time{}, fmt.Errorf("%w: start and end must both be set or both omitted", ErrInvalid)
	default:
		start, err := parseTime(startRaw)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("%w: start: %w", ErrInvalid, err)
		}
		end, err := parseTime(endRaw)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("%w: end: %w", ErrInvalid, err)
		}
		if !end.After(start) {
			return time.Time{}, time.Time{}, fmt.Errorf("%w: end must be after start", ErrInvalid)
		}
		if end.Sub(start) > maxWindow {
			return time.Time{}, time.Time{}, fmt.Errorf("%w: time window must be <= %s", ErrInvalid, maxWindow)
		}
		return start, end, nil
	}
}

func parseTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, errors.New("required")
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.UTC(), nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, errors.New("must be RFC3339")
	}
	return t.UTC(), nil
}

func parseLimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, errors.New("required")
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errors.New("must be an integer")
	}
	if n < 1 || n > maxLimit {
		return 0, fmt.Errorf("must be 1..%d", maxLimit)
	}
	return n, nil
}

func parseDurationParam(q url.Values, named, nsName string) (*uint64, error) {
	a := strings.TrimSpace(q.Get(named))
	b := strings.TrimSpace(q.Get(nsName))
	if a != "" && b != "" {
		return nil, errors.New("set only one of " + named + " or " + nsName)
	}
	if a != "" {
		d, err := time.ParseDuration(a)
		if err != nil || d < 0 {
			return nil, errors.New("must be a duration like 500ms")
		}
		n := uint64(d)
		return &n, nil
	}
	if b != "" {
		n, err := strconv.ParseUint(b, 10, 64)
		if err != nil {
			return nil, errors.New("must be nanoseconds")
		}
		return &n, nil
	}
	return nil, nil
}

func parseStatus(raw string) (*uint8, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return nil, nil
	}
	var code uint8
	switch raw {
	case "unset", "0":
		code = 0
	case "ok", "1":
		code = 1
	case "error", "err", "2":
		code = 2
	default:
		return nil, errors.New("must be unset, ok, or error")
	}
	return &code, nil
}

func parseAttr(raw string) (key, value string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", nil
	}
	k, v, ok := strings.Cut(raw, "=")
	if !ok || strings.TrimSpace(k) == "" {
		return "", "", errors.New("must be key=value")
	}
	k = strings.TrimSpace(k)
	if len(k) > maxAttrKey || len(v) > maxAttrValue {
		return "", "", errors.New("key or value too long")
	}
	return k, v, nil
}

func first(q url.Values, keys ...string) string {
	for _, k := range keys {
		if v := q.Get(k); v != "" {
			return v
		}
	}
	return ""
}

func isHex(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
