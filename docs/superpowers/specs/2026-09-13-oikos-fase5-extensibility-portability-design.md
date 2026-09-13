# Oikos Fase 5: Extensibility dan Portability Design

Tanggal: 2026-09-13  
Status: Disetujui dalam sesi brainstorming, menunggu review file

## Tujuan

Fase 5 membuat Oikos dapat diperluas melalui plugin proses terpisah, dapat
diperbarui melalui versioned binary slots tanpa mengganggu container, dan tidak
mengunci kode portable ke asumsi Linux.

## Scope MVP

- Plugin adalah binary terpisah yang berkomunikasi melalui gRPC Unix socket.
- Plugin hanya menerima event dan route capability yang dideklarasikan di
  manifest dan diizinkan host.
- Plugin crash diisolasi dari daemon utama dan dapat di-disable setelah retry
  terbatas.
- Endpoint plugin diproxy di namespace `/plugins/<id>/`.
- `oikos update` manual memakai versioned slots dan symlink `current` atomik.
- Candidate binary diverifikasi checksum/signature dan health-check sebelum
  switch.
- Rollback mengembalikan slot binary tervalidasi sebelumnya.
- Config/database dibackup sebelum update.
- `GOOS=linux`, `GOOS=darwin`, dan `GOOS=windows` compile; Linux tetap target
  production utama.
- `oikos doctor` menyatukan diagnostics dependency.

## Non-goals

- Go native plugin `.so`.
- Marketplace atau public plugin registry.
- Auto-update tanpa trigger admin.
- Live migration server antar-node.
- Full production support Windows/macOS.

## Plugin Architecture

Plugin binary berjalan sebagai subprocess dengan Unix socket lokal permission
`0600`. Host memiliki lifecycle: validate manifest, spawn, handshake Init,
subscribe event, health-check berkala, shutdown, dan crash isolation.

Kontrak logical plugin:

```go
type Event struct {
    Type     string
    ServerID string
    Metadata map[string]string
}

type Capability struct {
    Events []string
    Routes []string
}

type InitRequest struct {
    OikosVersion string
    ConfigPath   string
    PluginID     string
}

type InitResponse struct {
    Capabilities Capability
}

type PluginClient interface {
    Init(context.Context, InitRequest) (InitResponse, error)
    HandleEvent(context.Context, Event) error
    HealthCheck(context.Context) error
    Close() error
}
```

Manifest plugin:

```yaml
id: log-to-file
version: 1.0.0
binary: /var/lib/oikos/plugins/log-to-file
enabled: true
config: /etc/oikos/plugins/log-to-file.yaml
allowed_events:
  - server.crashed
allowed_routes: []
sha256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
```

Manifest id hanya boleh `[A-Za-z0-9_-]+`. Host menolak route global, route di
luar `/plugins/<id>/`, event yang tidak diizinkan, binary non-executable,
checksum mismatch, atau socket yang tidak dapat dibuat dengan mode aman.

## Plugin API Proxy

Plugin mengembalikan daftar route capability. Host hanya meneruskan request ke
route yang telah diizinkan pada namespace plugin. Plugin tidak mendapat akses
langsung ke SQLite, Docker socket, filesystem host, private key, atau event bus
internal. Event handler memiliki timeout dan error plugin tidak menghentikan
broadcast ke subscriber lain.

## Graceful Update

Update manager:

```go
type Release struct {
    Version string
    BinaryURL string
    SHA256 string
    Signature []byte
    GOOS string
    GOARCH string
}

type UpdateManager interface {
    Check(context.Context) (Release, error)
    Apply(context.Context, Release) error
    Rollback(context.Context) error
}
```

Layout update:

```text
/var/lib/oikos/versions/
  v0.1.0/oikos
  v0.2.0/oikos
  current -> v0.2.0
  previous -> v0.1.0
```

Alur `Apply`:

1. Backup `config.yaml`, SQLite database, dan manifest plugin ke folder
   timestamped dengan permission `0600`.
2. Download candidate ke temporary file.
3. Verifikasi SHA-256 dan signature.
4. Jalankan candidate dengan `--health-check-only` sebagai proses terpisah.
5. Jika health-check gagal, hapus candidate dan biarkan `current` unchanged.
6. Rename candidate ke slot versi immutable.
7. Ganti symlink `current` secara atomik dan simpan `previous`.
8. Daemon lama drain command baru; proses baru mengambil koneksi Panel dan
   monitoring.
9. Container Docker tidak direstart karena independen dari proses daemon.

`--health-check-only` validates Docker daemon connectivity with the Docker
client `Ping` API. It does not fabricate a container ID or call runtime
container metrics. The plugin manifest backup source is configured as
`plugins.manifest_path`; when present, the file is copied into the same
timestamped backup directory as config and SQLite.

Jika proses baru gagal readiness setelah switch, `Rollback` mengembalikan
symlink `current` ke slot previous. Backup database/config tetap tersedia dan
tidak dihapus otomatis.

## Portability

Interface portable berada di package umum. Implementasi Linux-specific seperti
cgroups, syscall process groups, SFTP permission, dan Unix sockets dipisah di
file build-tagged. Non-Linux menyediakan implementasi compile-safe yang
mengembalikan `ErrNotSupported` untuk fitur yang memang tidak tersedia.

Cross-build adalah validasi compile/API boundary, bukan klaim node production
fully supported di Windows/macOS.

## Doctor

`oikos doctor` menjalankan report actionable untuk:

- config dan permission file
- Docker connectivity
- cgroup availability
- Panel/mTLS connectivity
- filesystem roots
- backup directory
- plugin manifest/process/health
- current/previous binary slot

Exit code nonzero jika dependency wajib gagal; optional feature failure diberi
status warning dengan langkah perbaikan.

## SQLite

Migration `0005_plugins.sql`:

```sql
CREATE TABLE plugins (
    id               TEXT PRIMARY KEY,
    name             TEXT NOT NULL,
    version          TEXT NOT NULL,
    binary_path      TEXT NOT NULL,
    enabled          BOOLEAN NOT NULL DEFAULT 1,
    subscribed_events TEXT NOT NULL DEFAULT '[]',
    installed_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

## Error Handling dan Security

- Plugin process gagal tidak menjatuhkan daemon.
- Event plugin timeout dan tidak memblokir event bus utama.
- Health failure berulang men-disable plugin dan mencatat event.
- Update gagal sebelum switch tidak mengubah binary/config/state aktif.
- Update gagal sesudah switch memicu rollback ke slot tervalidasi.
- Checksum/signature wajib; checksum kosong ditolak.
- Secret tidak masuk manifest log, plugin event metadata, atau diagnostics.
- Unix socket dan backup memakai permission ketat.

## Testing

- Plugin fake: init, event subscription, capability filtering, health check,
  route namespace, timeout, shutdown, dan intentional crash isolation.
- Unix socket mode `0600`, cleanup, dan stale socket recovery.
- Update fake releases: checksum/signature success/failure, candidate health
  failure, atomic symlink switch, rollback, backup preservation, dan container
  ID/uptime unchanged.
- `GOOS=linux`, `GOOS=darwin`, `GOOS=windows` compile.
- Doctor matrix: dependency sehat, optional dependency gagal, mandatory
  dependency gagal, dan actionable output.
- Integration plugin process nyata dan update subprocess dijalankan dengan
  temporary directories; tidak bergantung systemd.

## Definition of Done

- Plugin contoh dapat di-load dan bereaksi pada `server.crashed` tanpa perubahan
  core.
- Plugin crash tidak menjatuhkan daemon.
- Update candidate sehat berpindah slot tanpa restart container.
- Update candidate gagal rollback tanpa kehilangan config/state.
- Cross-build tiga OS compile sukses.
- `oikos doctor` melaporkan dependency bermasalah secara actionable.

## Self-review

- Tidak ada keputusan arsitektur terbuka; transport, capability, update slots,
  rollback, portability, dan doctor sudah eksplisit.
- Plugin API tidak memberi akses langsung ke internal state.
- Namespace proxy mencegah plugin mengambil route global.
- Versioned slots menjaga binary lama untuk rollback dan tidak mengganggu Docker.
- Unsupported platform dibedakan dari unsupported compile agar ekspektasi tidak
  salah.
