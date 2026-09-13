package tunnel

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type TunnelSelfHealer struct {
	manager Manager
	policy  func(int) time.Duration
	mu      sync.Mutex
	attempt int
	failed  bool
}

func NewTunnelSelfHealer(manager Manager, nextBackoff func(int) time.Duration) *TunnelSelfHealer {
	if nextBackoff == nil {
		nextBackoff = func(int) time.Duration { return time.Second }
	}
	return &TunnelSelfHealer{manager: manager, policy: nextBackoff}
}

func (h *TunnelSelfHealer) Start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := h.manager.Reload(ctx); err != nil {
			h.mu.Lock()
			h.attempt++
			attempt := h.attempt
			h.mu.Unlock()
			if attempt > 5 {
				h.mu.Lock()
				h.failed = true
				h.mu.Unlock()
				return fmt.Errorf("tunnel recovery berhenti setelah 5 percobaan: %w", err)
			}
			timer := time.NewTimer(h.policy(attempt))
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			continue
		}
		h.mu.Lock()
		h.attempt = 0
		h.mu.Unlock()
		return nil
	}
}
