package filesystem

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type FileInfo struct {
	Name       string
	Path       string
	IsDir      bool
	SizeBytes  int64
	ModifiedAt time.Time
	Mode       string
}

type Manager interface {
	List(context.Context, string, string) ([]FileInfo, error)
	Read(context.Context, string, string) (io.ReadCloser, error)
	Write(context.Context, string, string, io.Reader) error
	Delete(context.Context, string, string, bool) error
	Rename(context.Context, string, string, string) error
	Copy(context.Context, string, string, string) error
	CreateDir(context.Context, string, string) error
	ResolvePath(string, string) (string, error)
}

type manager struct{ root string }

var validServerID = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func NewManager(root string) Manager { return &manager{root: root} }

func (m *manager) serverRoot(serverID string) (string, error) {
	if !validServerID.MatchString(serverID) {
		return "", ErrServerNotFound
	}
	return filepath.Join(m.root, serverID, "data"), nil
}

func (m *manager) ResolvePath(serverID, relPath string) (string, error) {
	root, err := m.serverRoot(serverID)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(relPath) || strings.Contains(relPath, "\x00") {
		return "", ErrPathOutsideRoot
	}
	// Backslash is a separator on Windows and an escape attempt on Linux.
	if strings.Contains(relPath, `\`) {
		return "", ErrPathOutsideRoot
	}
	abs := filepath.Join(root, filepath.Clean(relPath))
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", ErrPathOutsideRoot
	}
	if err := m.rejectSymlinkComponents(root, abs); err != nil {
		return "", err
	}
	return abs, nil
}

func (m *manager) rejectSymlinkComponents(root, target string) error {
	current := root
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return ErrPathOutsideRoot
	}
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if part == "." || part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return ErrPathOutsideRoot
		}
	}
	return nil
}

func (m *manager) ensureRoot(serverID string) (string, error) {
	root, err := m.serverRoot(serverID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0750); err != nil {
		return "", fmt.Errorf("buat root server: %w", err)
	}
	return root, nil
}

func (m *manager) List(ctx context.Context, serverID, relPath string) ([]FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	abs, err := m.ResolvePath(serverID, relPath)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	out := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		child, _ := filepath.Rel(filepath.Join(m.root, serverID, "data"), filepath.Join(abs, entry.Name()))
		out = append(out, FileInfo{Name: entry.Name(), Path: filepath.ToSlash(child), IsDir: info.IsDir(), SizeBytes: info.Size(), ModifiedAt: info.ModTime(), Mode: info.Mode().String()})
	}
	return out, nil
}

func (m *manager) Read(ctx context.Context, serverID, relPath string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	abs, err := m.ResolvePath(serverID, relPath)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return f, nil
}

func (m *manager) Write(ctx context.Context, serverID, relPath string, content io.Reader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root, err := m.ensureRoot(serverID)
	if err != nil {
		return err
	}
	abs, err := m.ResolvePath(serverID, relPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(abs), ".upload-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := io.Copy(tmp, content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	_ = root
	return os.Rename(name, abs)
}

func (m *manager) Delete(ctx context.Context, serverID, relPath string, recursive bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	abs, err := m.ResolvePath(serverID, relPath)
	if err != nil {
		return err
	}
	info, err := os.Lstat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	if info.IsDir() && recursive {
		return os.RemoveAll(abs)
	}
	return os.Remove(abs)
}
func (m *manager) Rename(ctx context.Context, serverID, oldPath, newPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	oldAbs, err := m.ResolvePath(serverID, oldPath)
	if err != nil {
		return err
	}
	newAbs, err := m.ResolvePath(serverID, newPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(newAbs), 0750); err != nil {
		return err
	}
	return os.Rename(oldAbs, newAbs)
}
func (m *manager) Copy(ctx context.Context, serverID, srcPath, dstPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	src, err := m.Read(ctx, serverID, srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	return m.Write(ctx, serverID, dstPath, src)
}
func (m *manager) CreateDir(ctx context.Context, serverID, relPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	abs, err := m.ResolvePath(serverID, relPath)
	if err != nil {
		return err
	}
	return os.MkdirAll(abs, 0750)
}
