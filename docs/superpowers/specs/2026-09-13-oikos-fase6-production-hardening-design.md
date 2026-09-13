# Oikos Fase 6: Production Hardening Design

Tanggal: 2026-09-13  
Status: Disetujui untuk implementasi bertahap

## Tujuan

Fase 6 mengeraskan dan memvalidasi fitur Fase 1-5 tanpa menambah kapabilitas
produk baru. Fokusnya keamanan runtime, dependency audit, benchmark, chaos
testing, dokumentasi operasional, dan release readiness.

## Scope dan Urutan

1. Static security analysis: `gosec` dan `govulncheck`, termasuk CI, dengan
   semua temuan high ditangani atau dibuktikan false positive.
2. Audit filesystem path resolution dan permission boundary dengan test
   traversal, symlink, prefix collision, dan TOCTOU-oriented scenarios.
3. Seccomp profile default untuk container Docker, dengan profile file yang
   versioned dan fallback/warning jelas.
4. Evaluasi AppArmor/SELinux dan fallback bila runtime/platform tidak mendukung.
5. User namespace remapping sebagai konfigurasi Docker yang explicit dan
   capability check, tanpa mengklaim aktif bila daemon tidak mengizinkan.
6. Benchmark/load-test harness dan laporan. Benchmark head-to-head Wings,
   puluhan/ratusan server, dan penetration test nyata berstatus **pending manual
   verification** bila membutuhkan VM/host production.
7. Dokumentasi production dan release checklist diperbarui inkremental sesuai
   behavior yang sudah diverifikasi.

## Non-goals

- Fitur produk baru atau perubahan kontrak API.
- Sertifikasi formal SOC2/ISO.
- Klaim benchmark yang belum diukur.
- Menjalankan systemd/penetration test production di sandbox terbatas.

## Static Analysis

CI menjalankan:

```bash
gosec ./...
govulncheck ./...
```

Tool versions dipatok pada CI atau tool manifest. Output disimpan sebagai
artifact. Temuan high wajib memiliki fix, suppression yang beralasan dengan
komentar, atau dokumentasi false-positive yang direview. Secret tidak boleh
masuk scanner artifact.

## Filesystem Security Audit

`internal/filesystem.Manager.ResolvePath` tetap menjadi single boundary. Test
wajib mencakup:

- `../../etc/passwd`
- absolute path
- backslash separator
- prefix collision seperti sibling `server-a` vs `server-ab`
- symlink file
- symlink parent
- rename/copy destination escape
- archive restore traversal/symlink entries
- cancellation selama streaming

Jika full `openat2(RESOLVE_BENEATH|RESOLVE_NO_SYMLINKS)` tidak tersedia, behavior
fallback yang sudah ada harus menolak path dan mencatat limitation; tidak boleh
diam-diam mengizinkan escape.

## Container Sandboxing

### Seccomp

Docker `ContainerSpec` mendapat sandbox options dari security layer. Profile
default disimpan sebagai versioned JSON di `deploy/docker/seccomp-default.json`
dan dipasang melalui Docker `SecurityOpt`. Profile harus diuji dengan container
smoke command yang memakai syscall normal. Jika profile tidak tersedia, Oikos
mengembalikan warning/error yang jelas sesuai configured enforcement mode.

Profile ilustratif tidak dianggap production-valid tanpa trace syscall. Release
documentation menyatakan profile compatibility policy per egg.

### AppArmor/SELinux

Startup probe mendeteksi `/sys/kernel/security/apparmor` atau SELinux status.
Oikos hanya mengaktifkan profile bila profile yang dipilih benar-benar tersedia;
jika tidak, health/doctor melaporkan `unsupported` atau `degraded`, bukan
memberi kesan sandbox aktif. Platform non-Linux mengembalikan `ErrNotSupported`.

### User namespace

Runtime security config menyediakan `UserNamespaceMode` dan `UserRemap` flag.
Docker create memakai `UsernsMode` hanya bila daemon capability check lulus.
Default production profile menolak fallback diam-diam ke root host; mode
permissive harus explicit dan menghasilkan warning.

## Benchmark dan Chaos

Repository menyimpan harness reproducible:

```text
docs/benchmarks/
  README.md
  idle-resource-usage.md
  oikos-vs-wings-comparison.md
  load-test-100-servers.md
scripts/benchmarks/
  measure-daemon.sh
  load-servers.sh
```

Setiap report mencatat host/kernel, binary commit, Go version, Docker version,
jumlah server, egg, sample interval, RSS/CPU method, dan raw output path. Data
Wings, load 10/50/100 server, dan penetration test diberi status **pending
manual verification** sampai dijalankan pada environment yang sesuai.

Chaos scenarios:

- Docker daemon unavailable.
- Disk/full or unwritable data directory.
- Panel unreachable.
- SQLite locked.

Setiap scenario harus menghasilkan exit/status/log yang predictable dan tidak
silent.

## Production Documentation

Dokumen berikut dibuat atau diperbarui berdasarkan implementasi aktual:

- `docs/architecture.md`
- `docs/installation.md`
- `docs/operations.md`
- `docs/disaster-recovery.md`
- `docs/security.md`
- `docs/egg-spec.md`

Dokumen tidak boleh menyebut fitur aktif jika test/doctor hanya menunjukkan
unsupported atau pending manual verification.

## Release Engineering

Release checklist menyiapkan:

- Semantic versioning.
- Keep a Changelog.
- Reproducible CI build.
- SHA-256 dan signature verification.
- Support/rollback policy.
- VM clean-install verification.

Rilis `v1.0.0`, signing key production, dan clean VM verification berstatus
pending manual verification bila credential/infrastructure tidak tersedia.

## Definition of Done Fase 6

- `gosec` dan `govulncheck` terintegrasi CI dengan high findings resolved.
- Filesystem traversal/symlink audit tests lulus.
- Seccomp default dan fallback status diuji.
- AppArmor/SELinux/user namespace support melaporkan capability secara akurat.
- Benchmark/chaos harness tersedia dengan metodologi dan status data jelas.
- Production docs konsisten dengan implementasi.
- Release checklist dan changelog tersedia.
- Semua production-only evidence ditandai pending manual verification, bukan
  diklaim lulus tanpa data.

## Self-review

- Scope murni hardening/validasi; tidak ada API produk baru.
- Urutan task memulai dari static analysis dan filesystem audit yang dapat diuji
  otomatis.
- Sandbox limitations ditangani melalui fallback eksplisit dan status pending,
  bukan simulasi yang disamakan dengan production evidence.
