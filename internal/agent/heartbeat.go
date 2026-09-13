package agent

import (
	"context"
	"time"

	nodepb "github.com/oikos/oikos/gen/go/node"
)

// DefaultHeartbeatInterval sesuai rencana Fase 1 (15 detik).
const DefaultHeartbeatInterval = 15 * time.Second

// RunHeartbeat mengirim heartbeat periodik hingga ctx selesai.
func RunHeartbeat(ctx context.Context, client nodepb.NodeServiceClient, nodeID string, interval time.Duration, since time.Time) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	fails := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case now := <-ticker.C:
			callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			_, err := client.Heartbeat(callCtx, &nodepb.HeartbeatRequest{
				NodeId:        nodeID,
				UptimeSeconds: int64(now.Sub(since).Seconds()),
			})
			cancel()
			if err != nil {
				fails++
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(ComputeBackoff(fails)):
				}
				continue
			}
			fails = 0
		}
	}
}
