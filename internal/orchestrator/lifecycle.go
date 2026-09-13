package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/oikos/oikos/internal/resource"
	"github.com/oikos/oikos/internal/runtime"
	"github.com/oikos/oikos/internal/store"
	"github.com/oikos/oikos/internal/tunnel"
)

// Lifecycle mengatur siklus hidup server: create/start/stop/restart/delete.
type Lifecycle struct {
	db      *store.DB
	rt      runtime.Runtime
	bus     *EventBus
	tunnels tunnel.Manager
	ports   PortProvider
}

func New(db *store.DB, rt runtime.Runtime, bus *EventBus) *Lifecycle {
	return NewWithTunnel(db, rt, bus, nil, nil)
}

// PortProvider memasok port yang didefinisikan egg. Lifecycle tidak membaca
// port dari environment server agar kontrak tunnel tetap eksplisit.
type PortProvider interface {
	Ports(context.Context, string) ([]tunnel.PortMapping, error)
}

type PortProviderFunc func(context.Context, string) ([]tunnel.PortMapping, error)

func (f PortProviderFunc) Ports(ctx context.Context, eggID string) ([]tunnel.PortMapping, error) {
	return f(ctx, eggID)
}

func NewWithTunnel(db *store.DB, rt runtime.Runtime, bus *EventBus, tunnels tunnel.Manager, ports PortProvider) *Lifecycle {
	return &Lifecycle{db: db, rt: rt, bus: bus, tunnels: tunnels, ports: ports}
}

func (l *Lifecycle) CreateServer(ctx context.Context, name, eggID, startup string, env map[string]string) (string, error) {
	envJSON, err := json.Marshal(env)
	if err != nil {
		return "", fmt.Errorf("encode env: %w", err)
	}
	id := uuid.NewString()
	if err := l.db.EnsureEgg(store.Egg{ID: eggID, Name: eggID}); err != nil {
		return "", fmt.Errorf("registrasi egg: %w", err)
	}
	if err := l.db.CreateServer(store.Server{
		ID: id, Name: name, EggID: eggID,
		Status: "installing", StartupCommand: startup, Environment: string(envJSON),
	}); err != nil {
		return "", fmt.Errorf("simpan server: %w", err)
	}
	if err := l.db.UpdateServerStatus(id, "stopped"); err != nil {
		return "", fmt.Errorf("update status: %w", err)
	}
	if l.tunnels != nil && l.ports != nil {
		mappings, err := l.ports.Ports(ctx, eggID)
		if err != nil {
			_ = l.db.DeleteServer(id)
			return "", fmt.Errorf("ambil port egg: %w", err)
		}
		for _, mapping := range mappings {
			mapping.ServerID = id
			if _, err := l.tunnels.RegisterPort(ctx, mapping); err != nil {
				_ = l.db.DeleteServer(id)
				return "", fmt.Errorf("register tunnel: %w", err)
			}
		}
	}
	l.bus.Publish(Event{Type: "server.created", ServerID: id})
	return id, nil
}

func (l *Lifecycle) StartServer(ctx context.Context, id string) error {
	s, err := getServer(l.db, id)
	if err != nil {
		return fmt.Errorf("ambil server: %w", err)
	}
	containerID := s.ContainerID
	lim, limErr := l.db.GetResourceLimits(id)
	hasLimits := limErr == nil && (lim.CPULimit > 0 || lim.MemoryLimitMB > 0 || lim.PIDLimit > 0)
	if containerID == "" {
		var env map[string]string
		if err := json.Unmarshal([]byte(s.Environment), &env); err != nil {
			return fmt.Errorf("decode env: %w", err)
		}
		spec := runtime.ContainerSpec{
			ServerID:    s.ID,
			Image:       "oikos/" + s.EggID,
			Command:     strings.Fields(s.StartupCommand),
			Env:         env,
			MountSource: "",
		}
		if hasLimits {
			spec.CPULimit = lim.CPULimit
			spec.MemoryLimit = lim.MemoryLimitMB * 1024 * 1024
			spec.PIDLimit = lim.PIDLimit
		}
		containerID, err = l.rt.Create(ctx, spec)
		if err != nil {
			return fmt.Errorf("runtime create: %w", err)
		}
		if err := l.db.SetServerContainer(id, containerID); err != nil {
			return fmt.Errorf("simpan container id: %w", err)
		}
	}
	if err := l.rt.Start(ctx, containerID); err != nil {
		return fmt.Errorf("runtime start: %w", err)
	}
	if err := setStatus(l.db, id, "running"); err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	if hasLimits && resource.DetectCgroupVersion() == "v2" {
		if cg, err := resource.FindContainerCgroup("/sys/fs/cgroup", containerID); err == nil {
			rlim := resource.Limits{CPUMillicores: lim.CPULimit, MemoryBytes: lim.MemoryLimitMB * 1024 * 1024, PIDMax: lim.PIDLimit}
			if err := resource.ApplyLimits(cg, rlim); err != nil {
				return fmt.Errorf("terapkan limit: %w", err)
			}
		}
	}
	l.bus.Publish(Event{Type: "server.started", ServerID: id})
	return nil
}

func (l *Lifecycle) StopServer(ctx context.Context, id string) error {
	s, err := getServer(l.db, id)
	if err != nil {
		return fmt.Errorf("ambil server: %w", err)
	}
	if s.ContainerID != "" {
		if err := l.rt.Stop(ctx, s.ContainerID, 10); err != nil {
			return fmt.Errorf("runtime stop: %w", err)
		}
	}
	if err := setStatus(l.db, id, "stopped"); err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	l.bus.Publish(Event{Type: "server.stopped", ServerID: id})
	return nil
}

func (l *Lifecycle) RestartServer(ctx context.Context, id string) error {
	s, err := getServer(l.db, id)
	if err != nil {
		return fmt.Errorf("ambil server: %w", err)
	}
	if s.ContainerID == "" {
		return fmt.Errorf("server %s belum punya container", id)
	}
	if err := l.rt.Restart(ctx, s.ContainerID); err != nil {
		return fmt.Errorf("runtime restart: %w", err)
	}
	if err := setStatus(l.db, id, "running"); err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	l.bus.Publish(Event{Type: "server.restarted", ServerID: id})
	return nil
}

// GetServer membaca satu server dari store.
func (l *Lifecycle) GetServer(id string) (store.Server, error) {
	return getServer(l.db, id)
}

// ListServers membaca semua server dari store.
func (l *Lifecycle) ListServers() ([]store.Server, error) {
	return l.db.ListServers()
}

func (l *Lifecycle) DeleteServer(ctx context.Context, id string) error {
	s, err := getServer(l.db, id)
	if err != nil {
		return fmt.Errorf("ambil server: %w", err)
	}
	if s.ContainerID != "" {
		if err := l.rt.Delete(ctx, s.ContainerID, true); err != nil {
			return fmt.Errorf("runtime delete: %w", err)
		}
	}
	if l.tunnels != nil {
		if err := l.tunnels.ReleaseServer(ctx, id); err != nil {
			return fmt.Errorf("release tunnel: %w", err)
		}
	}
	if err := l.db.DeleteServer(id); err != nil {
		return fmt.Errorf("hapus server: %w", err)
	}
	l.bus.Publish(Event{Type: "server.deleted", ServerID: id})
	return nil
}
