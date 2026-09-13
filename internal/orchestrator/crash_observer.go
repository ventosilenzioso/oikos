package orchestrator

import (
	"context"
	"time"

	"github.com/oikos/oikos/internal/runtime"
)

// CrashObserver detects a running-to-stopped/crashed runtime transition.
// Manual stops are tracked separately and do not trigger the healer.
type CrashObserver struct {
	db       *Lifecycle
	runtime  runtime.Runtime
	bus      *EventBus
	interval time.Duration
}

func NewCrashObserver(lifecycle *Lifecycle, rt runtime.Runtime, bus *EventBus, interval time.Duration) *CrashObserver {
	if interval <= 0 {
		interval = time.Second
	}
	return &CrashObserver{db: lifecycle, runtime: rt, bus: bus, interval: interval}
}

func (o *CrashObserver) Run(ctx context.Context) error {
	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()
	previous := map[string]runtime.ContainerStatus{}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			servers, err := o.db.ListServers()
			if err != nil {
				continue
			}
			for _, server := range servers {
				if server.ContainerID == "" {
					continue
				}
				status, err := o.runtime.Status(ctx, server.ContainerID)
				if err != nil {
					continue
				}
				if previous[server.ID] == runtime.StatusRunning && status != runtime.StatusRunning && !o.db.IsManualStop(server.ID) {
					_ = o.db.setLastCrash(server.ID)
					o.bus.Publish(Event{Type: "server.crashed", ServerID: server.ID})
				}
				previous[server.ID] = status
			}
		}
	}
}
