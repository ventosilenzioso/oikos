//go:build integration

package backup

import (
	"context"
	"github.com/oikos/oikos/internal/config"
	"github.com/oikos/oikos/internal/filesystem"
	"github.com/oikos/oikos/internal/store"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type integrationReader struct{ remaining int64 }

func (r *integrationReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := int64(len(p))
	if n > r.remaining {
		n = r.remaining
	}
	for i := int64(0); i < n; i++ {
		p[i] = 'x'
	}
	r.remaining -= n
	return int(n), nil
}

func TestHotBackupIntegration(t *testing.T) {
	if os.Getenv("OIKOS_FILESYSTEM_INTEGRATION") != "1" {
		t.Skip("filesystem integration dilewati: set OIKOS_FILESYSTEM_INTEGRATION=1")
	}
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "o.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateEgg(store.Egg{ID: "hot-egg", Name: "hot", DockerfilePath: "d", MetadataPath: "m"}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateServer(store.Server{ID: "srv", Name: "srv", EggID: "hot-egg", Status: "stopped", StartupCommand: "run", Environment: "{}"}); err != nil {
		t.Fatal(err)
	}
	fs := filesystem.NewManager(filepath.Join(dir, "servers"))
	data := &integrationReader{remaining: 16 * 1024 * 1024}
	if err := fs.Write(context.Background(), "srv", "large.bin", data); err != nil {
		t.Fatal(err)
	}
	mgr := NewManager(fs, db, statusProvider{status: "stopped"}, config.FilesystemConfig{ServerRoot: filepath.Join(dir, "servers"), BackupRoot: filepath.Join(dir, "backups")})
	result, err := mgr.Create(context.Background(), BackupOptions{ServerID: "srv"})
	if err != nil {
		t.Fatal(err)
	}
	if result.SizeBytes == 0 || len(result.ChecksumSHA) != 64 {
		t.Fatalf("result=%+v", result)
	}
}
