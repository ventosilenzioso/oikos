package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/oikos/oikos/internal/runtime"
	"github.com/oikos/oikos/internal/store"
)

// Lifecycle mengatur siklus hidup server: create/start/stop/restart/delete.
type Lifecycle struct {
	db  *store.DB
	rt  runtime.Runtime
	bus *EventBus
}

func New(db *store.DB, rt runtime.Runtime, bus *EventBus) *Lifecycle {
	return &Lifecycle{db: db, rt: rt, bus: bus}
}

func (l *Lifecycle) CreateServer(ctx context.Context, name, eggID, startup string, env map[string]string) (string, error) {
	envJSON, err := json.Marshal(env)
	if err != nil {
		return "", fmt.Errorf("encode env: %w", err)
	}
	id := uuid.NewString()
	if err := l.db.CreateServer(store.Server{
		ID: id, Name: name, EggID: eggID,
		Status: "installing", StartupCommand: startup, Environment: string(envJSON),
	}); err != nil {
		return "", fmt.Errorf("simpan server: %w", err)
	}
	if err := l.db.UpdateServerStatus(id, "stopped"); err != nil {
		return "", fmt.Errorf("update status: %w", err)
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
	if containerID == "" {
		var env map[string]string
		if err := json.Unmarshal([]byte(s.Environment), &env); err != nil {
			return fmt.Errorf("decode env: %w", err)
		}
		containerID, err = l.rt.Create(ctx, runtime.ContainerSpec{
			ServerID:    s.ID,
			Image:       "oikos/" + s.EggID,
			Command:     strings.Fields(s.StartupCommand),
			Env:         env,
			MountSource: "",
		})
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
	if err := l.db.DeleteServer(id); err != nil {
		return fmt.Errorf("hapus server: %w", err)
	}
	l.bus.Publish(Event{Type: "server.deleted", ServerID: id})
	return nil
}
