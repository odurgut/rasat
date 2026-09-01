package store

import "time"

// CriticalPathStep is one span on the longest root-to-leaf chain.
type CriticalPathStep struct {
	SpanID     string `json:"span_id"`
	Service    string `json:"service"`
	Operation  string `json:"operation"`
	DurationNs uint64 `json:"duration_ns"`
}

func attachCriticalPath(d *TraceDetail) {
	if d == nil {
		return
	}
	steps, ns := criticalPath(d.Spans)
	if steps == nil {
		steps = []CriticalPathStep{}
	}
	d.CriticalPath = steps
	d.CriticalPathNs = ns
}

// criticalPath follows the child that ends last (wall clock). Path duration is
// first-start to last-end on that chain — sibling work off the path is excluded.
func criticalPath(spans []SpanDetail) ([]CriticalPathStep, uint64) {
	if len(spans) == 0 {
		return []CriticalPathStep{}, 0
	}
	byID := make(map[string]*SpanDetail, len(spans))
	for i := range spans {
		byID[spans[i].SpanID] = &spans[i]
	}
	kids := make(map[string][]*SpanDetail, len(spans))
	var roots []*SpanDetail
	for i := range spans {
		s := &spans[i]
		p := s.ParentSpanID
		if p == "" || byID[p] == nil {
			roots = append(roots, s)
			continue
		}
		kids[p] = append(kids[p], s)
	}
	cur := pickLatestEnd(roots)
	if cur == nil {
		all := make([]*SpanDetail, 0, len(spans))
		for i := range spans {
			all = append(all, &spans[i])
		}
		cur = pickLatestEnd(all)
	}
	seen := make(map[string]struct{}, len(spans))
	out := make([]CriticalPathStep, 0, 8)
	for cur != nil {
		if _, ok := seen[cur.SpanID]; ok {
			break
		}
		seen[cur.SpanID] = struct{}{}
		out = append(out, CriticalPathStep{
			SpanID:     cur.SpanID,
			Service:    cur.Service,
			Operation:  cur.Operation,
			DurationNs: cur.DurationNs,
		})
		next := pickLatestEnd(kids[cur.SpanID])
		cur = next
	}
	return out, pathWallNs(out, byID)
}

func pickLatestEnd(list []*SpanDetail) *SpanDetail {
	var best *SpanDetail
	var bestEnd time.Time
	for _, s := range list {
		if s == nil {
			continue
		}
		end := s.Timestamp.Add(time.Duration(s.DurationNs))
		if best == nil || end.After(bestEnd) || (end.Equal(bestEnd) && s.DurationNs > best.DurationNs) {
			best = s
			bestEnd = end
		}
	}
	return best
}

func pathWallNs(steps []CriticalPathStep, byID map[string]*SpanDetail) uint64 {
	if len(steps) == 0 {
		return 0
	}
	first := byID[steps[0].SpanID]
	last := byID[steps[len(steps)-1].SpanID]
	if first == nil || last == nil {
		return 0
	}
	start := first.Timestamp
	end := last.Timestamp.Add(time.Duration(last.DurationNs))
	if !end.After(start) {
		return last.DurationNs
	}
	return uint64(end.Sub(start))
}
