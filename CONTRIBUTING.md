# Contributing to Rasat

User documentation lives in [docs/](docs/index.md) and is what [rasat.dev](https://rasat.dev) publishes. Write it for people who run Rasat, not for people browsing this tree. If a change alters behavior, env vars, ports, or the image, update those pages in the same pull request.

## Branches

`main` is the only long-lived branch. It is what CI tests and what a release tag points at.

- Open pull requests against `main`. Fork if you do not have write access.
- Use a short-lived branch for the change (`fix-stream-limit`, `docs-otlp-grpc`). Delete it after merge.
- Do not create version or release branches (`release/0.1`, `v0.1`, `develop`).
- Do not force-push `main`. Rebase your own topic branch until it is under review; after someone else has pushed to it, add commits instead.
- Maintainers may push small fixes to `main`. Everyone else uses a PR. Either way, `main` must stay green.

## Versioning

The product version is a git tag `vMAJOR.MINOR.PATCH` on `main`. No other suffix: `v0.1.0` ships, `v0.1.0-rc.1` does not (the release workflow ignores it).

While major is **0**:

| Bump | Use for |
|---|---|
| **PATCH** (`v0.1.1`) | Fixes, docs, internals. No intended break to env vars, HTTP query API, OTLP paths, or the image contract. |
| **MINOR** (`v0.2.0`) | Features. May break those contracts; list the break in the changelog. |
| **MAJOR** (`v1.0.0`) | When install, query API, and env vars are a stable contract. |

Do not move a tag after Docker Hub has that version. If the release job failed before push, fix `main` and cut the **next** patch.

The process reports the tag on [`GET /version`](docs/api.md) and in the UI rail. `make build` / `make compose-build` stamp `git describe`. Plain `go build` reports `dev` / `none`.

The query HTTP API is not versioned on the URL. Breaking changes belong in [docs/changelog.md](docs/changelog.md).

## Testing

You do not need ClickHouse or Docker for `go test`. Store and ingest tests use fakes.

```bash
gofmt -w cmd internal
go test -race -count=1 ./...
go vet ./...
golangci-lint run ./...   # v2.12.2 in CI; same major locally
cd web && npm ci && npm run build
make build
```

`make ci` is fmt-check, race tests, vet, and the Go binaries. It does not run golangci-lint or the UI build; GitHub Actions does.

Rules:

- New behavior needs a test. Prefer table-driven tests and injected dependencies (`LoadFrom`, `app.Options.Listener`) over `t.Setenv` and global state.
- Do not add a test that needs a live ClickHouse, a live Hub push, or `docker compose` in CI.
- UI: TypeScript `strict` stays on. Do not enlarge type to 14px. UI is 12px mono, `border-radius: 0`, no type scale.

To run the product from this tree: `make compose-build` (image from the checkout + ClickHouse). `make compose-up` pulls Hub `odurgut/rasat`, it does not build your branch.

## Pipeline

| Workflow | When | What |
|---|---|---|
| [ci.yml](.github/workflows/ci.yml) | PR and every push to `main` | `gofmt`, `go test -race`, `go vet`, golangci-lint **v2.12.2**, `make build`, `rasat-bench`, UI `npm run build`. Go **1.24.x**, Node **22**. |
| [release.yml](.github/workflows/release.yml) | Tag `vX.Y.Z` | The same Go/UI checks, then multi-arch image to Docker Hub, then a GitHub Release. |
| [hub-readme.yml](.github/workflows/hub-readme.yml) | `main` changes to `deploy/docker-hub.md` (or manual) | Hub short description + Overview. Does **not** build an image. |

`main` never publishes `latest`. Images exist only from tags.

Do not merge a red PR. Do not skip hooks. Do not push images from a laptop; the tag is the publish.

## Releases (maintainers)

CI on `main` is green, changelog **Unreleased** lists what is going out, then:

```bash
git checkout main
git pull
git tag v0.1.1
git push origin v0.1.1
```

That tag builds `odurgut/rasat:0.1.1`, `:0.1`, and `:latest` (`linux/amd64`, `linux/arm64`) and opens the GitHub Release.

In the same PR that you are about to tag (or immediately after):

1. Move **Unreleased** bullets under `## 0.1.1` (the tag without the `v`).
2. Leave an empty **Unreleased** section for the next cycle.
3. Pin install snippets and Hub examples to the new patch when you intend people to pull it (`odurgut/rasat:0.1.1`).

Hub credentials are the GitHub Environment **`DOCKERHUB`**. Overview updates need a Hub PAT with **Read, Write, and Delete**. Image push is not enough.

## Code

- Go **1.24.x**. Errors wrap with `%w`. No `panic` on the request path. No `slog.SetDefault` as an API.
- `cmd/rasat` stays thin: `InitApp` with the **unit list**, `WatchShutdown`, `Wait`. Each subsystem is an `app.Unit` (`InitFn` / reverse `CloseFn`) writing into `App`. HTTP lives in `internal/httpapi`. No globals.

## Pull requests

Keep the change focused. One concern per PR.

Do not sneak in auth, Kubernetes, tenants, alerting, OTLP logs/metrics, or a second registry.

Commit messages say **why**, in a complete sentence. Conventional-commit prefixes are not required.

By opening a pull request you license your contribution under Apache License 2.0.
