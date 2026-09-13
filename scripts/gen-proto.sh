#!/usr/bin/env bash
set -euo pipefail
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
export PATH="$PATH:$(go env GOPATH)/bin"
mkdir -p gen/go
protoc -I proto \
  --go_out=gen/go --go_opt=paths=source_relative \
  --go-grpc_out=gen/go --go-grpc_opt=paths=source_relative \
  proto/node/node.proto proto/server/server.proto proto/stream/stream.proto proto/tunnel/tunnel.proto proto/observability/observability.proto
