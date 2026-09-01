package logs

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDecodeObject(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	rows, err := decodeBody([]byte(`{
		"timestamp":"2026-08-01T12:00:00Z",
		"service":"payment-service",
		"level":"ERROR",
		"message":"database timeout",
		"trace_id":"abc123"
	}`), now, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("len %d", len(rows))
	}
	row := rows[0]
	if row.ServiceName != "payment-service" || row.Level != "ERROR" || row.Message != "database timeout" || row.TraceID != "abc123" {
		t.Fatalf("row %+v", row)
	}
	if !row.Timestamp.Equal(time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("ts %s", row.Timestamp)
	}
}

func TestDecodeArrayAndDefaults(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	rows, err := decodeBody([]byte(`[
		{"service":"checkout","message":"ok"},
		{"service":"checkout","level":"warning","message":"retry","span_id":"aabbccdd"}
	]`), now, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("len %d", len(rows))
	}
	if rows[0].Level != "INFO" || !rows[0].Timestamp.Equal(now) {
		t.Fatalf("defaults %+v", rows[0])
	}
	if rows[1].Level != "WARN" || rows[1].SpanID != "aabbccdd" {
		t.Fatalf("warn %+v", rows[1])
	}
}

func TestDecodeRejects(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	cases := []struct {
		name string
		body string
		max  int
		want error
	}{
		{name: "empty", body: "  ", want: errEmptyBody},
		{name: "not json", body: "nope", want: errInvalidLog},
		{name: "missing service", body: `{"message":"x"}`, want: errInvalidLog},
		{name: "bad timestamp", body: `{"service":"a","timestamp":"yesterday"}`, want: errInvalidLog},
		{name: "too many", body: `[{"service":"a"},{"service":"b"}]`, max: 1, want: errTooMany},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			max := tc.max
			if max == 0 {
				max = 10
			}
			_, err := decodeBody([]byte(tc.body), now, max)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
		})
	}
}

func TestNormalizeLevel(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"":            "INFO",
		" error ":     "ERROR",
		"err":         "ERROR",
		"warning":     "WARN",
		"INFO":        "INFO",
		"information": "INFO",
		"fatal":       "FATAL",
		"custom":      "CUSTOM",
	}
	for in, want := range cases {
		if got := normalizeLevel(in); got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestDecodeEmptyArray(t *testing.T) {
	t.Parallel()
	rows, err := decodeBody([]byte(`[]`), time.Now(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("len %d", len(rows))
	}
}

func TestDecodeTruncatesMessage(t *testing.T) {
	t.Parallel()
	msg := strings.Repeat("x", maxMessageLen+8)
	body := `{"service":"a","message":"` + msg + `"}`
	rows, err := decodeBody([]byte(body), time.Now().UTC(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len([]rune(rows[0].Message)) != maxMessageLen {
		t.Fatalf("len %d", len([]rune(rows[0].Message)))
	}
}
