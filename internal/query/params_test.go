package query

import (
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestParseSearchRequired(t *testing.T) {
	t.Parallel()
	_, err := ParseSearch(url.Values{}, time.Hour)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}

func TestParseSearch(t *testing.T) {
	t.Parallel()
	start := "2026-08-26T00:00:00Z"
	end := "2026-08-26T01:00:00Z"
	q := url.Values{
		"start":        {start},
		"end":          {end},
		"limit":        {"50"},
		"service":      {"checkout"},
		"op":           {"GET /pay"},
		"trace_id":     {"AABBCC"},
		"min_duration": {"500ms"},
		"status":       {"error"},
		"attr":         {"http.method=GET"},
	}
	got, err := ParseSearch(q, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if got.Service != "checkout" || got.Operation != "GET /pay" {
		t.Fatalf("identity %+v", got)
	}
	if got.TraceID != "aabbcc" {
		t.Fatalf("trace %s", got.TraceID)
	}
	if got.MinDurationNs == nil || *got.MinDurationNs != uint64(500*time.Millisecond) {
		t.Fatalf("duration %+v", got.MinDurationNs)
	}
	if got.StatusCode == nil || *got.StatusCode != 2 {
		t.Fatalf("status %+v", got.StatusCode)
	}
	if got.AttrKey != "http.method" || got.AttrValue != "GET" {
		t.Fatalf("attr %s=%s", got.AttrKey, got.AttrValue)
	}
	if got.Limit != 50 {
		t.Fatalf("limit %d", got.Limit)
	}
}

func TestParseLogs(t *testing.T) {
	t.Parallel()
	got, err := ParseLogs(url.Values{
		"start":    {"2026-08-26T00:00:00Z"},
		"end":      {"2026-08-26T01:00:00Z"},
		"limit":    {"20"},
		"service":  {"checkout"},
		"level":    {"error"},
		"trace_id": {"abc123"},
	}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if got.Service != "checkout" || got.Level != "ERROR" || got.TraceID != "abc123" || got.Limit != 20 {
		t.Fatalf("%+v", got)
	}
}

func TestParseLogsRequiresBounds(t *testing.T) {
	t.Parallel()
	_, err := ParseLogs(url.Values{}, time.Hour)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}

func TestParseSearchErrors(t *testing.T) {
	t.Parallel()
	base := url.Values{
		"start": {"2026-08-26T00:00:00Z"},
		"end":   {"2026-08-26T01:00:00Z"},
		"limit": {"10"},
	}
	tests := []struct {
		name string
		mod  func(url.Values)
	}{
		{name: "window", mod: func(v url.Values) { v.Set("end", "2026-08-27T02:00:00Z") }},
		{name: "limit", mod: func(v url.Values) { v.Set("limit", "0") }},
		{name: "limit high", mod: func(v url.Values) { v.Set("limit", "1001") }},
		{name: "status", mod: func(v url.Values) { v.Set("status", "nope") }},
		{name: "trace", mod: func(v url.Values) { v.Set("trace_id", "zz") }},
		{name: "attr", mod: func(v url.Values) { v.Set("attr", "nokey") }},
		{name: "both dur", mod: func(v url.Values) {
			v.Set("min_duration", "1ms")
			v.Set("min_duration_ns", "1")
		}},
		{name: "inverted", mod: func(v url.Values) { v.Set("end", "2026-08-25T00:00:00Z") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := url.Values{}
			for k, vals := range base {
				v[k] = append([]string{}, vals...)
			}
			tt.mod(v)
			_, err := ParseSearch(v, time.Hour)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestParseStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		code uint8
		ok   bool
	}{
		{in: "", ok: false},
		{in: "error", code: 2, ok: true},
		{in: "ERR", code: 2, ok: true},
		{in: "ok", code: 1, ok: true},
		{in: "unset", code: 0, ok: true},
		{in: "2", code: 2, ok: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			got, err := parseStatus(tt.in)
			if !tt.ok {
				if err != nil || got != nil {
					t.Fatalf("empty: %v %v", got, err)
				}
				return
			}
			if err != nil || got == nil || *got != tt.code {
				t.Fatalf("got %v err %v", got, err)
			}
		})
	}
}

func TestParseGetDefaultWindow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	got, err := ParseGet("AABB", url.Values{}, time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.TraceID != "aabb" {
		t.Fatalf("id %s", got.TraceID)
	}
	if !got.End.Equal(now) || got.End.Sub(got.Start) != time.Hour {
		t.Fatalf("window %s %s", got.Start, got.End)
	}
}

func TestParseGetExplicitWindow(t *testing.T) {
	t.Parallel()
	q := url.Values{
		"start": {"2026-08-26T00:00:00Z"},
		"end":   {"2026-08-26T01:00:00Z"},
	}
	got, err := ParseGet("aa", q, 24*time.Hour, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Start.Equal(time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("start %s", got.Start)
	}
}

func TestParseGetErrors(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		id   string
		q    url.Values
	}{
		{name: "empty", id: "", q: url.Values{}},
		{name: "hex", id: "zz", q: url.Values{}},
		{name: "long", id: strings.Repeat("a", 33), q: url.Values{}},
		{name: "partial", id: "aa", q: url.Values{"start": {"2026-08-26T00:00:00Z"}}},
		{name: "window", id: "aa", q: url.Values{
			"start": {"2026-08-26T00:00:00Z"},
			"end":   {"2026-08-27T02:00:00Z"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseGet(tt.id, tt.q, time.Hour, now)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestParseServices(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	got, err := ParseServices(url.Values{"limit": {"20"}}, time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Limit != 20 || !got.End.Equal(now) || got.End.Sub(got.Start) != time.Hour {
		t.Fatalf("%+v", got)
	}

	q := url.Values{
		"limit": {"5"},
		"start": {"2026-08-26T00:00:00Z"},
		"end":   {"2026-08-26T01:00:00Z"},
	}
	got, err = ParseServices(q, 24*time.Hour, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Limit != 5 || !got.Start.Equal(time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("%+v", got)
	}
}

func TestParseServicesErrors(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	tests := []url.Values{
		{},
		{"limit": {"0"}},
		{"limit": {"10"}, "start": {"2026-08-26T00:00:00Z"}},
		{"limit": {"10"}, "start": {"2026-08-26T00:00:00Z"}, "end": {"2026-08-27T02:00:00Z"}},
	}
	for _, q := range tests {
		if _, err := ParseServices(q, time.Hour, now); !errors.Is(err, ErrInvalid) {
			t.Fatalf("got %v for %v", err, q)
		}
	}
}

func TestParseOperations(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	got, err := ParseOperations(url.Values{"limit": {"20"}, "service": {"checkout"}}, time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Service != "checkout" || got.Limit != 20 || !got.End.Equal(now) {
		t.Fatalf("%+v", got)
	}
}

func TestParseOperationsRequiresService(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	if _, err := ParseOperations(url.Values{"limit": {"10"}}, time.Hour, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}

func TestParseServiceMap(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	got, err := ParseServiceMap(url.Values{"limit": {"20"}}, time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Limit != 20 || !got.End.Equal(now) || got.End.Sub(got.Start) != time.Hour {
		t.Fatalf("%+v", got)
	}
}

func TestParseMetrics(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	got, err := ParseMetrics(url.Values{"limit": {"20"}, "service": {"checkout"}}, time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Limit != 20 || got.Service != "checkout" || !got.End.Equal(now) || got.End.Sub(got.Start) != time.Hour {
		t.Fatalf("%+v", got)
	}

	q := url.Values{
		"limit": {"5"},
		"start": {"2026-08-26T00:00:00Z"},
		"end":   {"2026-08-26T01:00:00Z"},
	}
	got, err = ParseMetrics(q, 24*time.Hour, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Service != "" || got.Limit != 5 || !got.Start.Equal(time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("%+v", got)
	}

	got, err = ParseMetrics(url.Values{
		"limit": {"5"},
		"start": {"2026-08-26T00:00:00Z"},
		"end":   {"2026-08-26T01:00:00Z"},
		"step":  {"1m"},
	}, 24*time.Hour, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Step != time.Minute {
		t.Fatalf("step %s", got.Step)
	}
}

func TestParseMetricsErrors(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	if _, err := ParseMetrics(url.Values{}, time.Hour, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
	long := strings.Repeat("a", maxServiceName+1)
	if _, err := ParseMetrics(url.Values{"limit": {"10"}, "service": {long}}, time.Hour, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
	if _, err := ParseMetrics(url.Values{"limit": {"10"}, "step": {"nope"}}, time.Hour, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}

func TestParseErrorCauses(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	got, err := ParseErrorCauses(url.Values{"limit": {"5"}, "service": {"auth"}}, time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Service != "auth" || got.Limit != 5 || got.End.Sub(got.Start) != time.Hour {
		t.Fatalf("%+v", got)
	}
	fleet, err := ParseErrorCauses(url.Values{"limit": {"5"}}, time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if fleet.Service != "" || fleet.Limit != 5 {
		t.Fatalf("%+v", fleet)
	}
}

func TestParseErrorCausesErrors(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	if _, err := ParseErrorCauses(url.Values{"service": {"auth"}}, time.Hour, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
	long := strings.Repeat("a", maxServiceName+1)
	if _, err := ParseErrorCauses(url.Values{"limit": {"5"}, "service": {long}}, time.Hour, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("got %v", err)
	}
}
