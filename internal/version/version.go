// Package version is set at link time via -ldflags.
//
// Defaults are for `go test` and builds that skip ldflags. Production
// binaries use `git describe --tags --always --dirty` and the short commit
// (see the Makefile). There is no VERSION file; git tags are the source of truth.
package version

var (
	Version = "dev"
	Commit  = "none"
)
