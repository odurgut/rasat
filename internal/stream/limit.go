package stream

import (
	"sync"
	"time"
)

// RateLimit allows at most N events per second. Zero or a nil receiver is unlimited.
type RateLimit struct {
	n    int
	now  func() time.Time
	mu   sync.Mutex
	from time.Time
	used int
}

// NewRateLimit caps events per second. n <= 0 means unlimited (nil).
func NewRateLimit(n int) *RateLimit {
	if n <= 0 {
		return nil
	}
	return &RateLimit{n: n, now: time.Now}
}

// Allow reports whether one more event may be published in the current second.
func (r *RateLimit) Allow() bool {
	if r == nil || r.n <= 0 {
		return true
	}
	now := r.now
	if now == nil {
		now = time.Now
	}
	t := now()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.from.IsZero() || t.Sub(r.from) >= time.Second {
		r.from = t
		r.used = 0
	}
	if r.used >= r.n {
		return false
	}
	r.used++
	return true
}
