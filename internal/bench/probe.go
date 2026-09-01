package bench

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	searchBudget = 500 * time.Millisecond
	detailBudget = 200 * time.Millisecond
)

// Client probes search and detail on a running Rasat HTTP API.
type Client struct {
	Base   string
	Client *http.Client
}

// Sample is one timed request.
type Sample struct {
	D   time.Duration
	Err error
}

// Report is search + detail timings from a load run.
type Report struct {
	Search []Sample
	Detail []Sample
}

type searchBody struct {
	Traces []struct {
		TraceID string `json:"trace_id"`
	} `json:"traces"`
}

func (c Client) http() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return http.DefaultClient
}

func (c Client) base() string {
	return strings.TrimRight(c.Base, "/")
}

// Search hits GET /api/traces with a required window and limit.
func (c Client) Search(ctx context.Context, start, end time.Time, limit int) (Sample, []string) {
	q := url.Values{}
	q.Set("start", start.UTC().Format(time.RFC3339))
	q.Set("end", end.UTC().Format(time.RFC3339))
	q.Set("limit", fmt.Sprintf("%d", limit))
	u := c.base() + "/api/traces?" + q.Encode()
	t0 := time.Now()
	ids, err := c.getTraceIDs(ctx, u)
	return Sample{D: time.Since(t0), Err: err}, ids
}

// Detail hits GET /api/traces/:id with the same window.
func (c Client) Detail(ctx context.Context, id string, start, end time.Time) Sample {
	q := url.Values{}
	q.Set("start", start.UTC().Format(time.RFC3339))
	q.Set("end", end.UTC().Format(time.RFC3339))
	u := c.base() + "/api/traces/" + url.PathEscape(id) + "?" + q.Encode()
	t0 := time.Now()
	err := c.getOK(ctx, u)
	return Sample{D: time.Since(t0), Err: err}
}

func (c Client) getTraceIDs(ctx context.Context, u string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search status %d", resp.StatusCode)
	}
	var body searchBody
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(body.Traces))
	for _, row := range body.Traces {
		if row.TraceID != "" {
			ids = append(ids, row.TraceID)
		}
	}
	return ids, nil
}

func (c Client) getOK(ctx context.Context, u string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return err
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("detail status %d", resp.StatusCode)
	}
	return nil
}

// Percentile returns the duration at p (0–100) of successful samples.
func Percentile(samples []Sample, p int) time.Duration {
	ok := make([]time.Duration, 0, len(samples))
	for _, s := range samples {
		if s.Err == nil {
			ok = append(ok, s.D)
		}
	}
	if len(ok) == 0 {
		return 0
	}
	sort.Slice(ok, func(i, j int) bool { return ok[i] < ok[j] })
	if p <= 0 {
		return ok[0]
	}
	if p >= 100 {
		return ok[len(ok)-1]
	}
	i := (p * (len(ok) - 1)) / 100
	return ok[i]
}

// SearchBudget is the roadmap search target.
func SearchBudget() time.Duration { return searchBudget }

// DetailBudget is the roadmap detail target.
func DetailBudget() time.Duration { return detailBudget }

// FailCount is how many samples returned an error.
func FailCount(samples []Sample) int {
	n := 0
	for _, s := range samples {
		if s.Err != nil {
			n++
		}
	}
	return n
}
