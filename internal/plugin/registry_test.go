package plugin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/oikos/oikos/internal/orchestrator"
	"github.com/oikos/oikos/internal/store"
)

type registryHost struct {
	manifest Manifest
	status   HostStatus
	events   []Event
	err      error
	started  bool
	stopped  bool
	mu       sync.Mutex
}

type routeRegistryHost struct {
	*registryHost
	route http.Handler
}

func (h *routeRegistryHost) RouteHandler(string) http.Handler { return h.route }

func (h *registryHost) Start() error { h.started = true; return nil }
func (h *registryHost) Stop() error  { h.stopped = true; return nil }
func (h *registryHost) HandleEvent(ctx context.Context, event Event) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	h.mu.Lock()
	h.events = append(h.events, event)
	h.mu.Unlock()
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
	disabled.mu.Lock()
	disabledEvents := len(disabled.events)
	disabled.mu.Unlock()
	if disabledEvents != 0 {
		t.Fatalf("disabled plugin received events: %d", disabledEvents)
	}
	if err := r.StopAll(); err != nil {
		t.Fatal(err)
	}
	bus.Publish(orchestrator.Event{Type: "server.started", ServerID: "s2"})
	time.Sleep(20 * time.Millisecond)
	allowed.mu.Lock()
	allowedEvents := len(allowed.events)
	allowed.mu.Unlock()
	if allowedEvents != 1 {
		t.Fatalf("event bridge remained active: %d", allowedEvents)
	}
}

func TestDummyPluginReceivesEventFromEventBusEndToEnd(t *testing.T) {
	bus := orchestrator.NewEventBus()
	dummy := &registryHost{manifest: Manifest{ID: "dummy", AllowedEvents: []string{"server.crashed"}}, status: HostStatus{Healthy: true, Running: true}}
	registry := newTestRegistry(bus, dummy)
	if err := registry.StartAll(); err != nil {
		t.Fatal(err)
	}
	defer registry.StopAll()

	bus.Publish(orchestrator.Event{Type: "server.crashed", ServerID: "server-1"})
	waitForEvents(t, dummy, 1)
	dummy.mu.Lock()
	defer dummy.mu.Unlock()
	if len(dummy.events) != 1 || dummy.events[0].Type != "server.crashed" || dummy.events[0].ServerID != "server-1" {
		t.Fatalf("dummy plugin events=%+v", dummy.events)
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
	host.mu.Lock()
	events := len(host.events)
	host.mu.Unlock()
	if events != 3 {
		t.Fatalf("events after disable = %d, want 3", events)
	}
}

func TestRegistryDisablesPluginPersistentlyAfterThreeFailures(t *testing.T) {
	db := openPluginDB(t)
	if err := db.SavePlugin(store.Plugin{ID: "p", Name: "P", Version: "1", BinaryPath: "/p", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Enabled: true, InstalledAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	host := &registryHost{manifest: Manifest{ID: "p"}, err: errors.New("boom")}
	r := NewRegistry(db, orchestrator.NewEventBus(), HostOptions{EventTimeout: time.Second})
	r.hosts["p"] = hostAdapter{host}
	r.enabled["p"] = true
	for i := 0; i < 3; i++ {
		if err := r.DispatchEvent("p", Event{Type: "event"}); err == nil {
			t.Fatal("failure was swallowed")
		}
	}
	stored, err := db.GetPlugin("p")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Enabled {
		t.Fatal("plugin remained enabled in database")
	}
}

func TestRegistryLoadEnabledPersistsManifestAndRegistersNegotiatedRoutes(t *testing.T) {
	db := openPluginDB(t)
	path := fakeExecutable(t)
	if err := db.SavePlugin(store.Plugin{ID: "p", Name: "P", Version: "1", BinaryPath: path, ConfigPath: "/etc/p.yaml", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Enabled: true, SubscribedEvents: `[]`, AllowedRoutes: `["/plugins/p/status"]`, InstalledAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	r := NewRegistry(db, orchestrator.NewEventBus(), HostOptions{SocketRoot: t.TempDir()})
	if err := r.LoadEnabled(); err != nil {
		t.Fatal(err)
	}
	if got := r.manifests["p"]; got.Config != "/etc/p.yaml" || got.SHA256 == "" || len(got.AllowedRoutes) != 1 {
		t.Fatalf("loaded manifest = %+v", got)
	}
}

func TestRegistryLoadEnabledRemovesAndStopsStalePlugins(t *testing.T) {
	bus := orchestrator.NewEventBus()
	host := &registryHost{manifest: Manifest{ID: "stale"}}
	r := newTestRegistry(bus, host)
	p := r.Proxy()
	if err := p.Register("stale", "/plugins/stale/status", http.NotFoundHandler()); err != nil {
		t.Fatal(err)
	}
	db := openPluginDB(t)
	r.db = db
	if err := r.LoadEnabled(); err != nil {
		t.Fatal(err)
	}
	if !host.stopped {
		t.Fatal("stale host was not stopped")
	}
	if _, ok := r.hosts["stale"]; ok {
		t.Fatal("stale host remained in registry")
	}
	response := httptest.NewRecorder()
	p.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/plugins/stale/status", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("stale route status = %d", response.Code)
	}
}

func TestRegistryLoadEnabledErrorDoesNotLeaveMutexLocked(t *testing.T) {
	db := openPluginDB(t)
	if err := db.SavePlugin(store.Plugin{ID: "bad", Name: "Bad", Version: "1", BinaryPath: "/bad", SHA256: "bad", Enabled: true, SubscribedEvents: `[]`, InstalledAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	r := NewRegistry(db, orchestrator.NewEventBus(), HostOptions{})
	if err := r.LoadEnabled(); err == nil {
		t.Fatal("malformed plugin unexpectedly loaded")
	}
	done := make(chan struct{})
	go func() {
		r.mu.Lock()
		r.mu.Unlock()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("registry mutex remained locked after LoadEnabled error")
	}
}

func TestRegistryStopAllUnregistersRoutesAfterHostStopError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	good := &routeRegistryHost{registryHost: &registryHost{manifest: Manifest{ID: "good", AllowedRoutes: []string{"/plugins/good/status"}}}, route: handler}
	bad := &stoppingErrorHost{routeRegistryHost: &routeRegistryHost{registryHost: &registryHost{manifest: Manifest{ID: "bad", AllowedRoutes: []string{"/plugins/bad/status"}}}, route: handler}}
	r := newTestRegistry(orchestrator.NewEventBus(), good.registryHost, bad.registryHost)
	r.hosts["good"], r.hosts["bad"] = good, bad
	if err := r.StartAll(); err != nil {
		t.Fatal(err)
	}
	if err := r.StopAll(); err == nil {
		t.Fatal("StopAll unexpectedly succeeded")
	}
	for _, route := range []string{"/plugins/good/status", "/plugins/bad/status"} {
		response := httptest.NewRecorder()
		r.Proxy().Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, route, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("route %s remained registered with status %d", route, response.Code)
		}
	}
}

func TestRegistryStartAllRegistersNegotiatedRoutes(t *testing.T) {
	host := &routeRegistryHost{registryHost: &registryHost{manifest: Manifest{ID: "p", AllowedRoutes: []string{"/plugins/p/status"}}}, route: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })}
	r := newTestRegistry(orchestrator.NewEventBus(), host.registryHost)
	r.hosts["p"] = host
	if err := r.StartAll(); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	r.Proxy().Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/plugins/p/status", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("route status = %d", recorder.Code)
	}
	if err := r.StopAll(); err != nil {
		t.Fatal(err)
	}
}

func TestRegistryConcreteHostRegistersNegotiatedRouteHandlerAndRestarts(t *testing.T) {
	host := &Host{manifest: Manifest{ID: "p", AllowedRoutes: []string{"/plugins/p/status"}}, capability: Capability{Routes: []string{"/plugins/p/status"}}, options: HostOptions{RouteHandlers: map[string]http.Handler{"/plugins/p/status": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })}}}
	adapter := concreteHostAdapter{host: host}
	p := NewProxy()
	if err := p.Register("p", "/plugins/p/status", adapter.RouteHandler("/plugins/p/status")); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	p.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/plugins/p/status", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("route status = %d", response.Code)
	}
}

func TestRegistryStartAllRollsBackEarlierHostAndRoutes(t *testing.T) {
	good := &routeRegistryHost{registryHost: &registryHost{manifest: Manifest{ID: "a-good", AllowedRoutes: []string{"/plugins/a-good/status"}}}, route: http.NotFoundHandler()}
	bad := &failingStartHost{registryHost: &registryHost{manifest: Manifest{ID: "z-bad", AllowedRoutes: []string{"/plugins/z-bad/status"}}}}
	r := newTestRegistry(orchestrator.NewEventBus(), good.registryHost, bad.registryHost)
	r.hosts["good"], r.hosts["bad"] = good, bad
	if err := r.StartAll(); err == nil {
		t.Fatal("StartAll unexpectedly succeeded")
	}
	if good.stopped == false {
		t.Fatal("started host was not rolled back")
	}
	response := httptest.NewRecorder()
	r.Proxy().Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/plugins/a-good/status", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("rolled-back route status = %d", response.Code)
	}
}

type failingStartHost struct{ *registryHost }

func (*failingStartHost) Start() error { return errors.New("start failed") }

type stoppingErrorHost struct{ *routeRegistryHost }

func (*stoppingErrorHost) Stop() error { return errors.New("stop failed") }

func TestRegistryDispatchTimeoutUsesHostContext(t *testing.T) {
	host := &registryHost{manifest: Manifest{ID: "p"}, status: HostStatus{Healthy: true, Running: true}}
	r := newTestRegistry(orchestrator.NewEventBus(), host)
	r.options.EventTimeout = 10 * time.Millisecond
	started := make(chan struct{})
	release := make(chan struct{})
	host.err = nil
	blocking := &blockingHost{registryHost: host, started: started, release: release}
	r.hosts["p"] = blocking
	done := make(chan error, 1)
	go func() { done <- r.DispatchEvent("p", Event{Type: "event"}) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}
	if err := <-done; err == nil {
		t.Fatal("timeout did not return an error")
	}
	close(release)
}

type blockingHost struct {
	*registryHost
	started chan<- struct{}
	release <-chan struct{}
}

func (h *blockingHost) HandleEvent(ctx context.Context, event Event) error {
	close(h.started)
	select {
	case <-h.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
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
	for _, route := range []string{"/plugins/p/../admin", "/plugins/p/status"} {
		if err := p.Register("p", route, http.NotFoundHandler()); err == nil {
			t.Fatalf("route %q unexpectedly registered", route)
		}
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

func openPluginDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(t.TempDir() + "/plugins.db")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func fakeExecutable(t *testing.T) string {
	t.Helper()
	path := t.TempDir() + "/plugin"
	if err := os.WriteFile(path, []byte("plugin"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func waitForEvents(t *testing.T, host *registryHost, count int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		host.mu.Lock()
		done := len(host.events) >= count
		host.mu.Unlock()
		if done {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d events", count)
}
