package orchestrator

import (
	"testing"
	"time"
)

func TestRestartPolicyCapsBackoff(t *testing.T) {
	p := DefaultRestartPolicy
	if p.NextBackoff(0) != 5*time.Second || p.NextBackoff(1) != 10*time.Second || p.NextBackoff(99) != 5*time.Minute {
		t.Fatal("backoff salah")
	}
}
