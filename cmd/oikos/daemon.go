package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tunnelpb "github.com/oikos/oikos/gen/go/tunnel"
	"github.com/oikos/oikos/internal/agent"
	"github.com/oikos/oikos/internal/config"
	"github.com/oikos/oikos/internal/orchestrator"
	"github.com/oikos/oikos/internal/runtime"
	"github.com/oikos/oikos/internal/runtime/docker"
	"github.com/oikos/oikos/internal/store"
	"github.com/oikos/oikos/internal/tunnel"
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
	db, err := store.Open(filepath.Join(cfg.Node.DataDir, "oikos.db"))
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.MigrateNetworking(); err != nil {
		return err
	}
	node, err := db.GetNode()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("belum pairing, jalankan oikos install dulu")
		}
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	conn, err := agent.DialPanel(ctx, cfg.Panel.Address, node.CertPath, node.KeyPath, cfg.Panel.CAPath)
	if err != nil {
		return err
	}
	var rt runtime.Runtime
	if cfg.Runtime.Engine == "fake" {
		rt = runtime.NewFake()
	} else {
		rt, err = docker.New(cfg.Runtime.DockerSocket)
		if err != nil {
			return err
		}
	}
	bus := orchestrator.NewEventBus()
	defer conn.Close()
	tunnelClient := tunnelpb.NewTunnelServiceClient(conn)
	manager, err := tunnel.NewFromConfig(cfg.Runtime, tunnel.NewPanelAllocator(tunnelClient, node.ID), db, func(typ, serverID string) { bus.Publish(orchestrator.Event{Type: typ, ServerID: serverID}) })
	if err != nil {
		return err
	}
	portRoot := os.Getenv("OIKOS_EGGS_DIR")
	if portRoot == "" {
		portRoot = "eggs"
	}
	lc := orchestrator.NewWithTunnel(db, rt, bus, manager, orchestrator.EggPortProvider{Root: portRoot})
	return agent.Run(ctx, agent.DaemonArgs{
		PanelAddr: cfg.Panel.Address,
		CertPath:  node.CertPath,
		KeyPath:   node.KeyPath,
		CAPath:    cfg.Panel.CAPath,
		NodeID:    node.ID,
		LocalPort: cfg.API.LocalGRPCPort,
		Tunnel:    manager,
	}, lc)
}
