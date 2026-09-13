package backup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/oikos/oikos/internal/config"
	"github.com/oikos/oikos/internal/filesystem"
	"github.com/oikos/oikos/internal/store"
)

var (
	ErrServerRunning    = errors.New("server harus stopped untuk restore")
	ErrChecksumMismatch = errors.New("checksum backup tidak cocok")
)

type BackupOptions struct {
	ServerID      string
	StopBeforeRun bool
}
type BackupResult struct {
	BackupID    string
	SizeBytes   int64
	ChecksumSHA string
}
type RestoreOptions struct {
	ServerID       string
	BackupID       string
	TargetServerID string
}
type StatusProvider interface {
	Status(context.Context, string) (string, error)
}
type Manager interface {
	Create(context.Context, BackupOptions) (BackupResult, error)
	Restore(context.Context, RestoreOptions) error
	List(context.Context, string) ([]BackupResult, error)
	Delete(context.Context, string) error
}

type manager struct {
	filesystem filesystem.Manager
	db         *store.DB
	status     StatusProvider
	cfg        config.FilesystemConfig
}

func NewManager(fs filesystem.Manager, db *store.DB, status StatusProvider, cfg config.FilesystemConfig) Manager {
	return &manager{filesystem: fs, db: db, status: status, cfg: cfg}
}

func (m *manager) Create(ctx context.Context, opts BackupOptions) (BackupResult, error) {
	if opts.ServerID == "" {
		return BackupResult{}, fmt.Errorf("server id wajib diisi")
	}
	if opts.StopBeforeRun {
		return BackupResult{}, fmt.Errorf("StopBeforeRun membutuhkan lifecycle integration")
	}
	root, err := m.filesystem.ResolvePath(opts.ServerID, ".")
	if err != nil {
		return BackupResult{}, err
	}
	id := uuid.NewString()
	dir := filepath.Join(m.cfg.BackupRoot, opts.ServerID)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return BackupResult{}, err
	}
	archive := filepath.Join(dir, id+".tar.gz")
	tmp := archive + ".tmp"
	if err := m.db.SaveBackup(store.Backup{ID: id, ServerID: opts.ServerID, FilePath: archive, Status: "creating", CreatedAt: time.Now().UTC()}); err != nil {
		return BackupResult{}, err
	}
	result, err := createArchive(ctx, root, tmp, id)
	if err != nil {
		_ = m.db.UpdateBackupStatus(id, "failed", "", 0)
		_ = os.Remove(tmp)
		return BackupResult{}, err
	}
	if err := os.Rename(tmp, archive); err != nil {
		_ = m.db.UpdateBackupStatus(id, "failed", "", 0)
		return BackupResult{}, err
	}
	result.BackupID = id
	if err := m.db.UpdateBackupStatus(id, "completed", result.ChecksumSHA, result.SizeBytes); err != nil {
		return BackupResult{}, err
	}
	return result, nil
}

func (m *manager) Restore(ctx context.Context, opts RestoreOptions) error {
	target := opts.TargetServerID
	if target == "" {
		target = opts.ServerID
	}
	if target == "" {
		return fmt.Errorf("server id wajib diisi")
	}
	if m.status != nil {
		status, err := m.status.Status(ctx, target)
		if err != nil {
			return err
		}
		if status != "stopped" {
			return ErrServerRunning
		}
	}
	b, err := m.db.GetBackup(opts.BackupID)
	if err != nil {
		return err
	}
	if b.Status != "completed" {
		return fmt.Errorf("backup belum completed")
	}
	actual, err := checksumFile(b.FilePath)
	if err != nil {
		return err
	}
	if actual != b.ChecksumSHA {
		return ErrChecksumMismatch
	}
	root, err := m.filesystem.ResolvePath(target, ".")
	if err != nil {
		return err
	}
	staging := filepath.Join(filepath.Dir(root), ".restore-"+b.ID)
	_ = os.RemoveAll(staging)
	if err := os.MkdirAll(staging, 0750); err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	if err := extractArchive(ctx, b.FilePath, staging); err != nil {
		return err
	}
	previous := filepath.Join(filepath.Dir(root), ".previous-"+b.ID)
	_ = os.RemoveAll(previous)
	if _, err := os.Stat(root); err == nil {
		if err := os.Rename(root, previous); err != nil {
			return err
		}
	}
	if err := os.Rename(staging, root); err != nil {
		_ = os.Rename(previous, root)
		return err
	}
	_ = os.RemoveAll(previous)
	return nil
}

func (m *manager) List(ctx context.Context, serverID string) ([]BackupResult, error) {
	rows, err := m.db.ListBackups(serverID)
	if err != nil {
		return nil, err
	}
	out := make([]BackupResult, 0, len(rows))
	for _, b := range rows {
		out = append(out, BackupResult{BackupID: b.ID, SizeBytes: b.SizeBytes, ChecksumSHA: b.ChecksumSHA})
	}
	return out, nil
}
func (m *manager) Delete(ctx context.Context, id string) error {
	b, err := m.db.GetBackup(id)
	if err != nil {
		return err
	}
	if err := os.Remove(b.FilePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return m.db.DeleteBackup(id)
}
