package orchestrator

import "sync"

// Event is a domain message broadcast to subscribers.
type Event struct {
	Type     string
	ServerID string
}

// EventBus is a simple channel-based pub/sub implementation.
// Subscriber channels are buffered and slow consumers may drop events.
type EventBus struct {
	mu   sync.RWMutex
	subs map[string][]chan Event
}

func (b *EventBus) SubscribeWithCancel(typ string) (<-chan Event, func()) {
	ch, _, cancel := b.subscribeWithDone(typ)
	return ch, cancel
}

// SubscribeWithCancelDone is like SubscribeWithCancel and also exposes a
// signal for consumers that need to stop their receive loop on cancellation.
func (b *EventBus) SubscribeWithCancelDone(typ string) (<-chan Event, <-chan struct{}, func()) {
	return b.subscribeWithDone(typ)
}

func (b *EventBus) subscribeWithDone(typ string) (<-chan Event, <-chan struct{}, func()) {
	ch := make(chan Event, 64)
	done := make(chan struct{})
	b.mu.Lock()
	b.subs[typ] = append(b.subs[typ], ch)
	b.mu.Unlock()
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			for i, candidate := range b.subs[typ] {
				if candidate == ch {
					b.subs[typ] = append(b.subs[typ][:i], b.subs[typ][i+1:]...)
					break
				}
			}
			close(done)
		})
	}
	return ch, done, cancel
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
	subs := append([]chan Event(nil), b.subs[ev.Type]...)
	subs = append(subs, b.subs["*"]...)
	b.mu.RUnlock()
	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
		}
	}
}
