#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then printf '%s\n' 'Usage: run-chaos.sh --output DIR'; exit 0; fi
[[ "${OIKOS_CHAOS_ENABLE:-0}" == "1" ]] || { echo 'set OIKOS_CHAOS_ENABLE=1 to enable isolated chaos scenarios' >&2; exit 2; }
output=""; [[ "${1:-}" == "--output" && -n "${2:-}" ]] && output="$2" || { echo 'output is required' >&2; exit 2; }; mkdir -p "$output"
for scenario in docker-unavailable disk-unwritable panel-unreachable sqlite-locked; do printf 'scenario=%s\nstatus=pending manual verification\n' "$scenario" > "$output/$scenario.txt"; done
