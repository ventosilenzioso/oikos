package plugin

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/oikos/oikos/internal/orchestrator"
)

type registryHost struct {
	manifest Manifest
	status   HostStatus
	events   []Event
	err      error
}

func (h *registryHost) Start() error { return nil }
func (h *registryHost) Stop() error  { return nil }
func (h *registryHost) HandleEvent(event Event) error {
	h.events = append(h.events, event)
	return h.err
}
func (h *registryHost) HealthCheck() error { return nil }
func (h *registryHost) Routes() []string   { return h.manifest.AllowedRoutes }
func (h *registryHost) Status() HostStatus { return h.status }

func TestRegistryDispatchesOnlySubscribedEventsAndStopsBridge(t *testing.T) {
	bus := orchestrator.NewEventBus()
	allowed := &registryHost{manifest: Manifest{ID: "allowed", AllowedEvents: []string{"server.started"}}, status: HostStatus{Healthy: true, Running: true}}
	disabled := &registryHost{manifest: Manifest{ID: "disabled", AllowedEvents: []string{"server.started"}}, status: HostStatus{Healthy: true, Running: true}}
	r := newTestRegistry(bus, allowed, disabled)
	r.enabled["disabled"] = false
	if err := r.StartAll(); err != nil {
		t.Fatal(err)
	}
	bus.Publish(orchestrator.Event{Type: "server.started", ServerID: "s1"})
	bus.Publish(orchestrator.Event{Type: "server.stopped", ServerID: "s1"})
	waitForEvents(t, allowed, 1)
	if len(disabled.events) != 0 {
		t.Fatalf("disabled plugin received events: %+v", disabled.events)
	}
	if err := r.StopAll(); err != nil {
		t.Fatal(err)
	}
	bus.Publish(orchestrator.Event{Type: "server.started", ServerID: "s2"})
	time.Sleep(20 * time.Millisecond)
	if len(allowed.events) != 1 {
		t.Fatalf("event bridge remained active: %+v", allowed.events)
	}
}

func TestRegistryDisablesPluginAfterThreeDispatchFailures(t *testing.T) {
	bus := orchestrator.NewEventBus()
	host := &registryHost{manifest: Manifest{ID: "failing", AllowedEvents: []string{"server.crashed"}}, status: HostStatus{Healthy: true, Running: true}, err: errors.New("boom")}
	r := newTestRegistry(bus, host)
	if err := r.StartAll(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		bus.Publish(orchestrator.Event{Type: "server.crashed"})
	}
	waitForEvents(t, host, 3)
	if len(host.events) != 3 {
		t.Fatalf("events after disable = %d, want 3", len(host.events))
	}
}

func TestProxyRejectsRoutesOutsidePluginNamespace(t *testing.T) {
	p := NewProxy()
	if err := p.Register("p", "/plugins/p/status", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })); err != nil {
		t.Fatal(err)
	}
	if err := p.Register("p", "/admin", http.NotFoundHandler()); err == nil {
		t.Fatal("global route registration succeeded")
	}
	request := httptest.NewRequest(http.MethodGet, "/plugins/p/status", nil)
	response := httptest.NewRecorder()
	p.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("plugin route status = %d, want 200", response.Code)
	}
	request = httptest.NewRequest(http.MethodGet, "/admin", nil)
	response = httptest.NewRecorder()
	p.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("admin route status = %d, want 404", response.Code)
	}
}

func newTestRegistry(bus *orchestrator.EventBus, hosts ...*registryHost) *Registry {
	r := NewRegistry(nil, bus, HostOptions{EventTimeout: time.Second})
	for _, host := range hosts {
		r.hosts[host.manifest.ID] = hostAdapter{host}
		r.manifests[host.manifest.ID] = host.manifest
		r.enabled[host.manifest.ID] = true
	}
	return r
}

func waitForEvents(t *testing.T, host *registryHost, count int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for len(host.events) < count && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
}
