package agent

import (
	"context"
	"testing"
	"time"
)

func TestHeartbeatSendsPeriodically(t *testing.T) {
	panel := newMockPanel(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- RunHeartbeat(ctx, panel.mtlsClient(t), "node-1", 10*time.Millisecond, time.Now()) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if len(panel.heartbeats()) >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("heartbeat tidak terkirim")
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("err = %v, mau context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunHeartbeat tidak berhenti setelah cancel")
	}
	for _, id := range panel.heartbeats() {
		if id != "node-1" {
			t.Fatalf("node id = %q", id)
		}
	}
}
