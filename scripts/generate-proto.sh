#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

export PATH="$(go env GOPATH)/bin:$PATH"

go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

mkdir -p internal/grpc/gen/atellar/v1

protoc \
  --proto_path=api/proto \
  --go_out=internal/grpc/gen \
  --go_opt=paths=source_relative \
  --go-grpc_out=internal/grpc/gen \
  --go-grpc_opt=paths=source_relative \
  atellar/v1/agent.proto

echo "proto generated"
