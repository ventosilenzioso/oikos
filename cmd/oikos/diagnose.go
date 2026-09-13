package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
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

func runDiagnose(args []string) error {
	fs := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	configPath := fs.String("config", "config.yaml", "path config.yaml")
	if err := fs.Parse(args); err != nil {
		return err
	}
	failed := false
	ok := func(name, detail string) { fmt.Printf("[ok] %s: %s\n", name, detail) }
	info := func(name, detail string) { fmt.Printf("[..] %s: %s\n", name, detail) }
	fail := func(name, detail string) {
		failed = true
		fmt.Printf("[FAIL] %s: %s\n", name, detail)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fail("config", err.Error())
		return fmt.Errorf("diagnose gagal")
	}
	ok("config", fmt.Sprintf("panel=%s engine=%s", cfg.Panel.Address, cfg.Runtime.Engine))

	if err := os.MkdirAll(cfg.Node.DataDir, 0750); err != nil {
		fail("data-dir", err.Error())
	} else {
		ok("data-dir", cfg.Node.DataDir)
	}
	var db *store.DB
	if db, err = store.Open(filepath.Join(cfg.Node.DataDir, "oikos.db")); err != nil {
		fail("sqlite-open", err.Error())
		db = nil
	} else if err := db.Migrate(); err != nil {
		fail("sqlite-migrate", err.Error())
	} else {
		ok("sqlite", "open+migrate ok")
	}
	if cfg.Runtime.Engine != "docker" {
		info("docker", "dilewati (engine fake)")
	} else {
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			fail("docker-client", err.Error())
		} else {
			pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, err := cli.Ping(pingCtx)
			cancel()
			cli.Close()
			if err != nil {
				fail("docker-ping", err.Error())
			} else {
				ok("docker", "daemon reachable")
			}
		}
	}
	info("cgroup", "versi "+resource.DetectCgroupVersion())
	if db == nil {
		info("pairing", "dilewati (sqlite gagal)")
		info("cert", "dilewati (sqlite gagal)")
	} else {
		defer db.Close()
		node, err := db.GetNode()
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				info("pairing", "belum pairing")
				info("cert", "dilewati (belum pairing)")
			} else {
				fail("pairing", err.Error())
			}
		} else {
			ok("pairing", fmt.Sprintf("node %s (%s)", node.ID, node.Name))
			raw, err := os.ReadFile(node.CertPath)
			if err != nil {
				fail("cert", err.Error())
			} else if keyPEM, err := os.ReadFile(node.KeyPath); err != nil {
				fail("cert", err.Error())
			} else if cert, err := tls.X509KeyPair(raw, keyPEM); err != nil {
				fail("cert", err.Error())
			} else if leaf, err := x509.ParseCertificate(cert.Certificate[0]); err != nil {
				fail("cert", err.Error())
			} else if time.Now().After(leaf.NotAfter) {
				fail("cert", "kedaluwarsa "+leaf.NotAfter.String())
			} else {
				ok("cert", "valid hingga "+leaf.NotAfter.Format(time.RFC3339))
			}
		}
	}
	if failed {
		return fmt.Errorf("diagnose gagal")
	}
	return nil
}
