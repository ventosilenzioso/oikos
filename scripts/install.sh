#!/usr/bin/env bash
# Installer zero-config Oikos (fondasi): deteksi OS, pasang binary,
# buat systemd unit, lalu jalankan `oikos install`.
set -euo pipefail

BIN_URL="${OIKOS_BIN_URL:-https://github.com/oikos/oikos/releases/latest/download/oikos-linux-amd64}"
INSTALL_BIN="${OIKOS_INSTALL_BIN:-/usr/local/bin/oikos}"
CONFIG_DIR="${OIKOS_CONFIG_DIR:-/etc/oikos}"
DATA_DIR="${OIKOS_DATA_DIR:-/var/lib/oikos}"
SYSTEMD_DIR="${OIKOS_SYSTEMD_DIR:-/etc/systemd/system}"
FRPC_BIN="${OIKOS_FRPC_BIN:-/usr/local/bin/frpc}"

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "oikos install.sh hanya mendukung Linux" >&2
  exit 1
fi

if [[ $EUID -ne 0 ]]; then
  echo "jalankan sebagai root (sudo)" >&2
  exit 1
fi

echo "[1/4] download binary oikos..."
curl -fsSL -o "$INSTALL_BIN" "$BIN_URL"
chmod 0755 "$INSTALL_BIN"

if [[ "${OIKOS_FRPC_SKIP:-0}" == "1" ]]; then
  echo "frpc download dilewati (OIKOS_FRPC_SKIP=1)"
else
  if [[ -z "${OIKOS_FRPC_URL:-}" || -z "${OIKOS_FRPC_SHA256:-}" ]]; then
    echo "OIKOS_FRPC_URL dan OIKOS_FRPC_SHA256 wajib diisi, atau gunakan OIKOS_FRPC_SKIP=1 untuk development" >&2
    exit 1
  fi
  OIKOS_FRPC_BIN="$FRPC_BIN" bash scripts/install-frpc.sh
fi

echo "[2/4] buat direktori config..."
mkdir -p "$CONFIG_DIR/certs" "$DATA_DIR"
chmod 0750 "$CONFIG_DIR" "$DATA_DIR"

echo "[3/4] pasang systemd unit..."
mkdir -p "$SYSTEMD_DIR"
cp deploy/systemd/oikos.service "$SYSTEMD_DIR/oikos.service"
if command -v systemctl >/dev/null 2>&1 && [[ -d /run/systemd/system ]]; then
  systemctl daemon-reload
  SYSTEMD_OK=1
else
  echo "systemd tidak tersedia, lewati daemon-reload/enable (jalankan binary langsung)"
  SYSTEMD_OK=0
fi

echo "[4/4] pairing awal..."
if [[ -z "${OIKOS_TOKEN:-}" ]]; then
  echo "masukkan pairing token dari Panel:"
  read -r -s OIKOS_TOKEN
  echo
fi
"$INSTALL_BIN" install --token "$OIKOS_TOKEN" --config "$CONFIG_DIR/config.yaml"

if [[ "$SYSTEMD_OK" == "1" ]]; then
  echo "selesai. aktifkan daemon: systemctl enable --now oikos"
else
  echo "selesai. jalankan daemon: $INSTALL_BIN daemon --config $CONFIG_DIR/config.yaml"
fi
