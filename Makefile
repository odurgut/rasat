# Rasat — build and run. Product docs: docs/.

MODULE := github.com/odurgut/rasat
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS := -s -w -X $(MODULE)/internal/version.Version=$(VERSION) -X $(MODULE)/internal/version.Commit=$(COMMIT)

.PHONY: fmt fmt-check test vet lint build web run seed seed-live bench compose-up compose-build compose-down compose-reset ci version

COMPOSE := docker compose -f deploy/docker-compose.yml

fmt:
	gofmt -w cmd internal

test:
	go test -race -count=1 ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

web:
	cd web && npm ci && npm run build

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/rasat ./cmd/rasat

run: build
	./bin/rasat

seed:
	go run ./cmd/rasat-seed

seed-live:
	go run ./cmd/rasat-seed --live

bench:
	go run ./cmd/rasat-bench

version:
	@echo $(VERSION) $(COMMIT)

compose-up:
	$(COMPOSE) up -d --pull always

compose-build:
	VERSION="$(VERSION)" COMMIT="$(COMMIT)" $(COMPOSE) -f deploy/docker-compose.build.yml up --build -d

compose-down:
	$(COMPOSE) down

# Wipes the ClickHouse volume. Data is gone.
compose-reset:
	$(COMPOSE) down -v

ci: fmt-check test vet
	go build -trimpath -ldflags "$(LDFLAGS)" -o /tmp/rasat ./cmd/rasat
	go build -o /tmp/rasat-bench ./cmd/rasat-bench

fmt-check:
	@test -z "$$(gofmt -l cmd internal)" || (gofmt -l cmd internal && exit 1)
