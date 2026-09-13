# Oikos Fase 3: Reliability dan Observability Design

Tanggal: 2026-09-13  
Status: Disetujui dalam sesi brainstorming, menunggu review file

## Tujuan

Fase 3 membuat daemon dapat dipercaya tanpa pengawasan terus-menerus dan
memberikan visibilitas operasional yang dapat di-scrape atau di-query. Scope
ini mencakup structured logging, metrics Prometheus, health check, event
history SQLite, self-healing container, dan policy recovery tunnel.

## Scope MVP

- Semua log operasional memakai JSON structured logger berbasis `zerolog`.
- Metrics Prometheus tersedia pada HTTP `/metrics` yang bind ke localhost.
- Health report tersedia pada HTTP `/healthz` dengan status `healthy`,
  `degraded`, atau `unhealthy`.
- Event penting disimpan di SQLite sebelum broadcast melalui event bus dan dapat
  di-query serta dipangkas berdasarkan retention.
- Container crash dipulihkan memakai policy retry/backoff yang eksplisit.
- Tunnel failure memakai policy recovery yang sama secara konseptual, dengan
  retry detail dan alerting tetap lokal pada fase ini.
- Proto observability menjadi adapter opsional; HTTP endpoints tetap source
  standar untuk monitoring.

## Non-goals

- Dashboard Grafana atau UI metrics/log.
- Distributed tracing.
- Alerting Slack/Discord/email.
- Self-healing yang tidak memiliki batas retry.
- Perubahan fitur filesystem, backup, atau plugin.

## Komponen Observability

### Structured logger

`internal/observability/logger.go` menyediakan logger dengan field standar:

- `time`
- `level`
- `component`
- `server_id` bila relevan
- `message`

Output utama stdout untuk systemd journal. File output opsional tidak menjadi
dependency fase ini. Token pairing, private key, frp token, dan isi credential
tidak pernah masuk field log.

### Metrics

Metrics collector melakukan polling terjadwal terhadap store/runtime dan
menyimpan snapshot ringan. Handler HTTP tidak melakukan Docker API call langsung
per request.

Metrics wajib:

```text
oikos_servers_total{status="running|stopped|crashed"}
oikos_server_cpu_usage_percent{server_id}
oikos_server_memory_usage_bytes{server_id}
oikos_tunnel_status{server_id,status}
oikos_daemon_uptime_seconds
oikos_selfheal_restarts_total{server_id}
```

Runtime error saat polling dicatat sebagai log dan tidak boleh menjatuhkan
collector maupun HTTP server. Label `server_id` hanya berasal dari server yang
terdaftar di store.

### Health check

Kontrak report:

```go
type HealthStatus string

const (
    Healthy   HealthStatus = "healthy"
    Degraded  HealthStatus = "degraded"
    Unhealthy HealthStatus = "unhealthy"
)

type Check struct {
    Name   string `json:"name"`
    Status string `json:"status"`
    Error  string `json:"error,omitempty"`
}

type HealthReport struct {
    Status    HealthStatus `json:"status"`
    Checks    []Check      `json:"checks"`
    Timestamp time.Time    `json:"timestamp"`
}
```

Health probes di-inject untuk SQLite, Docker, dan Panel. Handler mengembalikan
HTTP 200 untuk `healthy` dan `degraded`, serta HTTP 503 untuk `unhealthy`.
Default bind `127.0.0.1` mencegah metrics dan dependency state terekspos ke
jaringan tanpa keputusan operator.

## Reliability

### Restart policy

```go
type RestartPolicy struct {
    MaxRetries       int
    BackoffBase      time.Duration
    BackoffMax       time.Duration
    StableResetAfter time.Duration
}

var DefaultRestartPolicy = RestartPolicy{
    MaxRetries:       5,
    BackoffBase:      5 * time.Second,
    BackoffMax:       5 * time.Minute,
    StableResetAfter: 5 * time.Minute,
}
```

`nextBackoff(attempt)` menghitung exponential backoff dan melakukan cap pada
`BackoffMax`. Self-healer subscribe `server.crashed`, membaca state server
terbaru, menunggu backoff, lalu memanggil `RestartServer`. Counter bertambah
hanya setelah restart sukses. Setelah `MaxRetries` tercapai, status menjadi
`crash_looping` dan event `server.recovery_failed` dipublish. Counter di-reset
setelah server running stabil selama `StableResetAfter`.

Stop manual harus ditandai sebagai operasi manual atau menghasilkan state
stopped yang dikenali watcher. Exit normal dari stop manual tidak boleh
menghasilkan `server.crashed`.

Tunnel manager Fase 2 menggunakan policy retry yang sama secara konsep, namun
event tunnel dan process lifecycle tetap terpisah dari container lifecycle.

## Event history

Migration `0003_observability.sql` menambah:

```sql
CREATE TABLE events (
    id         TEXT PRIMARY KEY,
    type       TEXT NOT NULL,
    server_id  TEXT REFERENCES servers(id) ON DELETE CASCADE,
    severity   TEXT NOT NULL DEFAULT 'info',
    message    TEXT NOT NULL,
    metadata   TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_events_created_at ON events(created_at);
CREATE INDEX idx_events_server_id ON events(server_id);

ALTER TABLE servers ADD COLUMN restart_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE servers ADD COLUMN last_crash_at DATETIME;
```

Event history ditulis sebelum realtime broadcast untuk event penting. Metadata
wajib JSON valid. Store menyediakan `ListEvents` dengan filter type/server/time
dan `PruneEvents(ctx, before)`; pruning dipanggil scheduler harian yang dikelola
daemon, bukan goroutine tersembunyi di layer persistence.

## HTTP dan konfigurasi

Default:

```yaml
observability:
  bind_addr: "127.0.0.1:9191"
  metrics_path: "/metrics"
  health_path: "/healthz"
  event_retention_days: 30
  metrics_interval: "15s"
  health_interval: "15s"
```

Server HTTP dipisahkan dari gRPC API lokal. Shutdown memakai context dan
`http.Server.Shutdown` dengan timeout agar tidak meninggalkan goroutine.

## Proto observability

`proto/observability/observability.proto` menyediakan adapter opsional:

- `StreamEvents`
- `GetServerMetrics`
- `GetHealthStatus`

Proto ini tidak menggantikan `/metrics` Prometheus dan tidak wajib untuk daemon
berjalan. Endpoint harus memakai koneksi mTLS agent yang sudah ada.

## Data flow

```text
runtime/store/event bus
        |
        +--> metrics collector --> Prometheus HTTP handler
        |
        +--> health probes ------> /healthz
        |
        +--> event recorder -----> SQLite events --> realtime subscribers
        |
        +--> self-healer --------> lifecycle.RestartServer
```

Error dependency dicatat dengan structured logger, diberi severity yang sesuai,

## Testing

- Unit logger memastikan JSON fields dan redaction secret.
- Unit metrics memastikan nama, tipe, label, dan error runtime tidak memutus
  scrape.
- Unit health memastikan kombinasi probes menghasilkan status dan HTTP code tepat.
- Unit event history memastikan insert, query, metadata JSON, dan pruning.
- Unit restart policy memastikan backoff, max retry, crash-loop termination,
  dan stable reset.
- Component test memakai fake runtime + SQLite temp DB + event bus.
- HTTP integration menguji `/metrics` dan `/healthz` pada localhost.
- Docker kill/recovery test environment-gated; suite default tidak memerlukan
  Docker.

## Definition of Done

- Container yang crash dapat restart dengan backoff dan berhenti setelah batas.
- Stop manual tidak memicu self-healing.
- `/metrics` menghasilkan exposition format valid.
- `/healthz` akurat untuk dependency sehat/degraded/unhealthy.
- Log Fase 1-2 tidak lagi memakai log operasional unstructured.
- Event penting dapat di-query dan dipangkas sesuai retention.
- Tunnel recovery menggunakan policy bounded, bukan retry tanpa batas.

## Self-review

- Placeholder tidak ada; seluruh kontrak, default, state transition, schema,
  failure handling, dan lapisan test dipatok.
- Scope tidak mencakup dashboard, tracing, external alerting, atau fitur fase
  lain.
- Metrics collector dipisahkan dari request handler untuk mencegah scrape
  memicu Docker calls dan latency tidak terkendali.
- Event direkam sebelum broadcast agar history tidak kehilangan event yang telah
  diterima subscriber realtime.
