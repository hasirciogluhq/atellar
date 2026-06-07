# syntax=docker/dockerfile:1
#
# Build from repo root:
#   docker build -f cmd/api/Dockerfile -t atellar-api .
# Run:
#   docker run --env-file cmd/api/.env.example -p 8080:8080 -p 9090:9090 atellar-api
#
# If build fails with "no space left on device", free Docker disk:
#   docker builder prune -af && docker system prune -af

FROM golang:1.25-alpine AS build

RUN apk add --no-cache ca-certificates git

WORKDIR /src

ENV CGO_ENABLED=0
ENV GOMAXPROCS=2

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd/api/ ./cmd/api/
COPY internal/config/ ./internal/config/
COPY internal/db/ ./internal/db/
COPY internal/grpc/ ./internal/grpc/
COPY internal/modules/ ./internal/modules/
COPY internal/platform/ ./internal/platform/
COPY internal/cluster/ ./internal/cluster/

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/atellar-api ./cmd/api

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

COPY --from=build /out/atellar-api /usr/local/bin/atellar-api
COPY internal/db/migrations /usr/share/atellar/migrations

ENV MIGRATIONS_PATH=/usr/share/atellar/migrations
ENV PORT=8080
ENV GRPC_PORT=9090

EXPOSE 8080 9090

ENTRYPOINT ["/usr/local/bin/atellar-api"]
