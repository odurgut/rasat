package httpapi

import "context"

// ReadyChecker reports whether storage can serve traffic.
// The ClickHouse implementation lives in internal/store; tests inject stubs.
type ReadyChecker interface {
	Ready(ctx context.Context) error
}
