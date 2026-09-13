package orchestrator

import (
	"context"
	"fmt"
	"github.com/oikos/oikos/internal/store"
	"sync/atomic"
)

type EventWriter interface{ SaveEvent(store.Event) error }

type EventRecorder struct {
	bus    *EventBus
	db     EventWriter
	events <-chan Event
	cancel func()
}

func NewEventRecorder(bus *EventBus, db EventWriter) *EventRecorder {
	ch, cancel := bus.SubscribeWithCancel("*")
	return &EventRecorder{bus: bus, db: db, events: ch, cancel: cancel}
}

func (r *EventRecorder) Start(ctx context.Context) {
	defer r.cancel()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-r.events:
			if !ok {
				return
			}
			_ = r.db.SaveEvent(store.Event{ID: newEventID(), Type: ev.Type, ServerID: ev.ServerID, Severity: "info", Message: ev.Type, Metadata: "{}"})
		}
	}
}

func (r *EventRecorder) Stop() { r.cancel() }

var eventSequence uint64

func newEventID() string { return fmt.Sprintf("event-%d", atomic.AddUint64(&eventSequence, 1)) }
