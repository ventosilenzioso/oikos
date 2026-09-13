package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	nodepb "github.com/oikos/oikos/gen/go/node"
	"github.com/oikos/oikos/internal/orchestrator"
)

// RunCommandStream menerima command Panel dan meneruskannya ke orchestrator.
// Command yang gagal diproses dilewati tanpa memutus stream; fungsi kembali
// hanya saat stream berakhir atau ctx selesai.
func RunCommandStream(ctx context.Context, client nodepb.NodeServiceClient, lc *orchestrator.Lifecycle) error {
	stream, err := client.StreamCommands(ctx, &nodepb.StreamRequest{})
	if err != nil {
		return fmt.Errorf("buka stream: %w", err)
	}
	for {
		cmd, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return fmt.Errorf("recv command: %w", err)
			}
		}
		if err := dispatch(ctx, lc, cmd); err != nil {
			continue
		}
	}
}

func dispatch(ctx context.Context, lc *orchestrator.Lifecycle, cmd *nodepb.NodeCommand) error {
	switch cmd.Action {
	case "create":
		var env map[string]string
		if raw := cmd.Args["environment"]; raw != "" {
			if err := json.Unmarshal([]byte(raw), &env); err != nil {
				return err
			}
		}
		_, err := lc.CreateServer(ctx, cmd.Args["name"], cmd.Args["egg_id"], cmd.Args["startup_command"], env)
		return err
	case "start":
		return lc.StartServer(ctx, cmd.ServerId)
	case "stop":
		return lc.StopServer(ctx, cmd.ServerId)
	case "delete":
		return lc.DeleteServer(ctx, cmd.ServerId)
	default:
		return fmt.Errorf("action tak dikenal: %s", cmd.Action)
	}
}
