# Plugin Fase 5

Plugin Oikos adalah binary subprocess yang berkomunikasi melalui gRPC Unix
socket. Host memvalidasi manifest, checksum, executable, capability, dan
namespace route sebelum plugin aktif.

Plugin hanya boleh menerima event yang tercantum di `allowed_events` dan route
yang berada di `/plugins/<id>/`. Plugin tidak mendapat akses langsung ke SQLite,
Docker socket, filesystem host, private key, atau event bus internal.

Contoh manifest:

```yaml
id: log-to-file
name: Log to file
version: 1.0.0
binary: /var/lib/oikos/plugins/log-to-file
enabled: true
allowed_events:
  - server.crashed
allowed_routes: []
sha256: 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
```

Plugin yang crash diisolasi dari daemon dan dinonaktifkan setelah kegagalan
berulang. Event handler memiliki timeout agar plugin lambat tidak memblokir
event bus utama.
