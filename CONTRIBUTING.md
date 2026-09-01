# Contributing to Rasat

User documentation is the product site source in [docs/](docs/index.md). Write it for people who run and use Rasat, not for people browsing this tree. When a change alters behavior, update those pages.

## Code

- Go: `gofmt`, `go test -race ./...`, `go vet ./...`. If you have it: `golangci-lint run`.
- New behavior needs a test. Prefer table-driven tests and injected dependencies (`LoadFrom`, `app.Options.Listener`) over `t.Setenv` and global state.
- Errors wrap with `%w`. No `panic` on the request path. No `slog.SetDefault` as an API.
- `cmd/rasat` stays thin: `InitApp` with the **unit list**, `WatchShutdown`, `Wait`. Each subsystem is an `app.Unit` (`InitFn` / reverse `CloseFn`) writing into `App`. HTTP lives in `internal/httpapi`. No globals.
- TypeScript: `strict` stays on. Do not enlarge type to 14px. UI is 12px mono, `border-radius: 0`, no type scale.

## PR

Keep the change focused. Do not sneak in auth, Kubernetes, tenants, or alerts.

By opening a pull request you license your contribution under Apache License 2.0.
