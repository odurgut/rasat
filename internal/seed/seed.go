// Package seed builds synthetic OTLP traces for local demos and tests.
// It does not write to ClickHouse; callers export via OTLP like a real app.
//
// The world is a small commerce platform (web-bff → gateway → checkout/catalog/…
// plus postgres, redis, elasticsearch, kafka). Scenarios cover the UI surfaces:
// search filters, waterfall trees, inspector attrs/events/links, and the map.
package seed

import (
	"math/rand"
	"time"

	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Options control how many traces to build and when they start.
type Options struct {
	Now   time.Time
	Count int
	Rand  *rand.Rand
}

type scenario func(e *emitter, remaining int, t0 time.Time) int

// Traces returns a mixed batch of shop traces. Count is the number of traces,
// not spans. Times are spread over the last 18 hours, denser toward Now.
func Traces(opt Options) ptrace.Traces {
	now := opt.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	n := opt.Count
	if n <= 0 {
		n = 16
	}
	rng := opt.Rand
	if rng == nil {
		rng = rand.New(rand.NewSource(now.UnixNano()))
	}

	e := &emitter{td: ptrace.NewTraces(), rng: rng}
	book := playbook()

	emitted := 0
	i := 0
	for emitted < n {
		sc := book[i%len(book)]
		i++
		t0 := stamp(now, emitted, rng)
		got := sc(e, n-emitted, t0)
		if got < 1 {
			got = 1
		}
		emitted += got
	}
	return e.td
}

func playbook() []scenario {
	return []scenario{
		func(e *emitter, _ int, t0 time.Time) int { return e.browse(randTrace(e.rng), t0) },
		func(e *emitter, _ int, t0 time.Time) int { return e.checkoutOK(randTrace(e.rng), t0) },
		func(e *emitter, _ int, t0 time.Time) int { return e.search(randTrace(e.rng), t0) },
		func(e *emitter, _ int, t0 time.Time) int { return e.login(randTrace(e.rng), t0) },
		func(e *emitter, _ int, t0 time.Time) int { return e.cart(randTrace(e.rng), t0) },
		func(e *emitter, _ int, t0 time.Time) int { return e.checkoutAuthFail(randTrace(e.rng), t0) },
		func(e *emitter, _ int, t0 time.Time) int { return e.checkoutPayFail(randTrace(e.rng), t0) },
		func(e *emitter, _ int, t0 time.Time) int { return e.checkoutSlow(randTrace(e.rng), t0) },
		func(e *emitter, rem int, t0 time.Time) int {
			if rem < 2 {
				return e.browse(randTrace(e.rng), t0)
			}
			return e.notifyPair(t0)
		},
		func(e *emitter, _ int, t0 time.Time) int { return e.fanout(randTrace(e.rng), t0) },
		func(e *emitter, _ int, t0 time.Time) int { return e.deep(randTrace(e.rng), t0) },
	}
}

func stamp(now time.Time, i int, rng *rand.Rand) time.Time {
	u := rng.Float64()
	ago := time.Duration(u*u*18*float64(time.Hour)) + time.Duration(i)*time.Second
	return now.Add(-ago)
}
