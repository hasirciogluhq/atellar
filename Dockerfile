# syntax=docker/dockerfile:1
#
# API server image. Build from the repository root:
#   docker build -t atellar .
# Run (DATABASE_URL and ADMIN_TOKEN are required):
#   docker run -e DATABASE_URL=... -e ADMIN_TOKEN=... -p 8080:8080 -p 9090:9090 atellar

FROM golang:1.25-alpine AS build

WORKDIR /src
ENV CGO_ENABLED=0

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY cmd/ ./cmd/
COPY pkg/ ./pkg/

ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath \
      -ldflags="-s -w -X github.com/hasirciogluhq/atellar/pkg/version.Version=${VERSION}" \
      -o /out/atellar ./cmd/atellar

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

COPY --from=build /out/atellar /usr/local/bin/atellar
COPY pkg/storage/postgres/migrations /usr/share/atellar/migrations

ENV MIGRATIONS_PATH=/usr/share/atellar/migrations
ENV PORT=8080
ENV GRPC_PORT=9090

EXPOSE 8080 9090

ENTRYPOINT ["/usr/local/bin/atellar", "server", "run"]
