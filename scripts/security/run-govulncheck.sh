#!/usr/bin/env bash
set -euo pipefail

GOBIN="$(go env GOPATH)/bin"
TOOL_VERSION="v1.1.4"
if [[ ! -x "$GOBIN/govulncheck" ]]; then
  GOBIN="$GOBIN" go install "golang.org/x/vuln/cmd/govulncheck@$TOOL_VERSION"
fi
mkdir -p artifacts/security
json_report="artifacts/security/govulncheck.json"
text_report="artifacts/security/govulncheck.txt"
set +e
"${GOVULNCHECK_BIN:-$GOBIN/govulncheck}" -json ./... >"$json_report" 2>"$text_report"
scanner_status=$?
set -e

if [[ "$scanner_status" -ne 0 && "$scanner_status" -ne 3 ]]; then
  cat "$text_report" >&2
  exit "$scanner_status"
fi

if [[ ! -s "$json_report" ]]; then
  cat "$text_report" >&2
  exit 1
fi

allowed_ids=(
  GO-2026-6253
  GO-2026-4887
  GO-2026-4883
)
new_ids=()
while IFS= read -r id; do
  known=0
  for allowed in "${allowed_ids[@]}"; do
    if [[ "$id" == "$allowed" ]]; then known=1; break; fi
  done
  if [[ "$known" -eq 0 ]]; then new_ids+=("$id"); fi
done < <(jq -r 'select(.finding.osv != null and (.finding.trace | length) > 1) | .finding.osv' "$json_report" | sort -u)

if [[ "${#new_ids[@]}" -gt 0 ]]; then
  printf 'govulncheck menemukan vulnerability ID baru yang belum direview: %s\n' "${new_ids[*]}" >&2
  cat "$text_report" >&2
  exit 1
fi

if [[ "$scanner_status" -eq 3 ]]; then
  printf 'govulncheck: hanya vulnerability yang sudah direview dan di-allowlist ditemukan\n' >&2
fi
