package agent

import (
	"context"
	"fmt"
	"net"
	"time"

	nodepb "github.com/oikos/oikos/gen/go/node"
	"github.com/oikos/oikos/internal/api"
	"github.com/oikos/oikos/internal/orchestrator"
	"github.com/oikos/oikos/internal/tunnel"
)

// DaemonArgs adalah parameter loop daemon.
type DaemonArgs struct {
	PanelAddr     string
	CertPath      string
	KeyPath       string
	CAPath        string
	NodeID        string
	LocalPort     int
	Tunnel        tunnel.Manager
	Observability *api.ObservabilityService
	Filesystem    *api.FilesystemService
}

// Run menjalankan loop daemon: serve API lokal, dial Panel, heartbeat +
// command stream, reconnect dengan backoff. Kembali hanya saat ctx selesai.
func Run(ctx context.Context, args DaemonArgs, lc *orchestrator.Lifecycle) error {
	lis, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", args.LocalPort))
	if err != nil {
		return fmt.Errorf("listen api lokal: %w", err)
	}
	apiSrv := api.NewServerWithServices(lc, args.Observability, args.Filesystem)
	go func() { _ = apiSrv.Serve(lis) }()
	defer apiSrv.GracefulStop()
	if args.Tunnel != nil {
		tunnelCtx, cancelTunnel := context.WithCancel(ctx)
		defer cancelTunnel()
		go args.Tunnel.Watch(tunnelCtx)
		defer args.Tunnel.Close(context.Background())
	}

	since := time.Now()
	attempt := 0
	for {
		if err := runOnce(ctx, args, lc, since); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
		}
		attempt++
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(ComputeBackoff(attempt)):
		}
	}
}

func runOnce(ctx context.Context, args DaemonArgs, lc *orchestrator.Lifecycle, since time.Time) error {
	conn, err := DialPanel(ctx, args.PanelAddr, args.CertPath, args.KeyPath, args.CAPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	client := nodepb.NewNodeServiceClient(conn)
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errCh := make(chan error, 2)
	go func() { errCh <- RunHeartbeat(cctx, client, args.NodeID, DefaultHeartbeatInterval, since) }()
	go func() { errCh <- RunCommandStream(cctx, client, lc) }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}
