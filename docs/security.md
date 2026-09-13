# Security Model

## Assets and Controls

| Asset | Threat | Control |
|---|---|---|
| mTLS cert/key | Node impersonation | Key/cert mode 0600, CA validation, rotation policy |
| Pairing token | Token theft/reuse | One-time validation and short expiry at Panel |
| Server files | Traversal/symlink escape | Central `ResolvePath`, Lstat component checks, jailed SFTP |
| Docker host | Container escape | Seccomp, optional LSM profile, user namespace, Docker least privilege |
| Plugin process | Host state access/crash | Unix socket 0600, capability allowlist, subprocess isolation |
| Update binary | Tampering/rollback loss | SHA-256, signature, version slots, state manifest, backup |

Seccomp/AppArmor/SELinux/user namespace status harus dilaporkan doctor secara
akurat. Unsupported host memakai fallback eksplisit, bukan klaim sandbox aktif.

Gosec high gate tidak menemukan high issue pada audit ini. Govulncheck masih
melaporkan dependency Docker/Moby tanpa upstream fixed version; hal tersebut
tetap dicatat sebagai residual risk dan tidak disuppress diam-diam.

Penetration test API gRPC/SFTP dan review pihak kedua: **pending manual verification**.

## Govulncheck Review

Review terakhir: 2026-09-13. Govulncheck dijalankan dalam dua mode:

- `govulncheck -json ./...` untuk ID/module/version.
- `govulncheck -show verbose ./...` untuk example trace.

Govulncheck mengklasifikasikan package/module trace, sehingga trace ke package
Docker tidak selalu berarti fungsi vulnerable spesifik dieksekusi. Klasifikasi
di bawah membandingkan trace tersebut dengan call site Oikos dan kondisi
eksploitasi advisory.

CI memakai policy allowlist, bukan men-disable govulncheck. Exit code `3` hanya
diterima bila semua **symbol-reachable** ID yang muncul adalah tiga ID di tabel
berikut. JSON govulncheck juga melaporkan module-only IDs dengan trace satu
elemen; itu tetap disimpan sebagai artifact dan dipantau, tetapi tidak dianggap
fungsi vulnerable yang reachable. ID symbol-reachable baru atau scanner error
tetap menggagalkan pipeline dan harus direview sebelum merge.

| ID / CVE | Severity | Package dan versi terpengaruh | Apakah code path vulnerable dipanggil Oikos? | Status | Alasan dan mitigasi | Review berikutnya |
|---|---|---|---|---|---|---|
| `GO-2026-6253` / `CVE-2026-17106` / `GHSA-hfg8-hc9c-6c3h` | High, CVSS 7.1 | `github.com/moby/go-archive v0.1.0`, fixed upstream `v0.3.0` | **Tidak untuk fungsi vulnerable.** Oikos memanggil `archive.TarWithOptions` untuk membuat build context dan `tar.FileInfoHeader` untuk backup; Oikos tidak memanggil fungsi vulnerable `Unpack`, `UnpackLayer`, `Untar`, `ApplyLayer`, atau Docker archive extraction. | `accepted-risk-unreachable` | Advisory adalah crafted-archive extraction escape. Oikos melakukan extraction restore dengan stdlib `archive/tar`, validasi relative path, menolak `..`, absolute path, hard link, dan symlink. Docker SDK archive API tidak dipakai untuk extraction. Tetap monitor versi `go-archive` dan evaluasi upgrade SDK. | 2026-10-13 |
| `GO-2026-4887` / `CVE-2026-34040` / `GHSA-x744-4wpc-v9h2` | High, CVSS 8.8 | Docker Engine / `github.com/docker/docker v28.5.2+incompatible`; advisory fixed Engine `29.3.1` | **Tidak pada kondisi vulnerable.** Oikos memakai Docker SDK untuk `Ping`, container lifecycle, stats, logs, exec, dan image build; Oikos tidak memasang/menggunakan AuthZ plugin dan tidak meneruskan request body arbitrer ke Docker API. | `accepted-risk-unreachable` | Advisory hanya berdampak bila Docker AuthZ plugin menginspeksi request body dan menerima oversized body yang dimanipulasi. Oikos tidak mengaktifkan AuthZ plugin. Host ini memakai Docker Engine `29.1.3`, belum `29.3.1`; upgrade Engine ke `29.3.1+` wajib untuk deployment yang menggunakan AuthZ plugin. Akses Docker socket tetap dibatasi ke user daemon/trusted operators. | 2026-10-13 |
| `GO-2026-4883` / `CVE-2026-33997` / `GHSA-pxq6-2prw-chj9` | Moderate, CVSS 6.8 | Docker Engine / `github.com/docker/docker v28.5.2+incompatible`; advisory fixed Engine `29.3.1` | **Tidak.** Oikos plugin adalah subprocess gRPC sendiri; Oikos tidak memanggil Docker Plugin API atau `docker plugin install`, yaitu flow yang memiliki privilege validation vulnerable. | `accepted-risk-unreachable` | Advisory membutuhkan user memasang malicious Docker plugin. Oikos tidak memasang Docker plugins dan memakai plugin capability model sendiri dengan Unix socket 0600 serta allowlist. Jangan memasang Docker plugin tidak tepercaya pada host node. Upgrade Engine ke `29.3.1+` saat tersedia di deployment. | 2026-10-13 |

Govulncheck juga melaporkan module-only IDs berikut pada run yang sama:

| ID | Module/version | Status | Alasan |
|---|---|---|---|
| `GO-2026-5617` / `CVE-2026-42306` / `GHSA-rg2x-37c3-w2rh` | `github.com/docker/docker v28.5.2+incompatible` | `accepted-risk-unreachable` | Hanya module trace; Oikos tidak memanggil Docker `cp` archive path yang menjadi advisory. |
| `GO-2026-5668` / `CVE-2026-41568` / `GHSA-vp62-88p7-qqf5` | `github.com/docker/docker v28.5.2+incompatible` | `accepted-risk-unreachable` | Hanya module trace; Oikos tidak memanggil Docker `cp` archive path yang menjadi advisory. |
| `GO-2026-5746` / `CVE-2026-41567` / `GHSA-x86f-5xw2-fm2r` | `github.com/docker/docker v28.5.2+incompatible` | `accepted-risk-unreachable` | Hanya module trace; Oikos tidak memanggil `PUT /containers/{id}/archive`. |
| `GO-2026-5932` | `golang.org/x/crypto v0.57.0` | `accepted-risk-unreachable` | Module mengandung `openpgp`, tetapi Oikos memakai `x/crypto/ssh`, bukan package `openpgp`. |

### Audit Interpretation

Temuan `GO-2026-4887` dan `GO-2026-4883` mempunyai trace package-level ke
`github.com/docker/docker`, tetapi itu bukan bukti bahwa AuthZ-plugin atau
Docker-plugin-install path dipanggil. Temuan `GO-2026-6253` mempunyai trace ke
fungsi archive yang digunakan untuk membuat archive/context; vulnerable behavior
berada pada extraction routines yang tidak digunakan Oikos.

Govulncheck tidak menyediakan severity CVSS dalam output JSON; severity di tabel
diambil dari advisory upstream yang direferensikan oleh Go Vulnerability
Database. Ketiga advisory berstatus unreviewed pada database Go saat audit.

Tidak ada finding yang diklasifikasikan `accepted-risk-monitored` tanpa alasan:
ketiga kondisi vulnerable spesifik tidak dipanggil Oikos. Namun dependency
Docker/Moby dan Docker Engine tetap harus direview ulang pada 2026-10-13 atau
lebih cepat bila upstream merilis patch baru. Penetration test Docker/AuthZ dan
validasi upgrade Engine production tetap **pending manual verification**.

Review ulang wajib dilakukan pada **2026-10-13**. Tambahkan tanggal ini ke
calendar/issue tracker dengan owner yang jelas; dokumentasi saja bukan reminder
yang dapat diandalkan untuk operasi jangka panjang.
