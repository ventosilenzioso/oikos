package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/oikos/oikos/internal/config"
	"github.com/oikos/oikos/internal/runtime/docker"
	"github.com/oikos/oikos/internal/store"
	internalupdate "github.com/oikos/oikos/internal/update"
)

var (
	updateVerifier     internalupdate.ReleaseVerifier = environmentReleaseVerifier{}
	updateHealthRunner                                = defaultHealthRunner
)

func runUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	version := fs.String("version", "", "release version")
	url := fs.String("url", "", "release binary URL")
	sha256 := fs.String("sha256", "", "release SHA-256")
	signature := fs.String("signature", "", "release signature (base64 or text)")
	versionsDir := fs.String("versions-dir", "", "versioned binary directory")
	configPath := fs.String("config", "config.yaml", "path config.yaml")
	healthOnly := fs.Bool("health-check-only", false, "only validate config, SQLite, and Docker")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *healthOnly {
		return runHealthCheck(*configPath)
	}
	if *version == "" || *url == "" || *sha256 == "" || *signature == "" || *versionsDir == "" {
		return errors.New("version, url, sha256, signature, dan versions-dir wajib diisi")
	}
	sig, err := decodeSignature(*signature)
	if err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	dbPath := filepath.Join(cfg.Node.DataDir, "oikos.db")
	backup := func() error {
		backupDir := filepath.Join(*versionsDir, "backups", time.Now().UTC().Format("20060102T150405.000000000Z"))
		if err := os.MkdirAll(backupDir, 0o700); err != nil {
			return err
		}
		if err := copyFile(*configPath, filepath.Join(backupDir, "config.yaml")); err != nil {
			return err
		}
		return copyFile(dbPath, filepath.Join(backupDir, "oikos.db"))
	}
	health := internalupdate.HealthRunnerFunc(func(candidate string) error {
		return updateHealthRunner(candidate, *configPath)
	})
	mgr := internalupdate.NewManager(internalupdate.SlotConfig{Root: *versionsDir, Backup: backup}, updateVerifier, health)
	return mgr.Apply(internalupdate.Release{Version: *version, BinaryURL: *url, SHA256: *sha256, Signature: sig})
}

func decodeSignature(value string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err == nil {
		return decoded, nil
	}
	decoded, err = hex.DecodeString(value)
	if err == nil {
		return decoded, nil
	}
	return []byte(value), nil
}

func runHealthCheck(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.Node.DataDir, 0o750); err != nil {
		return fmt.Errorf("config data directory: %w", err)
	}
	db, err := store.Open(filepath.Join(cfg.Node.DataDir, "oikos.db"))
	if err != nil {
		return fmt.Errorf("sqlite: %w", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		return fmt.Errorf("sqlite migration: %w", err)
	}
	if cfg.Runtime.Engine == "fake" {
		return nil
	}
	rt, err := docker.New(cfg.Runtime.DockerSocket)
	if err != nil {
		return fmt.Errorf("docker: %w", err)
	}
	if _, err := rt.Stats(context.Background(), "__health_probe__"); err != nil {
		return fmt.Errorf("docker: %w", err)
	}
	return nil
}

func defaultHealthRunner(candidate, configPath string) error {
	cmd := exec.Command(candidate, "--health-check-only", "--config", configPath)
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func copyFile(source, destination string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, data, 0o600)
}

type environmentReleaseVerifier struct{}

func (environmentReleaseVerifier) Verify(release internalupdate.Release, body []byte) error {
	key, err := decodeSignature(os.Getenv("OIKOS_UPDATE_PUBLIC_KEY"))
	if err != nil || len(key) != ed25519.PublicKeySize {
		return errors.New("OIKOS_UPDATE_PUBLIC_KEY must be a base64 or hex Ed25519 public key")
	}
	if !ed25519.Verify(ed25519.PublicKey(key), body, release.Signature) {
		return errors.New("invalid Ed25519 release signature")
	}
	return nil
}
