## Why

<!-- One or two sentences. Same idea as the Conventional Commit subject. -->

## How you checked

- [ ] `gofmt`, `go test -race -count=1 ./...`, `go vet ./...` (and `golangci-lint` if you changed Go)
- [ ] UI: `cd web && npm ci && npm run build` if you touched `web/`
- [ ] Docs in this PR if the change alters behavior, env vars, ports, or the image (`docs/`, including changelog **Unreleased** when operators will see it)

## Scope

Do not sneak in auth, Kubernetes, tenants, alerting, OTLP logs/metrics, or a second registry.
