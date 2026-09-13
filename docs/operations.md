# Operations

## Diagnosis

```bash
oikos doctor --config /etc/oikos/config.yaml
curl http://127.0.0.1:9191/healthz
curl http://127.0.0.1:9191/metrics
```

## Update

`oikos update` manual memverifikasi checksum/signature, menjalankan candidate
health check, membackup state, dan memakai versioned slots. Container Docker
tidak direstart. Jika candidate gagal, rollback ke slot sebelumnya.

## Backup/Restore

Backup hot tersedia melalui API, tetapi file yang sedang ditulis dapat tidak
konsisten. Restore wajib dilakukan saat server stopped dan checksum harus cocok.

## Plugin

Plugin enabled berjalan subprocess via Unix socket. Registry mem-forward event
yang diizinkan dan route hanya di namespace `/plugins/<id>/`. Plugin crash tidak
menjatuhkan daemon.

## Logging

Log operational berbentuk JSON structured. Jangan memasukkan token, password,
private key, Docker credential, atau frp token.
