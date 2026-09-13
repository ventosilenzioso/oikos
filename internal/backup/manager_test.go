package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oikos/oikos/internal/config"
	"github.com/oikos/oikos/internal/filesystem"
	"github.com/oikos/oikos/internal/store"
)

type statusProvider struct{ status string }

func (p statusProvider) Status(context.Context, string) (string, error) { return p.status, nil }

func newBackupFixture(t *testing.T, status string) (Manager, *store.DB) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "o.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateEgg(store.Egg{ID: "backup-egg", Name: "backup", DockerfilePath: "d", MetadataPath: "m"}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateServer(store.Server{ID: "srv-1", Name: "srv", EggID: "backup-egg", Status: status, StartupCommand: "run", Environment: "{}"}); err != nil {
		t.Fatal(err)
	}
	fs := filesystem.NewManager(filepath.Join(dir, "servers"))
	return NewManager(fs, db, statusProvider{status: status}, config.FilesystemConfig{ServerRoot: filepath.Join(dir, "servers"), BackupRoot: filepath.Join(dir, "backups")}), db
}

func TestCreateBackupAndRestoreChecksum(t *testing.T) {
	mgr, _ := newBackupFixture(t, "stopped")
	fs := mgr.(*manager).filesystem
	if err := fs.Write(context.Background(), "srv-1", "hello.txt", strings.NewReader("hello")); err != nil {
		t.Fatal(err)
	}
	result, err := mgr.Create(context.Background(), BackupOptions{ServerID: "srv-1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.SizeBytes == 0 || len(result.ChecksumSHA) != 64 {
		t.Fatalf("result=%+v", result)
	}
	if err := fs.Delete(context.Background(), "srv-1", "hello.txt", false); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Restore(context.Background(), RestoreOptions{ServerID: "srv-1", BackupID: result.BackupID}); err != nil {
		t.Fatal(err)
	}
	r, err := fs.Read(context.Background(), "srv-1", "hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	data, _ := io.ReadAll(r)
	if string(data) != "hello" {
		t.Fatalf("restored=%q", data)
	}
}

func TestRestoreRejectsRunningServer(t *testing.T) {
	mgr, _ := newBackupFixture(t, "running")
	err := mgr.Restore(context.Background(), RestoreOptions{ServerID: "srv-1", BackupID: "missing"})
	if !errors.Is(err, ErrServerRunning) {
		t.Fatalf("err=%v", err)
	}
}

func TestExtractArchiveRejectsTraversal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "../../escape", Mode: 0600, Size: 1}); err != nil {
		t.Fatal(err)
	}
	_, _ = tw.Write([]byte("x"))
	_ = tw.Close()
	_ = gz.Close()
	_ = f.Close()
	if err := extractArchive(context.Background(), path, t.TempDir()); err == nil {
		t.Fatal("traversal archive harus ditolak")
	}
}
