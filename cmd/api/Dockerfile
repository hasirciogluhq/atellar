# Build from repo root:
#   docker build -f cmd/api/Dockerfile -t atellar-api .
# Run:
#   docker run --env-file cmd/api/.env.example -p 8080:8080 -p 9090:9090 atellar-api

FROM golang:1.25-alpine AS build

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/atellar-api ./cmd/api

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

COPY --from=build /out/atellar-api /usr/local/bin/atellar-api
COPY internal/db/migrations /usr/share/atellar/migrations

ENV MIGRATIONS_PATH=/usr/share/atellar/migrations
ENV PORT=8080
ENV GRPC_PORT=9090

EXPOSE 8080 9090

ENTRYPOINT ["/usr/local/bin/atellar-api"]
