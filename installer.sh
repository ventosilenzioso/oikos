#!/usr/bin/env bash
set -euo pipefail

# Download the repository installer from GitHub and run it from a complete tree.
REPOSITORY="${OIKOS_REPOSITORY:-ventosilenzioso/oikos}"
SOURCE_URL="${OIKOS_SOURCE_URL:-https://github.com/${REPOSITORY}/archive/refs/heads/main.tar.gz}"

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "Oikos currently supports Linux nodes only." >&2
  exit 1
fi
if [[ $EUID -ne 0 ]]; then
  echo "Run this installer as root (for example, with sudo)." >&2
  exit 1
fi
if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required." >&2
  exit 1
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT
curl -fsSL "$SOURCE_URL" | tar -xzf - -C "$tmp_dir" --strip-components=1
exec bash "$tmp_dir/scripts/install.sh"
