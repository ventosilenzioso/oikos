package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDoctorFakeConfigSucceeds(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	data := []byte("node:\n  data_dir: " + filepath.Join(dir, "data") + "\nruntime:\n  engine: fake\n")
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
