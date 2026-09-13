#!/usr/bin/env bash
set -euo pipefail

: "${OIKOS_FRPC_URL:?OIKOS_FRPC_URL wajib diisi}"
: "${OIKOS_FRPC_SHA256:?OIKOS_FRPC_SHA256 wajib diisi}"
DEST="${OIKOS_FRPC_BIN:-/usr/local/bin/frpc}"

if [[ ! "$OIKOS_FRPC_SHA256" =~ ^[0-9a-fA-F]{64}$ ]]; then
  echo "checksum frpc harus berupa SHA-256 hex 64 karakter" >&2
  exit 1
fi

mkdir -p "$(dirname "$DEST")"
tmp="$(mktemp "$(dirname "$DEST")/.frpc.XXXXXX")"
trap 'rm -f "$tmp"' EXIT
curl -fsSL -o "$tmp" "$OIKOS_FRPC_URL"
actual="$(sha256sum "$tmp" | cut -d' ' -f1)"
expected="${OIKOS_FRPC_SHA256,,}"
if [[ "$actual" != "$expected" ]]; then
  echo "checksum frpc tidak cocok: expected=$expected actual=$actual" >&2
  exit 1
fi
chmod 0755 "$tmp"
mv -f "$tmp" "$DEST"
trap - EXIT
