#!/usr/bin/env bash
set -euo pipefail

GOBIN="$(go env GOPATH)/bin"
TOOL_VERSION="v2.22.8"
if [[ ! -x "$GOBIN/gosec" ]]; then
  GOBIN="$GOBIN" go install "github.com/securego/gosec/v2/cmd/gosec@$TOOL_VERSION"
fi
mkdir -p artifacts/security
"$GOBIN/gosec" -severity high -fmt text -out artifacts/security/gosec-high.txt ./...
# Keep a complete report for review, but only unresolved HIGH findings fail CI.
"$GOBIN/gosec" -fmt text -out artifacts/security/gosec.txt ./... || true
