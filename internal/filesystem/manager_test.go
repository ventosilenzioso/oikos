package filesystem

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePathRejectsEscapeAndSymlink(t *testing.T) {
	root := t.TempDir()
	m := NewManager(root)
	if err := os.MkdirAll(filepath.Join(root, "srv-1", "data"), 0750); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../../etc/passwd", "/etc/passwd", "a/../../b", `..\..\etc\passwd`} {
		if _, err := m.ResolvePath("srv-1", path); !errors.Is(err, ErrPathOutsideRoot) {
			t.Fatalf("%q err=%v", path, err)
		}
	}
	if err := os.Symlink("/etc", filepath.Join(root, "srv-1", "data", "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := m.ResolvePath("srv-1", "link/passwd"); !errors.Is(err, ErrPathOutsideRoot) {
		t.Fatalf("symlink err=%v", err)
	}
}

func TestServerRootsAreIsolated(t *testing.T) {
	root := t.TempDir()
	m := NewManager(root)
	if err := os.MkdirAll(filepath.Join(root, "a", "data"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "b", "data"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b", "data", "secret"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Read(context.Background(), "a", "../b/data/secret"); !errors.Is(err, ErrPathOutsideRoot) {
		t.Fatal("cross-server read harus ditolak")
	}
}

func TestManagerWriteReadAndOperations(t *testing.T) {
	m := NewManager(t.TempDir())
	ctx := context.Background()
	if err := m.Write(ctx, "srv", "nested/hello.txt", strings.NewReader("hello")); err != nil {
		t.Fatal(err)
	}
	r, err := m.Read(ctx, "srv", "nested/hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("data=%q", data)
	}
	if err := m.CreateDir(ctx, "srv", "copy"); err != nil {
		t.Fatal(err)
	}
	if err := m.Copy(ctx, "srv", "nested/hello.txt", "copy/hello.txt"); err != nil {
		t.Fatal(err)
	}
	if err := m.Rename(ctx, "srv", "copy/hello.txt", "copy/renamed.txt"); err != nil {
		t.Fatal(err)
	}
	entries, err := m.List(ctx, "srv", "copy")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "renamed.txt" {
		t.Fatalf("entries=%+v", entries)
	}
	if err := m.Delete(ctx, "srv", "copy", true); err != nil {
		t.Fatal(err)
	}
}
