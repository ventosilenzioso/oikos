package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
func (h concreteHostAdapter) HealthCheck() error               { return h.host.HealthCheck() }
func (h concreteHostAdapter) Routes() []string                 { return h.host.Routes() }
func (h concreteHostAdapter) Status() HostStatus               { return h.host.Status() }
func (h concreteHostAdapter) RouteHandler(string) http.Handler { return http.NotFoundHandler() }

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
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range plugins {
		if !p.Enabled {
			continue
		}
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
		r.hosts[p.ID], r.manifests[p.ID], r.enabled[p.ID] = concreteHostAdapter{host}, manifest, true
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
	for id, host := range hosts {
		if err := host.Start(); err != nil {
			r.mu.Lock()
			r.started = false
			r.mu.Unlock()
			return fmt.Errorf("start plugin %q: %w", id, err)
		}
		if provider, ok := host.(routeProvider); ok {
			for _, route := range r.manifests[id].AllowedRoutes {
				if err := r.proxy.Register(id, route, provider.RouteHandler(route)); err != nil {
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

func hasEnabledFlag(enabled map[string]bool, id string) bool {
	_, ok := enabled[id]
	return ok
}

func (r *Registry) StopAll() error {
	r.mu.Lock()
	cancels := r.cancels
	r.cancels = nil
	hosts := make([]hostLifecycle, 0, len(r.hosts))
	for _, host := range r.hosts {
		hosts = append(hosts, host)
	}
	r.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	r.bridgeWG.Wait()
	r.dispatchWG.Wait()
	for _, host := range hosts {
		if err := host.Stop(); err != nil {
			r.mu.Lock()
			r.started = false
			r.mu.Unlock()
			return err
		}
	}
	r.mu.Lock()
	r.started = false
	r.mu.Unlock()
	return nil
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
