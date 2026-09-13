package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/oikos/oikos/internal/orchestrator"
	"github.com/oikos/oikos/internal/store"
)

type hostLifecycle interface {
	Start() error
	Stop() error
	HandleEvent(Event) error
	HealthCheck() error
	Routes() []string
	Status() HostStatus
}

type hostAdapter struct{ hostLifecycle }

type Registry struct {
	db        *store.DB
	bus       *orchestrator.EventBus
	options   HostOptions
	hosts     map[string]hostLifecycle
	manifests map[string]Manifest
	enabled   map[string]bool
	failures  map[string]int
	cancels   []func()
	mu        sync.Mutex
}

func NewRegistry(db *store.DB, bus *orchestrator.EventBus, options HostOptions) *Registry {
	if bus == nil {
		bus = orchestrator.NewEventBus()
	}
	if options.EventTimeout <= 0 {
		options.EventTimeout = 5 * time.Second
	}
	return &Registry{db: db, bus: bus, options: options, hosts: map[string]hostLifecycle{}, manifests: map[string]Manifest{}, enabled: map[string]bool{}, failures: map[string]int{}}
}

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
		manifest := Manifest{ID: p.ID, Name: p.Name, Version: p.Version, Binary: p.BinaryPath, Enabled: true, AllowedEvents: events}
		host, err := NewHost(manifest, r.options)
		if err != nil {
			return err
		}
		r.hosts[p.ID], r.manifests[p.ID], r.enabled[p.ID] = host, manifest, true
	}
	return nil
}

func (r *Registry) StartAll() error {
	r.mu.Lock()
	hosts := make(map[string]hostLifecycle, len(r.hosts))
	for id, host := range r.hosts {
		if r.enabled[id] || !hasEnabledFlag(r.enabled, id) {
			hosts[id] = host
		}
	}
	r.mu.Unlock()
	for id, host := range hosts {
		if err := host.Start(); err != nil {
			return fmt.Errorf("start plugin %q: %w", id, err)
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
	for _, host := range hosts {
		if err := host.Stop(); err != nil {
			return err
		}
	}
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
			go func(id string, ch <-chan orchestrator.Event, done <-chan struct{}) {
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
	r.mu.Lock()
	host, enabled := r.hosts[id], r.enabled[id]
	if !hasEnabledFlag(r.enabled, id) {
		enabled = true
	}
	r.mu.Unlock()
	if host == nil || !enabled {
		return nil
	}
	result := make(chan error, 1)
	go func() { result <- host.HandleEvent(event) }()
	var err error
	select {
	case err = <-result:
	case <-time.After(r.options.EventTimeout):
		err = errors.New("plugin event timeout")
	}
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
