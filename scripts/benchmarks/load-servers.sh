#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then printf '%s\n' 'Usage: load-servers.sh --count 0|10|50|100 --output DIR'; exit 0; fi
count=""; output=""
while [[ $# -gt 0 ]]; do case "$1" in --count) count="$2"; shift 2;; --output) output="$2"; shift 2;; *) echo "unknown argument: $1" >&2; exit 2;; esac; done
[[ "$count" =~ ^(0|10|50|100)$ && -n "$output" ]] || { echo 'count must be 0, 10, 50, or 100 and output is required' >&2; exit 2; }
mkdir -p "$output"; printf 'status=pending manual verification\ncount=%s\n' "$count" > "$output/load-test.txt"
echo "production load requires a dedicated test host; report written to $output/load-test.txt"
