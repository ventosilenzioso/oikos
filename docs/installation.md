# Production Installation

Linux adalah target produksi utama. Node membutuhkan Docker, akses Docker
socket, dan koneksi outbound mTLS ke Panel. frp hanya diperlukan bila node
berada di balik NAT.

```bash
curl -fsSL https://raw.githubusercontent.com/ventosilenzioso/oikos/main/installer.sh | sudo bash
oikos install --token <one-time-token> --panel panel.example.com:9091
systemctl enable --now oikos
oikos doctor --config /etc/oikos/config.yaml
```

Hardening checklist:

- Gunakan token pairing sekali pakai dengan expiry pendek.
- Pastikan key/cert mode `0600`.
- Batasi akses user ke Docker socket.
- Bind `/metrics` dan `/healthz` ke localhost atau lindungi dengan network policy.
- Set seccomp/user namespace hanya setelah diuji terhadap egg yang dipakai.
- Simpan backup config dan SQLite di lokasi terpisah.
- Jalankan `gosec`, `govulncheck`, dan test suite sebelum deploy.

Clean VM, systemd handoff, dan production signing key: **pending manual verification**.
