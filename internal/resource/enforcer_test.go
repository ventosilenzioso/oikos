package resource

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyAndReadLimits(t *testing.T) {
	dir := t.TempDir()
	lim := Limits{CPUMillicores: 500, MemoryBytes: 32 * 1024 * 1024, PIDMax: 64}
	if err := ApplyLimits(dir, lim); err != nil {
		t.Fatal(err)
	}
	got, err := ReadLimits(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != lim {
		t.Fatalf("got %+v, mau %+v", got, lim)
	}
}

func TestApplySkipsZero(t *testing.T) {
	dir := t.TempDir()
	if err := ApplyLimits(dir, Limits{}); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"cpu.max", "memory.max", "pids.max"} {
		if _, err := os.Stat(filepath.Join(dir, f)); !os.IsNotExist(err) {
			t.Fatalf("%s harus tidak ditulis", f)
		}
	}
}

func TestFindContainerCgroup(t *testing.T) {
	root := t.TempDir()
	id := "abc123def456789"
	scope := filepath.Join(root, "system.slice", "docker-"+id+".scope")
	if err := os.MkdirAll(scope, 0755); err != nil {
		t.Fatal(err)
	}
	got, err := FindContainerCgroup(root, id)
	if err != nil {
		t.Fatal(err)
	}
	if got != scope {
		t.Fatalf("got %q, mau %q", got, scope)
	}
	if _, err := FindContainerCgroup(root, "tidakada000000"); err == nil {
		t.Fatal("harus error untuk id tak dikenal")
	}
}
