package seed

import (
	"math/rand"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

func randTrace(rng *rand.Rand) pcommon.TraceID {
	var id pcommon.TraceID
	fillNonZero(rng, id[:])
	return id
}

func randSpan(rng *rand.Rand) pcommon.SpanID {
	var id pcommon.SpanID
	fillNonZero(rng, id[:])
	return id
}

func fillNonZero(rng *rand.Rand, b []byte) {
	for {
		_, _ = rng.Read(b)
		for _, c := range b {
			if c != 0 {
				return
			}
		}
	}
}

func jitter(rng *rand.Rand, n int) int {
	if n <= 1 {
		return 0
	}
	return rng.Intn(n)
}
