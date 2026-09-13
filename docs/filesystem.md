# Filesystem dan Backup Fase 4

## Layout

- Data server: `<node.data_dir>/servers/<server_id>/data`.
- Backup: `<node.data_dir>/backups/<server_id>/`.

Semua operasi file melewati `ResolvePath`. API dan SFTP tidak menerima path host
absolut dari caller.

## Security

Path absolute, `..` yang keluar root, backslash separator, dan symlink pada file
atau parent ditolak. Server ID dibatasi karakter alphanumeric, `_`, dan `-`.
SFTP menggunakan credential public key yang terikat ke server dan backend manager
yang sama dengan API.

## Streaming API

Upload dan download memakai chunk maksimal sesuai `filesystem.upload_chunk_bytes`
(default 1 MiB). Upload ditulis ke temporary file lalu di-rename setelah stream
berhasil. File besar tidak dikumpulkan seluruhnya di memory daemon.

## Backup

Backup default bersifat hot dan tidak menghentikan container. Data dibaca dari
host filesystem, dikompresi sebagai `tar.gz`, dan checksum SHA-256 disimpan pada
SQLite. Hot backup bersifat best effort untuk file yang sedang ditulis; gunakan
opsi stop eksplisit untuk aplikasi yang memerlukan konsistensi penuh.

## Restore

Restore ke server existing hanya boleh saat status `stopped`. Checksum diverifikasi
sebelum ekstraksi. Archive diekstrak ke staging directory, absolute/traversal/
symlink entries ditolak, lalu staging di-rename ke data directory. Directory lama
dipertahankan sementara sebagai safety net sampai replacement berhasil.

## SFTP

SFTP disabled secara default dan bind localhost pada `127.0.0.1:2222` ketika
diaktifkan. Host key disimpan pada path konfigurasi dengan permission ketat.
Tidak ada akses shell host dan tidak ada akses lintas server.

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

Gunakan `oikos diagnose` untuk memeriksa data directory, SQLite, dan dependency
runtime sebelum mengaktifkan akses file.
