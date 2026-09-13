package config

import "testing"

func TestFilesystemDefaults(t *testing.T) {
	cfg := Default()
	if cfg.Filesystem.ServerRoot != "/var/lib/oikos/servers" {
		t.Fatal(cfg.Filesystem.ServerRoot)
	}
	if cfg.Filesystem.BackupRoot != "/var/lib/oikos/backups" {
		t.Fatal(cfg.Filesystem.BackupRoot)
	}
	if cfg.SFTP.Enabled {
		t.Fatal("SFTP default harus disabled")
	}
}
