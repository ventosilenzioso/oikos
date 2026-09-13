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
	"github.com/oikos/oikos/internal/config"
	"github.com/oikos/oikos/internal/resource"
	"github.com/oikos/oikos/internal/store"
)

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
	failures := 0
	ok := func(name, msg string) { fmt.Printf("[ok] %s: %s\n", name, msg) }
	warn := func(name, msg string) { fmt.Printf("[warn] %s: %s\n", name, msg) }
	fail := func(name, msg string) { failures++; fmt.Printf("[FAIL] %s: %s\n", name, msg) }
	ok("config", fmt.Sprintf("engine=%s panel=%s", cfg.Runtime.Engine, cfg.Panel.Address))
	if err := os.MkdirAll(cfg.Node.DataDir, 0750); err != nil {
		fail("node.data_dir", err.Error())
	} else {
		ok("node.data_dir", cfg.Node.DataDir)
	}

	for name, path := range map[string]string{"filesystem.server_root": cfg.Filesystem.ServerRoot, "filesystem.backup_root": cfg.Filesystem.BackupRoot} {
		if err := os.MkdirAll(path, 0750); err != nil {
			fail(name, err.Error())
		} else {
			ok(name, path)
		}
	}
	db, err := store.Open(filepath.Join(cfg.Node.DataDir, "oikos.db"))
	if err != nil {
		fail("sqlite", err.Error())
	} else {
		defer db.Close()
		if err := db.Migrate(); err != nil {
			fail("sqlite", err.Error())
		} else {
			ok("sqlite", "open+migrate ok")
		}
	}
	if cfg.Runtime.Engine == "fake" {
		warn("docker", "dilewati karena engine fake")
	} else {
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			fail("docker", err.Error())
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, err = cli.Ping(ctx)
			cancel()
			cli.Close()
			if err != nil {
				fail("docker", err.Error())
			} else {
				ok("docker", "daemon reachable")
			}
		}
	}
	ok("cgroup", resource.DetectCgroupVersion())
	if db != nil {
		if _, err := db.GetNode(); errors.Is(err, sql.ErrNoRows) {
			warn("pairing", "belum pairing")
		} else if err != nil {
			fail("pairing", err.Error())
		} else {
			ok("pairing", "node paired")
		}
	}
	if _, err := os.Stat(cfg.Runtime.FRPBinary); err != nil {
		warn("frp", "binary tidak tersedia")
	} else {
		ok("frp", cfg.Runtime.FRPBinary)
	}
	if failures > 0 {
		return fmt.Errorf("doctor menemukan %d failure", failures)
	}
	return nil
}
