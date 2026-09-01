package bench

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSearchAndDetail(t *testing.T) {
	t.Parallel()
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if r.URL.Path == "/api/traces" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"traces":[{"trace_id":"aa"}]}`))
			return
		}
		if r.URL.Path == "/api/traces/aa" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	c := Client{Base: srv.URL, Client: srv.Client()}
	end := time.Date(2026, 8, 31, 13, 0, 0, 0, time.UTC)
	start := end.Add(-time.Hour)
	s, ids := c.Search(t.Context(), start, end, 20)
	if s.Err != nil {
		t.Fatal(s.Err)
	}
	if len(ids) != 1 || ids[0] != "aa" {
		t.Fatalf("ids %v", ids)
	}
	d := c.Detail(t.Context(), ids[0], start, end)
	if d.Err != nil {
		t.Fatal(d.Err)
	}
	if n != 2 {
		t.Fatalf("calls %d", n)
	}
}

func TestPercentile(t *testing.T) {
	t.Parallel()
	samples := []Sample{
		{D: 10 * time.Millisecond},
		{D: 20 * time.Millisecond},
		{D: 30 * time.Millisecond},
		{D: 40 * time.Millisecond, Err: errProbe},
	}
	if Percentile(samples, 50) != 20*time.Millisecond {
		t.Fatalf("p50 %s", Percentile(samples, 50))
	}
	if FailCount(samples) != 1 {
		t.Fatalf("fails %d", FailCount(samples))
	}
	if SearchBudget() != 500*time.Millisecond || DetailBudget() != 200*time.Millisecond {
		t.Fatal("budgets")
	}
}

type probeErr string

func (e probeErr) Error() string { return string(e) }

const errProbe probeErr = "nope"
