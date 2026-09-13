#!/usr/bin/env bash
set -euo pipefail

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  printf '%s\n' 'Usage: measure-daemon.sh --pid PID --duration SECONDS --output DIR'
  exit 0
fi
pid=""; duration="60"; output=""
while [[ $# -gt 0 ]]; do case "$1" in --pid) pid="$2"; shift 2;; --duration) duration="$2"; shift 2;; --output) output="$2"; shift 2;; *) echo "unknown argument: $1" >&2; exit 2;; esac; done
[[ "$pid" =~ ^[0-9]+$ && -n "$output" ]] || { echo 'pid and output are required' >&2; exit 2; }
mkdir -p "$output"
printf 'commit=%s\ngo=%s\ndocker=%s\nkernel=%s\n' "$(git rev-parse HEAD 2>/dev/null || printf unknown)" "$(go version 2>/dev/null || printf unknown)" "$(docker --version 2>/dev/null || printf unknown)" "$(uname -sr)" > "$output/metadata.txt"
end=$((SECONDS + duration)); : > "$output/samples.csv"; printf 'timestamp,rss_kb\n' >> "$output/samples.csv"
while (( SECONDS < end )); do rss=$(awk '/VmRSS:/{print $2}' "/proc/$pid/status" 2>/dev/null || true); [[ -n "$rss" ]] || break; printf '%s,%s\n' "$(date +%s)" "$rss" >> "$output/samples.csv"; sleep 1; done
