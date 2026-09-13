package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDoctorCommandIsDispatched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.yaml")
	if err := os.WriteFile(path, []byte("node: [invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"doctor", "--config", path}); err == nil {
		t.Fatal("doctor harus mengembalikan error config, bukan unknown command")
	}
}
