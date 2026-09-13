package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	tunnelpb "github.com/oikos/oikos/gen/go/tunnel"
	"github.com/oikos/oikos/internal/agent"
	"github.com/oikos/oikos/internal/api"
	"github.com/oikos/oikos/internal/config"
	"github.com/oikos/oikos/internal/filesystem"
	"github.com/oikos/oikos/internal/observability"
	"github.com/oikos/oikos/internal/orchestrator"
	"github.com/oikos/oikos/internal/runtime"
	"github.com/oikos/oikos/internal/runtime/docker"
	"github.com/oikos/oikos/internal/store"
	"github.com/oikos/oikos/internal/tunnel"
	"github.com/prometheus/client_golang/prometheus"
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
	fileManager := filesystem.NewManager(cfg.Filesystem.ServerRoot)
	fileService := api.NewFilesystemService(fileManager, cfg.Filesystem.MaxUploadBytes, int(cfg.Filesystem.UploadChunkBytes))
	metrics := observability.NewMetrics(prometheus.NewRegistry(), observability.RuntimeSnapshotProvider{DB: db, Runtime: rt})
	health := observability.NewHealthChecker(map[string]observability.Probe{
		"sqlite": func(context.Context) error { return db.Ping() },
		"docker": func(ctx context.Context) error { _, err := rt.Stats(ctx, "__health_probe__"); return err },
		"panel":  func(context.Context) error { return nil },
	})
	obsServer := observability.NewHTTPServer(cfg.Observability, metrics.Handler(), observability.HealthHandler(health))
	go func() {
		if err := obsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = obsServer.Shutdown(shutdownCtx)
	}()
	if err := metrics.Refresh(ctx); err != nil {
		_ = err
	}
	recorder := orchestrator.NewEventRecorder(bus, db)
	recorderCtx, stopRecorder := context.WithCancel(ctx)
	go recorder.Start(recorderCtx)
	defer stopRecorder()
	go (&observability.Scheduler{Metrics: metrics, Health: health, MetricsInterval: cfg.Observability.MetricsInterval.Duration(), HealthInterval: cfg.Observability.HealthInterval.Duration()}).Run(ctx)
	return agent.Run(ctx, agent.DaemonArgs{
		PanelAddr:     cfg.Panel.Address,
		CertPath:      node.CertPath,
		KeyPath:       node.KeyPath,
		CAPath:        cfg.Panel.CAPath,
		NodeID:        node.ID,
		LocalPort:     cfg.API.LocalGRPCPort,
		Tunnel:        manager,
		Observability: api.NewObservabilityService(health, metrics, db),
		Filesystem:    fileService,
	}, lc)
}
