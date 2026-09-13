package orchestrator

import "sync"

// Event adalah pesan domain yang di-broadcast ke subscriber.
type Event struct {
	Type     string
	ServerID string
}

// EventBus adalah pub/sub sederhana berbasis channel.
// Channel subscriber berbuffer; Publish memblokir bila buffer penuh,
// jadi subscriber wajib terus membaca atau berhenti subscribe.
type EventBus struct {
	mu   sync.RWMutex
	subs map[string][]chan Event
}

func NewEventBus() *EventBus {
	return &EventBus{subs: map[string][]chan Event{}}
}

func (b *EventBus) Subscribe(typ string) <-chan Event {
	ch := make(chan Event, 64)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[typ] = append(b.subs[typ], ch)
	return ch
}

func (b *EventBus) Publish(ev Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subs[ev.Type] {
		ch <- ev
	}
}
