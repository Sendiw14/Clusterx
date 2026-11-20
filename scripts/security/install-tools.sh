#!/usr/bin/env bash
# Installs the pinned scanners with the Go toolchain (module checksums verified by sum.golang.org) into $GOBIN.
set -euo pipefail
export GOFLAGS=-trimpath GOWORK=off
go install golang.org/x/vuln/cmd/govulncheck@v1.9.0
go install github.com/zricethezav/gitleaks/v8@v8.30.1
go install github.com/google/osv-scanner/v2/cmd/osv-scanner@v2.7.0
go install github.com/anchore/syft/cmd/syft@v1.57.0
