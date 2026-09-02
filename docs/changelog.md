---
title: Changelog
description: What changed in each Rasat release.
---

# Changelog

Product version is a git tag: `vMAJOR.MINOR.PATCH`. The running process reports it on [`GET /version`](api.md) and in the UI rail. Until the first tag, this page is the unreleased log for the code that built this docs set.

`make build` and `make compose-build` stamp `git describe --tags --always --dirty` and the short commit into the binary. Hub images get the git tag (`v0.1.0`) from the release workflow. Builds that skip those flags (plain `go build`, Compose without `VERSION`) report `dev` / `none`.

The query HTTP API is not versioned separately. Breaking changes are listed here.

## Unreleased

- Docker Hub image `odurgut/rasat` from git tags `vX.Y.Z` (`0.1.0`, `latest`). Getting started is the image against ClickHouse you run; Compose is an optional laptop stack.
- Build identity from git (`GET /version`, UI rail).
