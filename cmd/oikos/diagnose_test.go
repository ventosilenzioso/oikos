package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiagnoseFreshOK(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := "node:\n  data_dir: \"" + filepath.Join(dir, "data") + "\"\nruntime:\n  engine: \"fake\"\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := runDiagnose([]string{"--config", cfgPath}); err != nil {
		t.Fatalf("diagnose fresh harus ok: %v", err)
	}
}

func TestDiagnoseFailsWhenDataDirIsFile(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "config.yaml")
	content := "node:\n  data_dir: \"" + blocker + "\"\nruntime:\n  engine: \"fake\"\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := runDiagnose([]string{"--config", cfgPath}); err == nil {
		t.Fatal("diagnose harus gagal bila data dir tidak bisa dibuat")
	}
}
