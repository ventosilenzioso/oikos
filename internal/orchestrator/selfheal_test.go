package orchestrator

import (
	"context"
	"github.com/oikos/oikos/internal/store"
	"testing"
	"time"
)

type healerLifecycle struct{ restarts int }

func (l *healerLifecycle) RestartServer(context.Context, string) error { l.restarts++; return nil }

type healerStore struct {
	server     store.Server
	increments int
	status     string
	events     int
}

func (s *healerStore) GetServer(string) (store.Server, error) { return s.server, nil }
func (s *healerStore) IncrementRestartCount(string) error {
	s.increments++
	s.server.RestartCount++
	return nil
}
func (s *healerStore) ResetRestartCount(string) error            { s.server.RestartCount = 0; return nil }
func (s *healerStore) UpdateServerStatus(_, status string) error { s.status = status; return nil }
func (s *healerStore) SaveEvent(store.Event) error               { s.events++; return nil }

func TestSelfHealerStopsAtMaxRetries(t *testing.T) {
	bus := NewEventBus()
	life := &healerLifecycle{}
	db := &healerStore{server: store.Server{ID: "s1", RestartCount: 2}}
	h := NewSelfHealer(bus, life, db, RestartPolicy{MaxRetries: 2, BackoffBase: 0, BackoffMax: 0})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Start(ctx)
	time.Sleep(10 * time.Millisecond)
	bus.Publish(Event{Type: "server.crashed", ServerID: "s1"})
	deadline := time.Now().Add(time.Second)
	for db.status != "crash_looping" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if db.status != "crash_looping" || life.restarts != 0 {
		t.Fatalf("status=%s restarts=%d", db.status, life.restarts)
	}
}

func TestManualStopDoesNotRestart(t *testing.T) {
	bus := NewEventBus()
	life := &healerLifecycle{}
	db := &healerStore{server: store.Server{ID: "s1"}}
	h := NewSelfHealer(bus, life, db, RestartPolicy{MaxRetries: 2, BackoffBase: 0, BackoffMax: 0})
	h.MarkManualStop("s1")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Start(ctx)
	time.Sleep(10 * time.Millisecond)
	bus.Publish(Event{Type: "server.crashed", ServerID: "s1"})
	time.Sleep(50 * time.Millisecond)
	if life.restarts != 0 {
		t.Fatal(life.restarts)
	}
}
