package store

import (
	"sort"
	"time"
)

const maxBottlenecks = 5

// Bottleneck is one span ranked by self time: duration minus the union of
// child intervals (parallel children are not double-counted).
type Bottleneck struct {
	SpanID      string `json:"span_id"`
	Service     string `json:"service"`
	Operation   string `json:"operation"`
	ExclusiveNs uint64 `json:"exclusive_ns"`
}

func attachBottlenecks(d *TraceDetail) {
	if d == nil {
		return
	}
	out := rankBottlenecks(d.Spans)
	if out == nil {
		out = []Bottleneck{}
	}
	d.Bottlenecks = out
}

func rankBottlenecks(spans []SpanDetail) []Bottleneck {
	if len(spans) == 0 {
		return []Bottleneck{}
	}
	byID := make(map[string]int, len(spans))
	kids := make(map[string][]int, len(spans))
	for i := range spans {
		byID[spans[i].SpanID] = i
	}
	for i := range spans {
		p := spans[i].ParentSpanID
		if p == "" {
			continue
		}
		if _, ok := byID[p]; !ok {
			continue
		}
		kids[p] = append(kids[p], i)
	}
	out := make([]Bottleneck, 0, len(spans))
	for i := range spans {
		s := spans[i]
		childSpans := make([]SpanDetail, 0, len(kids[s.SpanID]))
		for _, j := range kids[s.SpanID] {
			childSpans = append(childSpans, spans[j])
		}
		excl := selfTimeNs(s, childSpans)
		if excl == 0 {
			continue
		}
		out = append(out, Bottleneck{
			SpanID:      s.SpanID,
			Service:     s.Service,
			Operation:   s.Operation,
			ExclusiveNs: excl,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ExclusiveNs != out[j].ExclusiveNs {
			return out[i].ExclusiveNs > out[j].ExclusiveNs
		}
		if out[i].Service != out[j].Service {
			return out[i].Service < out[j].Service
		}
		return out[i].SpanID < out[j].SpanID
	})
	if len(out) > maxBottlenecks {
		out = out[:maxBottlenecks]
	}
	return out
}

func selfTimeNs(s SpanDetail, kids []SpanDetail) uint64 {
	covered := coverNs(s, kids)
	if s.DurationNs > covered {
		return s.DurationNs - covered
	}
	return 0
}

func coverNs(parent SpanDetail, kids []SpanDetail) uint64 {
	if len(kids) == 0 || parent.DurationNs == 0 {
		return 0
	}
	p0 := parent.Timestamp
	p1 := p0.Add(time.Duration(parent.DurationNs))
	type iv struct{ a, b time.Time }
	ivs := make([]iv, 0, len(kids))
	for _, c := range kids {
		a := c.Timestamp
		b := c.Timestamp.Add(time.Duration(c.DurationNs))
		if a.Before(p0) {
			a = p0
		}
		if b.After(p1) {
			b = p1
		}
		if b.After(a) {
			ivs = append(ivs, iv{a, b})
		}
	}
	if len(ivs) == 0 {
		return 0
	}
	sort.Slice(ivs, func(i, j int) bool { return ivs[i].a.Before(ivs[j].a) })
	start, end := ivs[0].a, ivs[0].b
	var n uint64
	for _, x := range ivs[1:] {
		if !x.a.After(end) {
			if x.b.After(end) {
				end = x.b
			}
			continue
		}
		n += uint64(end.Sub(start))
		start, end = x.a, x.b
	}
	return n + uint64(end.Sub(start))
}
