# Oikos Fase 6: Production Hardening Design

Tanggal: 2026-09-13  
Status: Disetujui untuk desain, menunggu review file

## Tujuan

Fase 6 hanya mengeraskan dan memvalidasi fitur Fase 1-5. Tidak ada kapabilitas
produk baru. Fokusnya audit keamanan, sandboxing container, validasi performa,
chaos testing, dokumentasi production, dan release engineering.

## Urutan Pengerjaan

1. Static security analysis dan dependency vulnerability audit.
2. Audit ulang filesystem path resolution dengan test serangan konkret.
3. Seccomp profile default untuk Docker container.
4. Evaluasi AppArmor/SELinux dengan fallback eksplisit.
5. User namespace remapping dan capability detection.
6. Benchmark/load-test/penetration-test harness dan laporan.
7. Dokumentasi production dan release checklist.

Setiap item dikerjakan terpisah dan di-commit setelah verification hijau.
## Non-goals

- Fitur produk baru atau perubahan kontrak Fase 1-5.
- Sertifikasi formal keamanan.
- Klaim benchmark yang belum diukur.
- Menjalankan handoff systemd, penetration test production, atau benchmark
  head-to-head Wings di sandbox terbatas.

## Static Security Analysis

CI menjalankan `gosec ./...` dan `govulncheck ./...`. Tool versions dicatat di
`tools.go` atau workflow. Temuan high harus diperbaiki, atau memiliki alasan
false-positive yang terdokumentasi. Dependency audit tidak boleh memasukkan
secret atau environment credentials ke artifact.

## Filesystem Security Audit

`filesystem.Manager.ResolvePath` tetap menjadi single security boundary. Test
wajib mencakup:

- relative traversal `../../etc/passwd`
- absolute path
- backslash separator
- sibling-prefix collision (`server-a` vs `server-ab`)
- symlink file
- symlink parent
- rename/copy destination escape
- archive restore traversal/symlink entries
- cancellation selama streaming

Fallback lexical/Lstat harus tetap deny-by-default bila `openat2` tidak tersedia.
Test race/TOCTOU mendokumentasikan batasan residual dan memastikan symlink tidak
pernah diikuti.

## Container Sandboxing

### Seccomp

Profile versioned disimpan di `deploy/docker/seccomp-default.json`. Docker
`ContainerSpec` menerima security options dari security layer dan menerapkan
profile melalui `SecurityOpt`. Smoke test memakai syscall normal dan memastikan
container tetap berjalan. Profile unavailable menghasilkan warning/error sesuai
mode enforcement, tidak diam-diam dianggap aktif.

### AppArmor dan SELinux

Startup capability probe mendeteksi AppArmor melalui securityfs/parser dan
SELinux melalui `/sys/fs/selinux`/`getenforce` bila tersedia. Profile hanya
diaktifkan jika platform dan profile benar-benar ada. Unsupported platform atau
host tanpa subsystem menghasilkan status `unsupported`/`degraded` yang jelas.

### User namespace

Runtime security options menyediakan mode user namespace/remapping yang explicit.
Docker create memakai option tersebut hanya setelah capability check. Fallback
ke root host harus explicit permissive mode dan menghasilkan warning.

## Benchmark dan Chaos Harness

Repository menyimpan:

```text
docs/benchmarks/
  README.md
  oikos-vs-wings-comparison.md
  load-test-100-servers.md
scripts/benchmarks/
  measure-daemon.sh
  load-servers.sh
```

Report wajib mencatat host, kernel, commit, Go/Docker versions, jumlah server,
egg, sample interval, RSS/CPU method, dan raw output. Idle benchmark existing
dapat direferensikan. Perbandingan Wings, load puluhan/ratusan server, dan
penetration test nyata berstatus **pending manual verification** sampai ada
VM/environment yang sesuai.

Chaos scenarios meliputi Docker unavailable, disk unwritable/full, Panel
unreachable, dan SQLite locked. Harness hanya menguji behavior predictable dan
log/event; tidak mengklaim failure injection production tanpa environment nyata.

## Production Documentation

Dokumen berikut harus konsisten dengan implementation aktual:

- `docs/architecture.md`
- `docs/installation.md`
- `docs/operations.md`
- `docs/disaster-recovery.md`
- `docs/security.md`
- `docs/egg-spec.md`

Dokumentasi menyebut batasan Linux-only, frp optional, pending manual
verification, backup/restore, plugin/update, sandbox fallback, dan rollback.

## Release Engineering

Siapkan `CHANGELOG.md` Keep a Changelog, semantic-versioning policy, reproducible
build/checksum/signature workflow, support policy, dan production release
checklist. `v1.0.0`, production signing key, clean VM installation, serta
systemd handoff tetap **pending manual verification** bila environment/credential
nyata belum tersedia.

## Definition of Done

- `gosec`/`govulncheck` berjalan di CI dan tidak ada high finding unresolved.
- Filesystem attack tests lulus.
- Seccomp default serta fallback status teruji.
- AppArmor/SELinux/user namespace melaporkan capability secara akurat.
- Benchmark, load-test, penetration-test, dan chaos harness tersedia dengan
  metodologi serta status hasil yang jujur.
- Dokumentasi production konsisten.
- Changelog, release checklist, checksum/signature process tersedia.
- Semua validasi yang membutuhkan environment nyata diberi status pending manual,
  bukan hasil simulasi yang diklaim sebagai production evidence.

## Self-review

- Scope hanya hardening dan validasi; tidak ada API produk baru.
- Urutan dimulai dari static analysis dan filesystem audit yang otomatis.
- Fallback security selalu observable.
- Production-only claims dipisahkan dari sandbox evidence.
