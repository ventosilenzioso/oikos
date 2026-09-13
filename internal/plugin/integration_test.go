//go:build integration

package plugin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oikos/oikos/internal/orchestrator"
)

func TestPluginCrashDoesNotStopRegistryIntegration(t *testing.T) {
	bus := orchestrator.NewEventBus()
	host := &registryHost{manifest: Manifest{ID: "crashing", AllowedEvents: []string{"server.crashed"}}, status: HostStatus{Healthy: true, Running: true}, err: errors.New("intentional plugin crash")}
	r := newTestRegistry(bus, host)
	if err := r.StartAll(); err != nil {
		t.Fatal(err)
	}
	defer r.StopAll()
	for i := 0; i < 3; i++ {
		if err := r.DispatchEvent("crashing", Event{Type: "server.crashed", ServerID: "s"}); err == nil {
			t.Fatal("crash error swallowed")
		}
	}
	if err := r.DispatchEvent("crashing", Event{Type: "server.crashed", ServerID: "s"}); err != nil {
		t.Fatal("disabled plugin should be ignored")
	}
	if host.started == false {
		t.Fatal("registry did not start")
	}
}

func TestDisabledPluginIsNotLoadedIntegration(t *testing.T) {
	bus := orchestrator.NewEventBus()
	host := &registryHost{manifest: Manifest{ID: "disabled", AllowedEvents: []string{"server.crashed"}}, status: HostStatus{Healthy: true, Running: true}}
	r := newTestRegistry(bus, host)
	r.enabled["disabled"] = false
	if err := r.StartAll(); err != nil {
		t.Fatal(err)
	}
	defer r.StopAll()
	bus.Publish(orchestrator.Event{Type: "server.crashed", ServerID: "s"})
	time.Sleep(20 * time.Millisecond)
	host.mu.Lock()
	defer host.mu.Unlock()
	if len(host.events) != 0 {
		t.Fatalf("disabled events=%d", len(host.events))
	}
	_ = context.Background()
}
