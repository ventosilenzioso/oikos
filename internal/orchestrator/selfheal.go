package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/oikos/oikos/internal/store"
)

type restartLifecycle interface {
	RestartServer(context.Context, string) error
}
type restartStore interface {
	GetServer(string) (store.Server, error)
	IncrementRestartCount(string) error
	ResetRestartCount(string) error
	UpdateServerStatus(string, string) error
	SaveEvent(store.Event) error
}

type SelfHealer struct {
	bus       *EventBus
	lifecycle restartLifecycle
	db        restartStore
	policy    RestartPolicy
	mu        sync.Mutex
	manual    map[string]bool
}

func NewSelfHealer(bus *EventBus, lifecycle restartLifecycle, db restartStore, policy RestartPolicy) *SelfHealer {
	return &SelfHealer{bus: bus, lifecycle: lifecycle, db: db, policy: policy, manual: map[string]bool{}}
}

func (h *SelfHealer) MarkManualStop(serverID string) {
	h.mu.Lock()
	h.manual[serverID] = true
	h.mu.Unlock()
}

func (h *SelfHealer) Start(ctx context.Context) {
	events, cancel := h.bus.SubscribeWithCancel("server.crashed")
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			go h.handle(ctx, ev.ServerID)
		}
	}
}

func (h *SelfHealer) handle(ctx context.Context, serverID string) {
	h.mu.Lock()
	manual := h.manual[serverID]
	delete(h.manual, serverID)
	h.mu.Unlock()
	if manual {
		return
	}
	s, err := h.db.GetServer(serverID)
	if err != nil {
		return
	}
	if s.RestartCount >= h.policy.MaxRetries {
		_ = h.db.UpdateServerStatus(serverID, "crash_looping")
		_ = h.db.SaveEvent(store.Event{ID: fmt.Sprintf("recovery-failed-%d", time.Now().UnixNano()), Type: "server.recovery_failed", ServerID: serverID, Severity: "error", Message: "restart limit reached", Metadata: "{}", CreatedAt: time.Now().UTC()})
		return
	}
	select {
	case <-ctx.Done():
		return
	case <-time.After(h.policy.NextBackoff(s.RestartCount)):
	}
	if err := h.lifecycle.RestartServer(ctx, serverID); err != nil {
		return
	}
	_ = h.db.IncrementRestartCount(serverID)
	_ = h.db.SaveEvent(store.Event{ID: fmt.Sprintf("recovered-%d", time.Now().UnixNano()), Type: "server.recovered", ServerID: serverID, Severity: "info", Message: "server restarted", Metadata: "{}", CreatedAt: time.Now().UTC()})
}
