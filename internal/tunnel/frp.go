package tunnel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/oikos/oikos/internal/store"
)

type TunnelStore interface {
	SaveTunnel(store.Tunnel) error
	GetTunnelByMapping(string, int, string) (store.Tunnel, error)
	ListTunnels(string) ([]store.Tunnel, error)
	DeleteTunnelByMapping(string, int, string) error
	UpdateTunnelStatus(string, string) error
}

type tunnelEvent struct{ Type, ServerID, TunnelID string }
type eventPublisher interface{ Publish(any) }

type FrpManager struct {
	mu        sync.Mutex
	cfg       FRPConfig
	allocator PanelAllocator
	store     TunnelStore
	process   ProcessController
	bus       *eventBusAdapter
	closed    chan struct{}
	mappings  map[string]PortMapping
}

// eventBusAdapter avoids an import cycle while forwarding events to the event bus.
type eventBusAdapter struct{ publish func(string, string) }

func NewFrpManager(cfg FRPConfig, allocator PanelAllocator, st TunnelStore, process ProcessController, publish func(string, string)) *FrpManager {
	m := &FrpManager{cfg: cfg, allocator: allocator, store: st, process: process, closed: make(chan struct{}), mappings: map[string]PortMapping{}}
	if publish != nil {
		m.bus = &eventBusAdapter{publish: publish}
	}
	return m
}

func (m *FrpManager) ConfigPath() string { return m.cfg.ConfigPath }

func (m *FrpManager) RegisterPort(ctx context.Context, mapping PortMapping) (Assignment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if old, err := m.store.GetTunnelByMapping(mapping.ServerID, mapping.LocalPort, mapping.Protocol); err == nil && old.RemotePort > 0 {
		return Assignment{TunnelID: old.ID, RemotePort: old.RemotePort, Status: TunnelStatus(old.Status)}, nil
	}
	a, err := m.allocator.Register(ctx, mapping)
	if err != nil {
		return Assignment{}, err
	}
	t := store.Tunnel{ID: a.TunnelID, ServerID: mapping.ServerID, LocalPort: mapping.LocalPort, RemotePort: a.RemotePort, Protocol: mapping.Protocol, Status: string(a.Status)}
	if err := m.store.SaveTunnel(t); err != nil {
		return Assignment{}, fmt.Errorf("simpan tunnel: %w", err)
	}
	mapping.TunnelID = a.TunnelID
	mapping.RemotePort = a.RemotePort
	m.mappings[a.TunnelID] = mapping
	if err := m.rebuildLocked(ctx); err != nil {
		_ = m.store.UpdateTunnelStatus(t.ID, string(TunnelError))
		m.emit("tunnel.disconnected", t.ServerID)
		return Assignment{}, err
	}
	m.emit("tunnel.connected", t.ServerID)
	return a, nil
}

func (m *FrpManager) ReleasePort(ctx context.Context, serverID string, localPort int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tunnels, err := m.store.ListTunnels(serverID)
	if err != nil {
		return err
	}
	for _, t := range tunnels {
		if t.LocalPort != localPort {
			continue
		}
		if err := m.allocator.Release(ctx, PortMapping{TunnelID: t.ID, ServerID: t.ServerID, LocalPort: t.LocalPort, RemotePort: t.RemotePort, Protocol: t.Protocol}); err != nil {
			return err
		}
		if err := m.store.DeleteTunnelByMapping(serverID, localPort, t.Protocol); err != nil {
			return err
		}
		delete(m.mappings, t.ID)
		m.emit("tunnel.disconnected", serverID)
	}
	return m.rebuildLocked(ctx)
}

func (m *FrpManager) ReleaseServer(ctx context.Context, serverID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tunnels, err := m.store.ListTunnels(serverID)
	if err != nil {
		return err
	}
	for _, t := range tunnels {
		if err := m.allocator.Release(ctx, PortMapping{TunnelID: t.ID, ServerID: t.ServerID, LocalPort: t.LocalPort, RemotePort: t.RemotePort, Protocol: t.Protocol}); err != nil {
			return err
		}
		if err := m.store.DeleteTunnelByMapping(serverID, t.LocalPort, t.Protocol); err != nil {
			return err
		}
		delete(m.mappings, t.ID)
		m.emit("tunnel.disconnected", serverID)
	}
	return m.rebuildLocked(ctx)
}

func (m *FrpManager) Status(ctx context.Context, serverID string) ([]TunnelStatus, error) {
	tunnels, err := m.store.ListTunnels(serverID)
	if err != nil {
		return nil, err
	}
	out := make([]TunnelStatus, 0, len(tunnels))
	for _, t := range tunnels {
		out = append(out, TunnelStatus(t.Status))
	}
	return out, nil
}

func (m *FrpManager) Reload(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rebuildLocked(ctx)
}

func (m *FrpManager) rebuildLocked(ctx context.Context) error {
	mappings := make([]PortMapping, 0, len(m.mappings))
	for _, mapping := range m.mappings {
		mappings = append(mappings, mapping)
	}
	data, err := GenerateConfig(m.cfg, mappings)
	if err != nil {
		return err
	}
	if err := WriteConfigAtomic(m.cfg.ConfigPath, data); err != nil {
		return err
	}
	if len(mappings) == 0 {
		return m.process.Stop(ctx)
	}
	if len(mappings) == 1 {
		return m.process.Start(ctx, m.cfg.ConfigPath)
	}
	return m.process.Reload(ctx, m.cfg.ConfigPath)
}

func (m *FrpManager) emit(typ, serverID string) {
	if m.bus != nil {
		m.bus.publish(typ, serverID)
	}
}

func (m *FrpManager) Close(ctx context.Context) error {
	select {
	case <-m.closed:
	default:
		close(m.closed)
	}
	return m.process.Stop(ctx)
}

// Watch observes the frpc child and attempts a reload when it exits.
// Detailed retry policy and alerting remain the responsibility of Fase 3.
func (m *FrpManager) Watch(ctx context.Context) {
	attempt := 0
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-m.process.Wait():
			if !ok {
				return
			}
			m.emit("tunnel.disconnected", "")
			attempt++
			select {
			case <-ctx.Done():
				return
			case <-contextAfter(ctx, backoff(attempt)):
			}
			if err := m.Reload(ctx); err == nil {
				attempt = 0
			}
		}
	}
}

func backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := time.Second << min(attempt-1, 4)
	if d > 30*time.Second {
		return 30 * time.Second
	}
	return d
}

func contextAfter(ctx context.Context, d time.Duration) <-chan time.Time {
	t := time.NewTimer(d)
	go func() { <-ctx.Done(); t.Stop() }()
	return t.C
}
