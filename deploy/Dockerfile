# syntax=docker/dockerfile:1

FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.24-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/internal/ui/dist ./internal/ui/dist
# Pass from the host (`make compose-build`) or the release workflow. .git is not in the image context.
ARG VERSION=dev
ARG COMMIT=none
RUN CGO_ENABLED=0 GOOS=linux go build \
  -trimpath \
  -ldflags="-s -w -X github.com/odurgut/rasat/internal/version.Version=${VERSION} -X github.com/odurgut/rasat/internal/version.Commit=${COMMIT}" \
  -o /out/rasat ./cmd/rasat

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/rasat /rasat
USER nonroot:nonroot
EXPOSE 8080 4317
LABEL org.opencontainers.image.title="Rasat" \
  org.opencontainers.image.description="Self-hosted observability for OpenTelemetry traces, logs, and a service map" \
  org.opencontainers.image.url="https://rasat.dev" \
  org.opencontainers.image.documentation="https://rasat.dev/docs/getting-started" \
  org.opencontainers.image.source="https://github.com/odurgut/rasat" \
  org.opencontainers.image.licenses="Apache-2.0" \
  org.opencontainers.image.vendor="odurgut"
ENTRYPOINT ["/rasat"]
