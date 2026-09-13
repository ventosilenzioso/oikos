package main

import (
	"github.com/oikos/oikos/internal/store"
	"os"
	"path/filepath"
	"testing"
)

func TestDoctorFakeConfigSucceeds(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "servers"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "backups"), 0750); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(filepath.Join(dir, "data", "oikos.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	db.Close()
	data := []byte("node:\n  data_dir: " + filepath.Join(dir, "data") + "\nruntime:\n  engine: fake\nfilesystem:\n  server_root: " + filepath.Join(dir, "servers") + "\n  backup_root: " + filepath.Join(dir, "backups") + "\n")
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := runDoctor([]string{"--config", configPath}); err != nil {
		t.Fatal(err)
	}
}

func TestDoctorRequiredDataDirFailure(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "config.yaml")
	data := []byte("node:\n  data_dir: " + blocker + "\nruntime:\n  engine: fake\n")
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := runDoctor([]string{"--config", configPath}); err == nil {
		t.Fatal("doctor harus gagal")
	}
}
