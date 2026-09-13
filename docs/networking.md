# Networking Fase 2

Oikos node menjalankan `frpc` sebagai child process. `frps` adalah deployment
terpisah yang harus disediakan administrator pada alamat publik. Token frp
berbeda dari token pairing Oikos dan tidak boleh disamakan.

## Installer

Installer memerlukan:

- `OIKOS_FRPC_URL`: URL release binary frpc yang dipatok.
- `OIKOS_FRPC_SHA256`: SHA-256 binary, 64 karakter hex.
- `OIKOS_FRPC_BIN`: path instalasi opsional, default `/usr/local/bin/frpc`.

Binary di-download ke temporary file, checksum diverifikasi, lalu dipindah
secara atomik. Checksum salah tidak mengganti binary lama. `OIKOS_FRPC_SKIP=1`
hanya untuk development/test lokal.

## Prasyarat frps

- `frps` dapat dijangkau node pada TCP server port yang dikonfigurasi.
- Panel mengalokasikan remote port unik dari range yang disepakati.
- Token frp disimpan sebagai secret dan tidak dicetak ke log.
- Payload aplikasi tidak otomatis mendapat enkripsi tambahan dari relay frp.
- Relay private network melewati frps; HA frps dan WireGuard mesh bukan scope MVP.

## Konfigurasi node

```yaml
runtime:
  frp_binary: "/usr/local/bin/frpc"
  frp_config: "/var/lib/oikos/frpc.toml"
  frp_server_addr: "frps.example.com"
  frp_server_port: 7000
  frp_token: ""
  frp_remote_port_min: 30000
  frp_remote_port_max: 40000
```
