# Oikos Fase 4 Filesystem and Data Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Membangun filesystem manager terisolasi, API streaming, backup/restore tar.gz, dan SFTP minimal yang aman per server.

**Architecture:** `internal/filesystem.Manager` menjadi satu backend untuk gRPC dan SFTP. Seluruh operasi melewati `ResolvePath`, menolak traversal dan symlink escape, serta memakai streaming reader/writer. Backup menulis arsip ke staging dan menghitung SHA-256; restore memvalidasi lalu mengganti data directory secara atomik.

**Tech Stack:** Go 1.26, standard library `os/path/filepath/archive/tar/compress/gzip/crypto/sha256`, gRPC/protobuf existing, `github.com/pkg/sftp`, `golang.org/x/crypto/ssh`, SQLite `modernc.org/sqlite`.

**Spec:** `docs/superpowers/specs/2026-09-13-oikos-fase4-filesystem-data-design.md`

## Global Constraints

- Module Go: `github.com/oikos/oikos`.
- Go minimum: 1.23.
- SQLite driver: `modernc.org/sqlite` (tanpa cgo).
- Server data: `<data_dir>/servers/<server_id>/data`.
- Backup data: `<data_dir>/backups/<server_id>/`.
- Format backup MVP: `tar.gz` streaming.
- `ResolvePath` adalah satu-satunya path resolver untuk API dan SFTP.
- Absolute path, `..` escape, encoded separator, symlink file, dan symlink parent wajib ditolak.
- Restore ke server existing hanya boleh saat status `stopped`.
- Archive entry absolute, traversal, dan symlink ditolak.
- Upload/download tidak boleh memuat seluruh file ke memory.
- SFTP disabled by default dan bind localhost.

---

## File Map

- Modify `internal/config/config.go`, `defaults.go`, `config.yaml`: filesystem/SFTP config.
- Create `internal/filesystem/errors.go`: sentinel errors.
- Create `internal/filesystem/manager.go`: root isolation dan operations.
- Create `internal/filesystem/manager_test.go`: traversal/symlink/isolation tests.
- Create `internal/filesystem/stream_test.go`: large streaming behavior.
- Create `migrations/0004_filesystem_backup.sql`: backup/SFTP metadata.
- Modify `internal/store/models.go`, `sqlite.go`: backup/credential CRUD.
- Create `internal/backup/archive.go`, `manager.go`, `restore.go`: backup pipeline.
- Create `internal/backup/manager_test.go`: checksum/staging/restore tests.
- Create `proto/filesystem/filesystem.proto`: file RPCs.
- Generate `gen/go/filesystem/`; modify `scripts/gen-proto.sh`.
- Create `internal/api/filesystem.go`, `filesystem_test.go`: gRPC file adapter.
- Create `internal/filesystem/sftp.go`, `sftp_test.go`: minimal jailed SFTP.
- Modify `cmd/oikos/daemon.go`, `internal/agent/daemon.go`: filesystem/backup services.
- Create `docs/filesystem.md`: operations, security, backup limitations.

---

### Task 1: Config filesystem/SFTP dan migration SQLite

**Files:**
- Create: `internal/config/filesystem_test.go`, `migrations/0004_filesystem_backup.sql`
- Modify: `internal/config/config.go`, `internal/config/defaults.go`, `config.yaml`, `internal/store/models.go`, `internal/store/sqlite.go`
- Test: `internal/config/filesystem_test.go`, `internal/store/backup_test.go`

**Interfaces:**
- Produces: `config.FilesystemConfig`, `config.SFTPConfig`; `store.Backup`, `store.SFTPCredential`; `MigrateFilesystem() error`; backup/credential CRUD.

- [ ] **Step 1: Write failing config test**

```go
func TestFilesystemDefaults(t *testing.T) {
    cfg := Default()
    if cfg.Filesystem.ServerRoot != "/var/lib/oikos/servers" { t.Fatal(cfg.Filesystem.ServerRoot) }
    if cfg.Filesystem.BackupRoot != "/var/lib/oikos/backups" { t.Fatal(cfg.Filesystem.BackupRoot) }
    if cfg.SFTP.Enabled { t.Fatal("SFTP default harus disabled") }
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/config/ -run TestFilesystemDefaults -v`
Expected: FAIL karena config fields belum ada.

- [ ] **Step 3: Implement config and migration**

Add `FilesystemConfig{ServerRoot, BackupRoot string; UploadChunkBytes, MaxUploadBytes int64}` and `SFTPConfig{Enabled bool; BindAddr, HostKeyPath string}`. Defaults: `/var/lib/oikos/servers`, `/var/lib/oikos/backups`, `1048576`, `10737418240`, `false`, `127.0.0.1:2222`, `/etc/oikos/certs/sftp_host.key`. Add YAML block. Migration uses `CREATE TABLE IF NOT EXISTS backups`, `sftp_credentials`, indexes, foreign keys, and `MigrateFilesystem()` called idempotently by `store.Migrate()`.

- [ ] **Step 4: Implement store CRUD and test**

`Backup` fields: `ID, ServerID, FilePath, ChecksumSHA, Status string; SizeBytes int64; CreatedAt, CompletedAt time.Time`. `SFTPCredential` fields: `ID, ServerID, Username, PublicKey, PasswordHash string`. Implement `SaveBackup`, `GetBackup`, `ListBackups`, `UpdateBackupStatus`, `DeleteBackup`, `SaveSFTPCredential`, `GetSFTPCredential`.

- [ ] **Step 5: Verify and commit**

Run: `go test ./internal/config/ ./internal/store/ -v`
Expected: PASS, termasuk migration dipanggil dua kali.

```bash
```

---

### Task 2: Filesystem manager root isolation

**Files:**
- Create: `internal/filesystem/errors.go`, `internal/filesystem/manager.go`, `internal/filesystem/manager_test.go`, `internal/filesystem/stream_test.go`

**Interfaces:**
- Produces: `Manager` dengan `List`, `Read`, `Write`, `Delete`, `Rename`, `Copy`, `CreateDir`, `ResolvePath`; `NewManager(serverRoot string) Manager`.

- [ ] **Step 1: Write failing security tests**

```go
func TestResolvePathRejectsEscapeAndSymlink(t *testing.T) {
    root := t.TempDir(); m := NewManager(root); os.MkdirAll(filepath.Join(root,"srv-1","data"),0750)
    for _, path := range []string{"../../etc/passwd", "/etc/passwd", "a/../../b", "..\\..\\etc\\passwd"} { if _, err := m.ResolvePath("srv-1", path); !errors.Is(err, ErrPathOutsideRoot) { t.Fatalf("%q err=%v", path, err) } }
    os.Symlink("/etc", filepath.Join(root,"srv-1","data","link"))
    if _, err := m.ResolvePath("srv-1", "link/passwd"); !errors.Is(err, ErrPathOutsideRoot) { t.Fatalf("symlink err=%v", err) }
}

func TestServerRootsAreIsolated(t *testing.T) {
    root := t.TempDir(); m := NewManager(root); os.MkdirAll(filepath.Join(root,"a","data"),0750); os.MkdirAll(filepath.Join(root,"b","data"),0750)
    os.WriteFile(filepath.Join(root,"b","data","secret"), []byte("x"),0600)
    if _, err := m.Read(context.Background(), "a", "../b/data/secret"); !errors.Is(err, ErrPathOutsideRoot) { t.Fatal("cross-server read harus ditolak") }
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/filesystem/ -v`
Expected: FAIL karena package manager belum ada.

- [ ] **Step 3: Implement path resolver**

Root server adalah `filepath.Join(serverRoot, serverID, "data")`; validasi server ID hanya `[A-Za-z0-9_-]+`. Reject absolute paths before clean. Decode URL path once if caller supplies encoded path. Clean `filepath.Join(root, relPath)` and boundary-check with `filepath.Rel`. Walk each existing component using `os.Lstat`; reject any symlink. Return sentinel errors without exposing host absolute paths.

- [ ] **Step 4: Implement operations**

Create server root on manager construction or first operation with mode `0750`. `List` returns relative child paths and `os.FileMode.String()`. `Read` opens verified path and returns `io.ReadCloser`. `Write` creates parent only after resolving parent, streams to temp file in same directory, chmod 0600, syncs, renames. Delete supports recursive only when requested. Rename/Copy resolve both paths and stream copy with `io.Copy`.

- [ ] **Step 5: Add streaming test and verify**

Write a 16MB `bytes.Reader`, call `Write`, then `Read` and `io.Copy(io.Discard, reader)`; assert content length and no API returning `[]byte`. Run: `go test ./internal/filesystem/ -v`.

- [ ] **Step 6: Commit**

```bash
```

---

### Task 3: Backup archive and restore manager

**Files:**
- Create: `internal/backup/archive.go`, `internal/backup/manager.go`, `internal/backup/restore.go`, `internal/backup/manager_test.go`

**Interfaces:**
- Produces: `BackupOptions`, `BackupResult`, `RestoreOptions`, `Manager`; `NewManager(filesystem.Manager, *store.DB, runtimeStatusProvider, config.FilesystemConfig) Manager`.

- [ ] **Step 1: Write failing tests**

```go
func TestCreateBackupAndRestoreChecksum(t *testing.T) {
    fs := newTestFilesystem(t); db := newBackupDB(t); mgr := NewManager(fs, db, stoppedStatusProvider{}, testFilesystemConfig(t))
    writeTestFile(t, fs, "srv-1", "hello.txt", "hello")
    result, err := mgr.Create(context.Background(), BackupOptions{ServerID:"srv-1"}); if err != nil { t.Fatal(err) }
    if result.SizeBytes == 0 || len(result.ChecksumSHA) != 64 { t.Fatalf("result=%+v", result) }
    if err := mgr.Restore(context.Background(), RestoreOptions{ServerID:"srv-1", BackupID:result.BackupID}); err != nil { t.Fatal(err) }
}

func TestRestoreRejectsRunningServerAndCorruptArchive(t *testing.T) {
    mgr := newBackupManager(t, runningStatusProvider{})
    if err := mgr.Restore(context.Background(), RestoreOptions{ServerID:"srv-1", BackupID:"backup-1"}); !errors.Is(err, ErrServerRunning) { t.Fatalf("err=%v", err) }
    mgr = newBackupManager(t, stoppedStatusProvider{}); corrupt := createCorruptBackup(t, mgr)
    if err := mgr.Restore(context.Background(), RestoreOptions{ServerID:"srv-1", BackupID:corrupt}); !errors.Is(err, ErrChecksumMismatch) { t.Fatalf("err=%v", err) }
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/backup/ -v`
Expected: FAIL karena backup package belum ada.

- [ ] **Step 3: Implement streaming tar.gz create**

Create metadata row status `creating`, write archive to `<backupRoot>/<serverID>/<id>.tar.gz.tmp`, use `tar.Writer` + `gzip.Writer` + `sha256.Hash` via `io.MultiWriter`, walk filesystem root without symlinks, write relative headers only, close/sync, rename final, stat size, update status completed/checksum. Any error marks failed and removes temp.

- [ ] **Step 4: Implement restore staging**

Load metadata, require stopped status, hash archive and compare SHA-256 before extraction. Extract into sibling `.restore-<id>` staging dir; reject absolute names, `..` relative escape, symlink/type entries, and paths outside staging. After complete, rename current data to `.previous-<id>`, rename staging to data, remove previous only after success; restore old directory on replacement failure.

- [ ] **Step 5: Implement list/delete and verify**

Implement backup manager List/Delete with DB metadata and archive removal. Run: `go test ./internal/backup/ -v`.

- [ ] **Step 6: Commit**

```bash
```

---

### Task 4: Filesystem gRPC streaming API

**Files:**
- Create: `proto/filesystem/filesystem.proto`, `internal/api/filesystem.go`, `internal/api/filesystem_test.go`
- Modify: `scripts/gen-proto.sh`
- Generate: `gen/go/filesystem/`

**Interfaces:**
- Produces: `FilesystemService` RPCs `List`, `Delete`, `Rename`, `Copy`, `CreateDir`, client-streaming `Upload`, server-streaming `Download`; chunk limit 1 MiB.

- [ ] **Step 1: Define proto and write bufconn test**

Proto messages:

```proto
message FileRequest { string server_id = 1; string path = 2; }
message FileInfo { string name = 1; string path = 2; bool is_dir = 3; int64 size_bytes = 4; int64 modified_unix = 5; string mode = 6; }
message ListResponse { repeated FileInfo files = 1; }
message UploadChunk { string server_id = 1; string path = 2; bytes data = 3; bool last = 4; }
message UploadResponse { int64 size_bytes = 1; }
message DownloadRequest { string server_id = 1; string path = 2; }
message DownloadChunk { bytes data = 1; bool last = 2; }
message RenameRequest { string server_id = 1; string old_path = 2; string new_path = 3; }
service FilesystemService { rpc List(FileRequest) returns (ListResponse); rpc Delete(FileRequest) returns (google.protobuf.Empty); rpc Rename(RenameRequest) returns (google.protobuf.Empty); rpc Copy(RenameRequest) returns (google.protobuf.Empty); rpc CreateDir(FileRequest) returns (google.protobuf.Empty); rpc Upload(stream UploadChunk) returns (UploadResponse); rpc Download(DownloadRequest) returns (stream DownloadChunk); }
```

Test uploads 3 chunks, downloads them, compares bytes, and calls traversal path expecting gRPC InvalidArgument/PermissionDenied.

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/api/ -run TestFilesystem -v`
Expected: FAIL karena proto/service belum ada.

- [ ] **Step 3: Generate and implement adapter**

Upload requires first chunk metadata, rejects missing/changed server/path, enforces max upload bytes, streams to manager writer, and returns size. Download reads manager and emits chunks <=1 MiB. Map sentinel errors to gRPC codes: traversal/permission `PermissionDenied`, missing `NotFound`, invalid request `InvalidArgument`, quota `ResourceExhausted`.

- [ ] **Step 4: Verify and commit**

Run: `make proto && go test ./internal/api/ -run TestFilesystem -v`.

```bash
git commit -m "feat: filesystem grpc streaming api"
```

---

### Task 5: Minimal jailed SFTP

**Files:**
- Create: `internal/filesystem/sftp.go`, `internal/filesystem/sftp_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Produces: `NewSFTPServer(cfg config.SFTPConfig, manager Manager, credentials CredentialStore) (*SFTPServer,error)`; `ListenAndServe(context.Context) error`; `Shutdown(context.Context) error`.

- [ ] **Step 1: Write failing SFTP security test**

Test creates an SSH keypair, credential bound to `srv-1`, starts server on `127.0.0.1:0`, connects with `ssh.Client`, opens SFTP client, writes/reads `hello.txt`, and asserts `../../srv-2/data/secret` fails.

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/filesystem/ -run TestSFTP -v`
Expected: FAIL karena SFTP server belum ada.

- [ ] **Step 3: Implement server and jail**

Generate/load host key, authenticate public keys against credential store, derive server ID from username/credential, and expose only manager-backed operations. Reject sessions without matching credential. Bind localhost unless explicitly configured. Use `pkg/sftp` request handlers backed by manager; never chdir to an unvalidated host path.

- [ ] **Step 4: Verify and commit**

Run: `go get github.com/pkg/sftp@latest golang.org/x/crypto@latest && go test ./internal/filesystem/ -run TestSFTP -v`.

```bash
git commit -m "feat: minimal jailed sftp"
```

---

### Task 6: Backup/file lifecycle wiring and docs

**Files:**
- Modify: `internal/orchestrator/lifecycle.go`, `cmd/oikos/daemon.go`, `internal/agent/daemon.go`
- Create: `internal/backup/lifecycle_test.go`, `docs/filesystem.md`

**Interfaces:**
- Consumes: filesystem/backup managers and existing lifecycle status.
- Produces: daemon-owned filesystem/backup services; `BackupServer` hot by default; `RestoreServer` rejects running server.

- [ ] **Step 1: Write lifecycle test**

```go
func TestRestoreServerRequiresStopped(t *testing.T) {
    backup := newBackupManager(t, runningStatusProvider{})
    err := backup.Restore(context.Background(), RestoreOptions{ServerID:"srv-1", BackupID:"b1"})
    if !errors.Is(err, ErrServerRunning) { t.Fatalf("err=%v", err) }
}
```

- [ ] **Step 2: Implement daemon ownership**

Initialize filesystem root and backup root from config, call `MigrateFilesystem`, construct managers once, register gRPC FilesystemService alongside existing API, and close SFTP/backup workers on context cancellation. Do not stop containers for default hot backup; only explicit `StopBeforeRun` requests may stop them.

- [ ] **Step 3: Document operations**

`docs/filesystem.md` must explain root layout, path safety, streaming limits, hot backup consistency limitation, restore stopped requirement, checksum, SFTP localhost default, credential lifecycle, and commands/examples for file API/backup.

- [ ] **Step 4: Verify and commit**

Run: `go test ./... && go vet ./...`.

```bash
git commit -m "feat: wiring filesystem backup lifecycle"
```

---

### Task 7: Integration tests and final verification

**Files:**
- Create: `internal/backup/hot_integration_test.go` with `//go:build integration`
- Create: `docs/filesystem-integration.md`

- [ ] **Step 1: Add environment-gated integration test**

Use `OIKOS_FILESYSTEM_INTEGRATION=1` to enable. Create a 16MB file, run backup while a writer modifies another file, verify archive checksum/extraction, modify/delete data, restore while stopped, and assert content. Run a second case with running status and assert restore rejection. Skip with reason when env is absent.

- [ ] **Step 2: Run tests**

Run:

```bash
```

Expected: unit/component PASS; large-file integration explicit SKIP unless enabled.

- [ ] **Step 3: Full verification**

```bash
make proto && make build && go test -count=1 ./... && go vet ./... && test -z "$(gofmt -l cmd internal gen)"
```

Expected: all checks pass.

- [ ] **Step 4: Commit final status**

```bash
git commit -m "chore: verifikasi fase 4 filesystem data"
```

---

## Self-Review

- Spec coverage: root isolation/path safety (Task 2), streaming API (Task 4), tar.gz/checksum/hot backup/restore staging (Task 3), SFTP (Task 5), SQLite metadata/config (Task 1), lifecycle integration/docs (Task 6), and integration evidence (Task 7) are mapped.
- No implementation step relies on host absolute paths from callers or whole-file buffers.
- Restore safety is explicit: stopped-only, checksum before extraction, staging, archive-entry validation, and rollback on replacement failure.
- SFTP and gRPC share the same manager, so traversal and permission policy cannot diverge.
- Integration tests are environment-gated with explicit skip behavior; no large-file or SFTP production claim is made without evidence.
