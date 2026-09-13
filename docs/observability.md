# Observability dan Reliability Fase 3

## Endpoint

Daemon menyediakan endpoint HTTP observability pada `127.0.0.1:9191` secara
default:

- `/metrics`: exposition format Prometheus.
- `/healthz`: JSON health report.

Keduanya sengaja bind localhost. Jangan expose port ini ke jaringan publik tanpa
authentication dan network policy tambahan.

## Konfigurasi

```yaml
observability:
  bind_addr: "127.0.0.1:9191"
  metrics_path: "/metrics"
  health_path: "/healthz"
  event_retention_days: 30
  metrics_interval: "15s"
  health_interval: "15s"
```

Metrics dikumpulkan ke snapshot berkala sehingga request scrape tidak memanggil
Docker API secara langsung.

## Metrics

- `oikos_servers_total{status}`: jumlah server per status.
- `oikos_server_cpu_usage_percent{server_id}`: CPU server.
- `oikos_server_memory_usage_bytes{server_id}`: memory server.
- `oikos_tunnel_status{server_id,status}`: status tunnel.
- `oikos_daemon_uptime_seconds`: uptime daemon.
- `oikos_selfheal_restarts_total{server_id}`: counter restart otomatis.

## Health

- `healthy`: SQLite, Docker, dan Panel sehat.
- `degraded`: dependency Panel tidak tersedia tetapi daemon masih dapat
  menjalankan operasi lokal.
- `unhealthy`: SQLite/Docker gagal atau beberapa dependency gagal.

Health endpoint mengembalikan HTTP 200 untuk healthy/degraded dan HTTP 503 untuk
unhealthy.

## Self-healing

Default policy:

- Maksimum 5 retry.
- Backoff mulai 5 detik dan eksponensial.
- Backoff maksimum 5 menit.
- Counter di-reset setelah server stabil selama 5 menit.

Stop manual tidak boleh diperlakukan sebagai crash. Setelah batas retry tercapai,
status server menjadi `crash_looping` dan daemon berhenti mencoba restart.

Tunnel menggunakan policy bounded yang sama secara konseptual. Detail alerting
dan event history production diperluas pada fase berikutnya.

## Event history

Event penting disimpan di tabel SQLite `events` sebelum broadcast realtime.
Metadata harus JSON valid. Retention default adalah 30 hari; pruning dilakukan
melalui operasi store terjadwal, bukan query HTTP.

Event history dapat difilter berdasarkan `server_id`, `type`, dan rentang waktu.
Jangan menyimpan token, private key, password, atau credential dalam metadata.

## Operasional

Gunakan `oikos diagnose --config /etc/oikos/config.yaml` untuk pemeriksaan
config, SQLite, Docker, cgroup, pairing, dan sertifikat. Untuk scrape lokal:

```bash
curl --fail http://127.0.0.1:9191/metrics
curl --fail http://127.0.0.1:9191/healthz
```

Dashboard, distributed tracing, dan external alerting bukan bagian daemon Fase
3; Panel atau monitoring deployment dapat mengonsumsi endpoint standar ini.
