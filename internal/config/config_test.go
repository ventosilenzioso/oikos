package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultCAPath(t *testing.T) {
	c := Default()
	if c.Panel.CAPath == "" {
		t.Fatal("default ca_path kosong")
	}
}

func TestDefault(t *testing.T) {
	c := Default()
	if c.Panel.Address == "" {
		t.Fatal("default panel address kosong")
	}
	if c.Runtime.Engine != "docker" {
		t.Fatalf("default engine = %q, mau docker", c.Runtime.Engine)
	}
}

func TestLoadMissingFileUsesDefault(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "tidak-ada.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Node.DataDir != Default().Node.DataDir {
		t.Fatal("file hilang harus fallback ke default")
	}
}

func TestLoadOverridesDefault(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	content := "panel:\n  address: \"panel.test:9091\"\nlog:\n  level: \"debug\"\n"
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Panel.Address != "panel.test:9091" {
		t.Fatalf("address = %q", c.Panel.Address)
	}
	if c.Log.Level != "debug" {
		t.Fatalf("level = %q", c.Log.Level)
	}
	if c.Runtime.Engine != "docker" {
		t.Fatalf("engine harus tetap default docker, dapat %q", c.Runtime.Engine)
	}
}
