package filesystem

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecurityAuditRejectsTraversalVariants(t *testing.T) {
	root := t.TempDir()
	m := NewManager(root)
	if err := os.MkdirAll(filepath.Join(root, "server-a", "data"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "server-ab", "data"), 0750); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "outside.txt")
	if err := os.WriteFile(outside, []byte("protected"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../../outside.txt", "/etc/passwd", `..\..\outside.txt`, "../server-ab/data/secret"} {
		if _, err := m.ResolvePath("server-a", path); !errors.Is(err, ErrPathOutsideRoot) {
			t.Fatalf("path %q err=%v", path, err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(root, "server-a", "data", "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := m.ResolvePath("server-a", "link"); !errors.Is(err, ErrPathOutsideRoot) {
		t.Fatalf("symlink err=%v", err)
	}
	if got, err := os.ReadFile(outside); err != nil || string(got) != "protected" {
		t.Fatalf("outside changed: %q %v", got, err)
	}
}

func TestSecurityAuditRejectsRenameAndCopyDestinationEscape(t *testing.T) {
	m := NewManager(t.TempDir())
	if err := m.Write(context.Background(), "server-a", "source.txt", strings.NewReader("safe")); err != nil {
		t.Fatal(err)
	}
	if err := m.Rename(context.Background(), "server-a", "source.txt", "../../escape.txt"); !errors.Is(err, ErrPathOutsideRoot) {
		t.Fatalf("rename err=%v", err)
	}
	if err := m.Copy(context.Background(), "server-a", "source.txt", "../../escape.txt"); !errors.Is(err, ErrPathOutsideRoot) {
		t.Fatalf("copy err=%v", err)
	}
}

func TestSecurityAuditRejectsSymlinkParent(t *testing.T) {
	root := t.TempDir()
	m := NewManager(root)
	if err := os.MkdirAll(filepath.Join(root, "server-a", "data"), 0750); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "outside")
	if err := os.MkdirAll(target, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "server-a", "data", "nested")); err != nil {
		t.Fatal(err)
	}
	if err := m.Write(context.Background(), "server-a", "nested/file.txt", strings.NewReader("escape")); !errors.Is(err, ErrPathOutsideRoot) {
		t.Fatalf("write through symlink err=%v", err)
	}
}
