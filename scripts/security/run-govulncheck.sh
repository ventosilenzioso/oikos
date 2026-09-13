#!/usr/bin/env bash
set -euo pipefail

GOBIN="$(go env GOPATH)/bin"
TOOL_VERSION="v1.1.4"
if [[ ! -x "$GOBIN/govulncheck" ]]; then
  GOBIN="$GOBIN" go install "golang.org/x/vuln/cmd/govulncheck@$TOOL_VERSION"
fi
mkdir -p artifacts/security
"$GOBIN/govulncheck" ./... | tee artifacts/security/govulncheck.txt
