package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/client"
	"github.com/oikos/oikos/internal/agent"
	"github.com/oikos/oikos/internal/config"
	"github.com/oikos/oikos/internal/orchestrator"
	"github.com/oikos/oikos/internal/plugin"
	"github.com/oikos/oikos/internal/resource"
	"github.com/oikos/oikos/internal/security"
	"github.com/oikos/oikos/internal/store"
)

type doctorResult int

const (
	doctorOK doctorResult = iota
	doctorWarn
	doctorFail
)

type doctorCheck struct {
	Name     string
	Required bool
	Run      func() (doctorResult, string)
}

func runDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	configPath := fs.String("config", "config.yaml", "path config.yaml")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Printf("[FAIL] config: %v\n", err)
		return err
	}
	var db *store.DB
	checks := []doctorCheck{
		{Name: "config", Required: true, Run: func() (doctorResult, string) {
			return doctorOK, fmt.Sprintf("engine=%s panel=%s", cfg.Runtime.Engine, cfg.Panel.Address)
		}},
		{Name: "node.data_dir", Required: true, Run: func() (doctorResult, string) { return checkReadableWritableDir(cfg.Node.DataDir) }},
		{Name: "filesystem.server_root", Required: true, Run: func() (doctorResult, string) { return checkExistingDir(cfg.Filesystem.ServerRoot) }},
		{Name: "filesystem.backup_root", Required: true, Run: func() (doctorResult, string) { return checkExistingDir(cfg.Filesystem.BackupRoot) }},
		{Name: "sqlite", Required: true, Run: func() (doctorResult, string) {
			var err error
			db, err = store.Open(filepath.Join(cfg.Node.DataDir, "oikos.db"))
			if err != nil {
				return doctorFail, err.Error()
			}
			if err := db.Ping(); err != nil {
				db.Close()
				db = nil
				return doctorFail, err.Error()
			}
			if err := db.MigratePlugins(); err != nil {
				return doctorFail, "plugin schema: " + err.Error()
			}
			return doctorOK, "read/write connection ok"
		}},
		{Name: "docker.socket", Required: cfg.Runtime.Engine != "fake", Run: func() (doctorResult, string) {
			if cfg.Runtime.Engine == "fake" {
				return doctorWarn, "not required: engine fake"
			}
			options := []client.Opt{client.WithAPIVersionNegotiation()}
			if cfg.Runtime.DockerSocket != "" {
				options = append(options, client.WithHost(cfg.Runtime.DockerSocket))
			} else {
				options = append(options, client.FromEnv)
			}
			cli, err := client.NewClientWithOpts(options...)
			if err != nil {
				return doctorFail, err.Error()
			}
			defer cli.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if _, err := cli.Ping(ctx); err != nil {
				return doctorFail, err.Error()
			}
			return doctorOK, "daemon reachable"
		}},
		{Name: "cgroups.v2", Required: false, Run: func() (doctorResult, string) {
			if v := resource.DetectCgroupVersion(); v == "v2" {
				return doctorOK, "available"
			}
			return doctorWarn, "v2 unavailable; resource limits may be degraded"
		}},
		{Name: "panel.mtls", Required: false, Run: func() (doctorResult, string) {
			if db == nil {
				return doctorWarn, "skipped: SQLite unavailable"
			}
			node, err := db.GetNode()
			if errors.Is(err, sql.ErrNoRows) {
				return doctorWarn, "skipped: node belum paired"
			}
			if err != nil {
				return doctorFail, err.Error()
			}
			if _, err := os.Stat(node.CertPath); err != nil {
				return doctorFail, "cert: " + err.Error()
			}
			if _, err := os.Stat(node.KeyPath); err != nil {
				return doctorFail, "key: " + err.Error()
			}
			if _, err := security.LoadClientTLS(node.CertPath, node.KeyPath, cfg.Panel.CAPath); err != nil {
				return doctorFail, err.Error()
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			conn, err := agent.DialPanel(ctx, cfg.Panel.Address, node.CertPath, node.KeyPath, cfg.Panel.CAPath)
			if err != nil {
				return doctorFail, "mTLS/gRPC: " + err.Error()
			}
			_ = conn.Close()
			return doctorOK, "mTLS/gRPC reachable"
		}},
		{Name: "plugins", Required: false, Run: func() (doctorResult, string) {
			if db == nil {
				return doctorWarn, "skipped: SQLite unavailable"
			}
			registry := plugin.NewRegistry(db, orchestrator.NewEventBus(), plugin.HostOptions{SocketRoot: filepath.Join(cfg.Node.DataDir, "plugins"), OikosVersion: "doctor"})
			if err := registry.LoadEnabled(); err != nil {
				return doctorFail, "load: " + err.Error()
			}
			health := registry.HealthCheckAll()
			for id, err := range health {
				if err != nil {
					return doctorFail, id + ": " + err.Error()
				}
			}
			if len(health) == 0 {
				return doctorWarn, "no enabled plugins"
			}
			return doctorOK, fmt.Sprintf("%d enabled plugin(s) healthy", len(health))
		}},
		{Name: "tunnel.frpc", Required: false, Run: func() (doctorResult, string) {
			if cfg.Runtime.FRPBinary == "" {
				return doctorWarn, "skipped: frp binary tidak dikonfigurasi"
			}
			if _, err := os.Stat(cfg.Runtime.FRPBinary); err != nil {
				return doctorWarn, "process tidak aktif atau binary tidak tersedia: " + err.Error()
			}
			return doctorWarn, "binary tersedia; process status dikelola daemon"
		}},
	}
	failures := 0
	for _, check := range checks {
		result, msg := check.Run()
		switch result {
		case doctorOK:
			fmt.Printf("[ok] %s: %s\n", check.Name, msg)
		case doctorWarn:
			fmt.Printf("[warn] %s: %s\n", check.Name, msg)
		case doctorFail:
			fmt.Printf("[FAIL] %s: %s\n", check.Name, msg)
			if check.Required {
				failures++
			}
		}
	}
	if db != nil {
		db.Close()
	}
	if failures > 0 {
		return fmt.Errorf("doctor menemukan %d required failure", failures)
	}
	return nil
}

func checkExistingDir(path string) (doctorResult, string) {
	if path == "" {
		return doctorFail, "path kosong"
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return doctorWarn, "belum dibuat: " + path
		}
		return doctorFail, err.Error()
	}
	if !info.IsDir() {
		return doctorFail, "bukan direktori"
	}
	return doctorOK, "tersedia"
}

func checkReadableWritableDir(path string) (doctorResult, string) {
	if path == "" {
		return doctorFail, "path kosong"
	}
	info, err := os.Stat(path)
	if err != nil {
		return doctorFail, err.Error()
	}
	if !info.IsDir() {
		return doctorFail, "bukan direktori"
	}
	perm := info.Mode().Perm()
	if perm&0400 == 0 {
		return doctorFail, "tidak readable menurut permission bits"
	}
	if perm&0200 == 0 {
		return doctorWarn, "readable tetapi permission bits tidak writable"
	}
	return doctorOK, "readable dan writable menurut permission bits"
}
