package observability

import (
	"context"
	"testing"
	"time"
)

func TestSchedulerStopsOnContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Scheduler{MetricsInterval: time.Millisecond, HealthInterval: time.Millisecond}
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("scheduler tidak berhenti")
	}
}
