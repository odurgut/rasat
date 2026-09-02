package stream

import (
	"testing"
	"time"
)

func TestRateLimitUnlimited(t *testing.T) {
	t.Parallel()
	if !(*RateLimit)(nil).Allow() {
		t.Fatal("nil should allow")
	}
	if NewRateLimit(0) != nil {
		t.Fatal("zero should be nil")
	}
}

func TestRateLimitCapsPerSecond(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	r := &RateLimit{n: 2, now: func() time.Time { return now }}
	if !r.Allow() {
		t.Fatal("first should allow")
	}
	if !r.Allow() {
		t.Fatal("second should allow")
	}
	if r.Allow() {
		t.Fatal("third should deny")
	}
	now = now.Add(time.Second)
	if !r.Allow() {
		t.Fatal("next window should allow")
	}
}
