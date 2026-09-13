#!/usr/bin/env bash
set -euo pipefail

test_checksum_failure_does_not_replace() {
  local dir
  dir="$(mktemp -d)"
  trap 'rm -rf "$dir"' RETURN
  printf 'old' > "$dir/frpc"
  printf 'new' > "$dir/payload"
  if OIKOS_FRPC_URL="file://$dir/payload" \
    OIKOS_FRPC_SHA256="0000000000000000000000000000000000000000000000000000000000000000" \
    OIKOS_FRPC_BIN="$dir/frpc" bash scripts/install-frpc.sh; then
    return 1
  fi
  [[ "$(<"$dir/frpc")" == old ]]
}

test_checksum_success_replaces_atomically() {
  local dir digest
  dir="$(mktemp -d)"
  trap 'rm -rf "$dir"' RETURN
  printf 'new' > "$dir/payload"
  digest="$(sha256sum "$dir/payload" | cut -d' ' -f1)"
  OIKOS_FRPC_URL="file://$dir/payload" OIKOS_FRPC_SHA256="$digest" OIKOS_FRPC_BIN="$dir/frpc" bash scripts/install-frpc.sh
  [[ "$(<"$dir/frpc")" == new ]]
  [[ -x "$dir/frpc" ]]
}

test_checksum_failure_does_not_replace
test_checksum_success_replaces_atomically
echo "install-frpc tests: PASS"
