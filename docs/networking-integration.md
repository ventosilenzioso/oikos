# frp Integration

Fase 2 menyediakan manager dan integration test untuk deployment `frps` yang
terpisah. Host development ini tidak memiliki `frpc` dan tidak menjalankan
`frps`, sehingga test real dicatat sebagai skip, bukan dianggap lulus.

## Menjalankan frps

1. Salin `deploy/frp/frps.toml.example` ke host publik.
2. Ganti `auth.token` dengan secret acak.
3. Batasi firewall ke port control `7000` dan remote range yang dipakai Panel.
4. Jalankan `frps -c frps.toml` sebagai user non-root dengan service manager.

## Test real

Set environment berikut pada host yang memiliki frps reachable dan binary frpc:

```bash
OIKOS_FRPS_ADDR=127.0.0.1 \
OIKOS_FRPS_PORT=7000 \
OIKOS_FRP_TOKEN="$FRP_TOKEN" \
OIKOS_FRPC_BIN=/usr/local/bin/frpc \
go test -tags integration ./internal/tunnel/ -run TestFrpPublicTunnel -v -timeout 5m
```

Test tetap environment-gated dan akan menjelaskan alasan skip bila dependency
tidak tersedia. Keberhasilan unit/component test tidak membuktikan NAT publik;
klaim tersebut memerlukan hasil integration test di atas.
