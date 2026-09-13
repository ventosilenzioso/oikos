package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/oikos/oikos/internal/orchestrator"
	"github.com/oikos/oikos/internal/store"
)

type hostLifecycle interface {
	Start() error
	Stop() error
	HandleEvent(context.Context, Event) error
	HealthCheck() error
	Routes() []string
	Status() HostStatus
}

type hostAdapter struct{ hostLifecycle }

type concreteHostAdapter struct{ host *Host }

func (h concreteHostAdapter) Start() error { return h.host.Start() }
func (h concreteHostAdapter) Stop() error  { return h.host.Stop() }
func (h concreteHostAdapter) HandleEvent(ctx context.Context, event Event) error {
	return h.host.HandleEventContext(ctx, event)
}
func (h concreteHostAdapter) HealthCheck() error { return h.host.HealthCheck() }
func (h concreteHostAdapter) Routes() []string   { return h.host.Routes() }
func (h concreteHostAdapter) Status() HostStatus { return h.host.Status() }
func (h concreteHostAdapter) RouteHandler(route string) http.Handler {
	return h.host.RouteHandler(route)
}

type routeProvider interface{ RouteHandler(string) http.Handler }

type Registry struct {
	db         *store.DB
	bus        *orchestrator.EventBus
	options    HostOptions
	hosts      map[string]hostLifecycle
	manifests  map[string]Manifest
	enabled    map[string]bool
	failures   map[string]int
	cancels    []func()
	bridgeWG   sync.WaitGroup
	dispatchWG sync.WaitGroup
	proxy      *Proxy
	started    bool
	mu         sync.Mutex
}

func NewRegistry(db *store.DB, bus *orchestrator.EventBus, options HostOptions) *Registry {
	if bus == nil {
		bus = orchestrator.NewEventBus()
	}
	if options.EventTimeout <= 0 {
		options.EventTimeout = 5 * time.Second
	}
	return &Registry{db: db, bus: bus, options: options, hosts: map[string]hostLifecycle{}, manifests: map[string]Manifest{}, enabled: map[string]bool{}, failures: map[string]int{}, proxy: NewProxy()}
}

func (r *Registry) Proxy() *Proxy { return r.proxy }

func (r *Registry) LoadEnabled() error {
	if r.db == nil {
		return errors.New("plugin registry database is nil")
	}
	plugins, err := r.db.ListPlugins()
	if err != nil {
		return err
	}
	loaded := make(map[string]struct{}, len(plugins))
	loadedHosts := make(map[string]hostLifecycle)
	loadedManifests := make(map[string]Manifest)
	for _, p := range plugins {
		if !p.Enabled {
			continue
		}
		loaded[p.ID] = struct{}{}
		var events []string
		if err := json.Unmarshal([]byte(p.SubscribedEvents), &events); err != nil {
			return fmt.Errorf("plugin %q subscribed events: %w", p.ID, err)
		}
		var routes []string
		if err := json.Unmarshal([]byte(p.AllowedRoutes), &routes); err != nil {
			return fmt.Errorf("plugin %q allowed routes: %w", p.ID, err)
		}
		manifest := Manifest{ID: p.ID, Name: p.Name, Version: p.Version, Binary: p.BinaryPath, Config: p.ConfigPath, SHA256: p.SHA256, Enabled: true, AllowedEvents: events, AllowedRoutes: routes}
		host, err := NewHost(manifest, r.options)
		if err != nil {
			return err
		}
		loadedHosts[p.ID], loadedManifests[p.ID] = concreteHostAdapter{host}, manifest
	}
	r.mu.Lock()
	for id, host := range loadedHosts {
		r.hosts[id], r.manifests[id], r.enabled[id] = host, loadedManifests[id], true
	}
	stale := make(map[string]hostLifecycle)
	for id := range r.hosts {
		if _, ok := loaded[id]; !ok {
			stale[id] = r.hosts[id]
			delete(r.hosts, id)
			delete(r.manifests, id)
			delete(r.enabled, id)
			delete(r.failures, id)
		}
	}
	r.mu.Unlock()
	for id, host := range stale {
		_ = host.Stop()
		r.proxy.Unregister(id)
	}
	return nil
}

func (r *Registry) StartAll() error {
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return nil
	}
	r.started = true
	hosts := make(map[string]hostLifecycle, len(r.hosts))
	for id, host := range r.hosts {
		if r.enabled[id] || !hasEnabledFlag(r.enabled, id) {
			hosts[id] = host
		}
	}
	r.mu.Unlock()
	started := make([]string, 0, len(hosts))
	ids := make([]string, 0, len(hosts))
	for id := range hosts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		host := hosts[id]
		if err := host.Start(); err != nil {
			r.rollbackStarted(started)
			r.mu.Lock()
			r.started = false
			r.mu.Unlock()
			return fmt.Errorf("start plugin %q: %w", id, err)
		}
		started = append(started, id)
		if provider, ok := host.(routeProvider); ok {
			for _, route := range r.manifests[id].AllowedRoutes {
				handler := provider.RouteHandler(route)
				if err := r.proxy.Register(id, route, handler); err != nil {
					r.rollbackStarted(append(started, id))
					r.mu.Lock()
					r.started = false
					r.mu.Unlock()
					return fmt.Errorf("register plugin %q route %q: %w", id, route, err)
				}
			}
		}
	}
	r.bridge()
	return nil
}

func (r *Registry) rollbackStarted(ids []string) {
	for _, id := range ids {
		if host, ok := r.hosts[id]; ok {
			_ = host.Stop()
		}
		r.proxy.Unregister(id)
	}
}

func hasEnabledFlag(enabled map[string]bool, id string) bool {
	_, ok := enabled[id]
	return ok
}

func (r *Registry) StopAll() error {
	r.mu.Lock()
	cancels := r.cancels
	r.cancels = nil
	hosts := make(map[string]hostLifecycle, len(r.hosts))
	for id, host := range r.hosts {
		hosts[id] = host
	}
	r.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	r.bridgeWG.Wait()
	r.dispatchWG.Wait()
	var firstErr error
	for _, host := range hosts {
		if err := host.Stop(); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	for id := range hosts {
		r.proxy.Unregister(id)
	}
	r.mu.Lock()
	r.started = false
	r.mu.Unlock()
	return firstErr
}

func (r *Registry) bridge() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, manifest := range r.manifests {
		if !r.enabled[id] && hasEnabledFlag(r.enabled, id) {
			continue
		}
		for _, eventType := range manifest.AllowedEvents {
			ch, done, cancel := r.bus.SubscribeWithCancelDone(eventType)
			r.cancels = append(r.cancels, cancel)
			r.bridgeWG.Add(1)
			go func(id string, ch <-chan orchestrator.Event, done <-chan struct{}) {
				defer r.bridgeWG.Done()
				for {
					select {
					case <-done:
						return
					case event := <-ch:
						r.DispatchEvent(id, Event{Type: event.Type, ServerID: event.ServerID})
					}
				}
			}(id, ch, done)
		}
	}
}

func (r *Registry) DispatchEvent(id string, event Event) error {
	r.dispatchWG.Add(1)
	defer r.dispatchWG.Done()
	r.mu.Lock()
	host, enabled := r.hosts[id], r.enabled[id]
	if !hasEnabledFlag(r.enabled, id) {
		enabled = true
	}
	r.mu.Unlock()
	if host == nil || !enabled {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.options.EventTimeout)
	defer cancel()
	err := host.HandleEvent(ctx, event)
	if err == nil {
		r.mu.Lock()
		r.failures[id] = 0
		r.mu.Unlock()
		return nil
	}
	r.mu.Lock()
	r.failures[id]++
	failed := r.failures[id] >= 3
	if failed {
		r.enabled[id] = false
	}
	r.mu.Unlock()
	if failed && r.db != nil {
		_ = r.db.SetPluginEnabled(id, false)
	}
	return err
}

func (r *Registry) HealthCheckAll() map[string]error {
	r.mu.Lock()
	hosts := make(map[string]hostLifecycle, len(r.hosts))
	for id, host := range r.hosts {
		if r.enabled[id] || !hasEnabledFlag(r.enabled, id) {
			hosts[id] = host
		}
	}
	r.mu.Unlock()
	result := make(map[string]error, len(hosts))
	for id, host := range hosts {
		result[id] = host.HealthCheck()
	}
	return result
}
