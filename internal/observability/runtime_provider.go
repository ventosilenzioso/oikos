package observability

import (
	"context"
	"github.com/oikos/oikos/internal/runtime"
	"github.com/oikos/oikos/internal/store"
)

type RuntimeSnapshotProvider struct {
	DB      *store.DB
	Runtime runtime.Runtime
}

func (p RuntimeSnapshotProvider) Snapshot(ctx context.Context) ([]ServerMetric, []TunnelMetric, error) {
	servers, err := p.DB.ListServers()
	if err != nil {
		return nil, nil, err
	}
	out := make([]ServerMetric, 0, len(servers))
	for _, s := range servers {
		metric := ServerMetric{ID: s.ID, Status: s.Status}
		if s.ContainerID != "" && p.Runtime != nil {
			stats, statErr := p.Runtime.Stats(ctx, s.ContainerID)
			if statErr != nil {
				return nil, nil, statErr
			}
			metric.CPU = stats.CPUPercent
			metric.MemoryBytes = stats.MemoryUsedMB * 1024 * 1024
		}
		out = append(out, metric)
	}
	tunnelRows, err := p.DB.ListTunnelsForMetrics()
	if err != nil {
		return nil, nil, err
	}
	tunnels := make([]TunnelMetric, 0, len(tunnelRows))
	for _, row := range tunnelRows {
		tunnels = append(tunnels, TunnelMetric{ServerID: row.ServerID, Status: row.Status})
	}
	return out, tunnels, nil
}
