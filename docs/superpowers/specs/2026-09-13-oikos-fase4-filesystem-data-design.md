# Oikos Fase 4: Filesystem dan Data Design

Tanggal: 2026-09-13  
Status: Disetujui dalam sesi brainstorming, menunggu review file

## Tujuan

Fase 4 memberi akses file server yang aman melalui API gRPC dan SFTP minimal,
serta menyediakan backup/restore yang streaming, terisolasi per server, dan
memvalidasi integritas arsip.

## Keputusan Scope

- Isolasi utama: root directory per server dengan path resolution terpusat,
  pemeriksaan boundary, dan nofollow terhadap symlink escape.
- Format backup MVP: `tar.gz` streaming memakai standard library.
- API file manager dan backup/restore adalah fitur inti.
- SFTP minimal diimplementasikan setelah manager aman; SFTP memakai manager yang
  sama sebagai backend, bukan logic filesystem duplikat.
- Server data: `<data_dir>/servers/<server_id>/data`.
- Backup data: `<data_dir>/backups/<server_id>/`.
- Restore memakai staging directory pada filesystem yang sama untuk atomic rename.

## Non-goals

- UID/GID host unik per server pada MVP.
- Chroot atau namespace filesystem penuh.
- Backup cloud/S3 dan scheduler backup otomatis.
- Version control file.
- Snapshot Btrfs/ZFS/LVM; hot backup bersifat best effort.
- SFTP fitur lanjutan di luar list/read/write/list directory dasar.

## Filesystem Manager

Kontrak tunggal yang dipakai API dan SFTP:

```go
type FileInfo struct {
    Name       string
    Path       string
    IsDir      bool
    SizeBytes  int64
    ModifiedAt time.Time
    Mode       string
}

type Manager interface {
    List(ctx context.Context, serverID, relPath string) ([]FileInfo, error)
    Read(ctx context.Context, serverID, relPath string) (io.ReadCloser, error)
    Write(ctx context.Context, serverID, relPath string, content io.Reader) error
    Delete(ctx context.Context, serverID, relPath string, recursive bool) error
    Rename(ctx context.Context, serverID, oldPath, newPath string) error
    Copy(ctx context.Context, serverID, srcPath, dstPath string) error
    CreateDir(ctx context.Context, serverID, relPath string) error
    ResolvePath(serverID, relPath string) (string, error)
}
```

`ResolvePath` adalah satu-satunya path resolver. Ia menolak server ID kosong,
absolute path, `..` yang keluar root, encoded separator setelah decoding,
symlink file, dan symlink parent. Root harus dibuat dengan mode directory aman.
Operasi tidak pernah menerima path host dari caller.

Error sentinel:

- `ErrPathOutsideRoot`
- `ErrNotFound`
- `ErrPermission`
- `ErrServerNotFound`
- `ErrQuotaExceeded`

Implementasi menggunakan `filepath.Clean` + boundary check dan pemeriksaan
`Lstat` setiap komponen path sebelum operasi. Race/TOCTOU yang tersisa dibatasi
dengan menolak symlink dan membuka file melalui path yang sudah diverifikasi;
hardening syscall penuh tetap dapat diperluas di fase production.

Read/Write menerima stream. File besar tidak boleh dikumpulkan menjadi `[]byte`
di daemon.

## File API

`proto/filesystem/filesystem.proto` menyediakan:

- unary `List`
- unary `Delete`, `Rename`, `CreateDir`
- server-streaming `Download`
- client-streaming `Upload`
- unary `Copy`

Upload chunk pertama membawa server ID dan relative path; chunk berikutnya hanya
membawa bytes. Server membatasi chunk size, memeriksa context, menulis ke
temporary file, lalu rename setelah stream berhasil. Upload gagal menghapus
temporary file. Download membaca file dari manager dan mengirim chunk terukur.

## Backup

Kontrak:

```go
type BackupOptions struct {
    ServerID      string
    StopBeforeRun bool
}

type BackupResult struct {
    BackupID    string
    SizeBytes   int64
    ChecksumSHA string
}

type RestoreOptions struct {
    ServerID       string
    BackupID       string
    TargetServerID string
}

type Manager interface {
    Create(context.Context, BackupOptions) (BackupResult, error)
    Restore(context.Context, RestoreOptions) error
    List(context.Context, string) ([]BackupResult, error)
    Delete(context.Context, string) error
}
```

Create membaca data server langsung dari host filesystem sambil container dapat
tetap berjalan bila `StopBeforeRun=false`. Arsip ditulis sebagai tar.gz melalui
stream dan checksum SHA-256 dihitung terhadap bytes arsip final. Metadata backup
disimpan di SQLite dengan status `creating`, `completed`, atau `failed`.

Archive entries wajib relative, tidak mengandung `..`, dan symlink ditolak pada
MVP. Restore memvalidasi checksum sebelum ekstraksi, mengekstrak ke staging
directory, menolak entry berbahaya, lalu mengganti data directory secara aman.
Restore ke server existing hanya boleh saat status server `stopped`.

Hot backup adalah best effort: file yang sedang ditulis dapat tidak atomik.
`StopBeforeRun=true` tersedia untuk aplikasi sensitif seperti database.

## SFTP Minimal

SFTP memakai `golang.org/x/crypto/ssh` dan `github.com/pkg/sftp`. Authentication
berbasis public key yang tersimpan pada tabel `sftp_credentials`, bukan user OS.
Setiap credential terikat ke server ID dan session di-jail ke root server.
Backend operasi memanggil filesystem manager yang sama.

SFTP bind localhost secara default, port terpisah, dan tidak aktif bila belum
ada credential. Private key host dan credential metadata memakai permission
ketat; password plaintext tidak pernah disimpan.

## SQLite Schema

Migration `0004_filesystem_backup.sql` menambah:

```sql
CREATE TABLE backups (
    id              TEXT PRIMARY KEY,
    server_id       TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    file_path       TEXT NOT NULL,
    size_bytes      INTEGER NOT NULL,
    checksum_sha256 TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'creating',
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at    DATETIME
);

CREATE TABLE sftp_credentials (
    id              TEXT PRIMARY KEY,
    server_id       TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    username        TEXT NOT NULL,
    public_key      TEXT,
    password_hash   TEXT,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_backups_server_id ON backups(server_id);
```

## Configuration

```yaml
filesystem:
  server_root: "/var/lib/oikos/servers"
  backup_root: "/var/lib/oikos/backups"
  upload_chunk_bytes: 1048576
  max_upload_bytes: 10737418240

sftp:
  enabled: false
  bind_addr: "127.0.0.1:2222"
  host_key_path: "/etc/oikos/certs/sftp_host.key"
```

Disk quota check dilakukan sebelum dan selama upload bila limit tersedia dari
resource layer. Default SFTP disabled mencegah listener terbuka tanpa setup
credential.

## Error Handling

- Path invalid dikembalikan sebagai error terklasifikasi tanpa membocorkan path
  host absolut.
- Upload/backup/restore gagal membersihkan staging/temp artifact.
- Restore tidak menyentuh data existing sebelum checksum dan extraction staging
  berhasil.
- Backup hot mencatat status dan error metadata; daemon tidak crash.
- Context cancellation menghentikan stream dan menghapus temp file.

## Testing

- Unit path traversal: `../../etc/passwd`, absolute path, encoded separator,
  dot path, symlink file, symlink parent, dan boundary prefix collision.
- Unit manager: list/read/write/delete/rename/copy/create dir, permission,
  server isolation, context cancellation.
- Unit streaming: file besar diproses melalui `io.Reader`, bukan whole-buffer.
- Unit archive: tar.gz valid, checksum, corrupt archive, absolute/traversal
  entry, symlink entry, staging cleanup.
- Component gRPC bufconn: list, upload, download, delete, rename, dan chunks.
- SFTP minimal: public-key authentication, jail root, dan backend manager.
- Integration opsional: file 1GB+, hot backup container nyata, restore stopped,
  dan reject restore saat running.

## Definition of Done

- File API dapat mengelola data dalam root server tanpa path escape.
- Upload/download streaming tidak memuat file besar seluruhnya ke RAM.
- SFTP client standar dapat mengakses satu server dan tidak dapat keluar root.
- Backup hot menghasilkan arsip valid dan checksum cocok.
- Restore running ditolak; restore stopped mengembalikan data utuh.
- Upload melebihi quota ditolak.
- Unit/component tests lulus; integration test nyata dicatat PASS atau SKIP
  dengan alasan environment yang jelas.

## Self-review

- Tidak ada placeholder atau keputusan terbuka.
- `ResolvePath` menjadi boundary tunggal bagi API dan SFTP.
- Restore staging mencegah data existing rusak akibat archive corrupt.
- SFTP minimal tetap berada di belakang manager yang sama, sehingga aturan
  traversal tidak dapat berbeda antara API dan SFTP.
- Hot backup limitation didokumentasikan dan opsi stop eksplisit disediakan.
