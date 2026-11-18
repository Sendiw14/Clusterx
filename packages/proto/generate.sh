#!/usr/bin/env bash
# Reproducible Go code generation for the CLUSTERX worker protocol (TASK-M3-BE-01, AC-20).
# CI runs this and fails on any diff under gen/go. Tool versions are pinned here and in the
# dependency matrix; protoc itself must be installed at exactly PROTOC_VERSION.
set -euo pipefail

PROTOC_VERSION="29.3"
PROTOC_GEN_GO_VERSION="v1.36.12"      # google.golang.org/protobuf/cmd/protoc-gen-go
PROTOC_GEN_GO_GRPC_VERSION="v1.6.2"   # google.golang.org/grpc/cmd/protoc-gen-go-grpc

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$here"

got="$(protoc --version | awk '{print $2}')"
if [[ "$got" != "$PROTOC_VERSION" ]]; then
  echo "generate.sh: protoc $PROTOC_VERSION required, found $got" >&2
  exit 1
fi

# Plugins go into a private, version-keyed bin dir so a different global install cannot leak in.
bin="$here/.bin/go-$PROTOC_GEN_GO_VERSION-grpc-$PROTOC_GEN_GO_GRPC_VERSION"
if [[ ! -x "$bin/protoc-gen-go" || ! -x "$bin/protoc-gen-go-grpc" ]]; then
  mkdir -p "$bin"
  GOBIN="$bin" GOWORK=off go install "google.golang.org/protobuf/cmd/protoc-gen-go@$PROTOC_GEN_GO_VERSION"
  GOBIN="$bin" GOWORK=off go install "google.golang.org/grpc/cmd/protoc-gen-go-grpc@$PROTOC_GEN_GO_GRPC_VERSION"
fi

rm -rf gen/go
mkdir -p gen/go
PATH="$bin:$PATH" protoc \
  -I . \
  --go_out=gen/go --go_opt=paths=source_relative \
  --go-grpc_out=gen/go --go-grpc_opt=paths=source_relative \
  clusterx/worker/v1/worker.proto

echo "generated: $(find gen/go -name '*.go' | sort | tr '\n' ' ')"
