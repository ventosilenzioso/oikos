package orchestrator

import (
	"context"
	"github.com/oikos/oikos/internal/store"
	"sync"
	"testing"
	"time"
)

type healerLifecycle struct {
	mu       sync.Mutex
	restarts int
}

func (l *healerLifecycle) RestartServer(context.Context, string) error {
	l.mu.Lock()
	l.restarts++
	l.mu.Unlock()
	return nil
}
func (l *healerLifecycle) RestartCount() int { l.mu.Lock(); defer l.mu.Unlock(); return l.restarts }

type healerStore struct {
	mu         sync.Mutex
	server     store.Server
	increments int
	status     string
	events     int
}

func (s *healerStore) GetServer(string) (store.Server, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.server, nil
}
func (s *healerStore) IncrementRestartCount(string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.increments++
	s.server.RestartCount++
	return nil
}
func (s *healerStore) ResetRestartCount(string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.server.RestartCount = 0
	return nil
}
func (s *healerStore) UpdateServerStatus(_, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
	return nil
}
func (s *healerStore) SaveEvent(store.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events++
	return nil
}
func (s *healerStore) Status() string { s.mu.Lock(); defer s.mu.Unlock(); return s.status }

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
	for db.Status() != "crash_looping" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if db.Status() != "crash_looping" || life.RestartCount() != 0 {
		t.Fatalf("status=%s restarts=%d", db.Status(), life.RestartCount())
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
	if life.RestartCount() != 0 {
		t.Fatal(life.RestartCount())
	}
}
