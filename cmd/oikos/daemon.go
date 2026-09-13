package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/oikos/oikos/internal/agent"
	"github.com/oikos/oikos/internal/config"
	"github.com/oikos/oikos/internal/store"
)

func runDaemon(args []string) error {
	fs := flag.NewFlagSet("daemon", flag.ContinueOnError)
	configPath := fs.String("config", "/etc/oikos/config.yaml", "path config.yaml")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	flagConfig = *configPath
	lc, err := openLifecycleWith(cfg.Runtime.Engine)
	if err != nil {
		return err
	}
	db, err := store.Open(filepath.Join(cfg.Node.DataDir, "oikos.db"))
	if err != nil {
		return err
	}
	defer db.Close()
	node, err := db.GetNode()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("belum pairing, jalankan oikos install dulu")
		}
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return agent.Run(ctx, agent.DaemonArgs{
		PanelAddr: cfg.Panel.Address,
		CertPath:  node.CertPath,
		KeyPath:   node.KeyPath,
		CAPath:    cfg.Panel.CAPath,
		NodeID:    node.ID,
		LocalPort: cfg.API.LocalGRPCPort,
	}, lc)
}
