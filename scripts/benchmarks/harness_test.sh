#!/usr/bin/env bash
set -euo pipefail
for script in scripts/benchmarks/measure-daemon.sh scripts/benchmarks/load-servers.sh scripts/chaos/run-chaos.sh; do bash "$script" --help >/dev/null; done
if bash scripts/benchmarks/load-servers.sh --count 3 --output /tmp/f6-invalid >/dev/null 2>&1; then exit 1; fi
if OIKOS_CHAOS_ENABLE=1 bash scripts/chaos/run-chaos.sh --output /tmp/f6-chaos-test; then test -f /tmp/f6-chaos-test/docker-unavailable.txt; else exit 1; fi
echo 'harness tests: PASS'
