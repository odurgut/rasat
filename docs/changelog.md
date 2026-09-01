---
title: Changelog
description: What changed in each Rasat release.
---

# Changelog

Product version is a git tag: `vMAJOR.MINOR.PATCH`. The running process reports it on [`GET /version`](api.md) and in the UI rail. This documentation tracks **current main**, not an archive per tag.

`make build` and `make compose-up` stamp `git describe --tags --always --dirty` and the short commit into the binary. Builds that skip those flags (plain `go build`, Compose without `VERSION`) report `dev` / `none`.

The query HTTP API is not versioned separately. Breaking changes are listed here.

## Unreleased

- Build identity from git (`GET /version`, UI rail, Compose via `make compose-up`).
