package orchestrator

import "time"

type RestartPolicy struct {
	MaxRetries       int
	BackoffBase      time.Duration
	BackoffMax       time.Duration
	StableResetAfter time.Duration
}

var DefaultRestartPolicy = RestartPolicy{MaxRetries: 5, BackoffBase: 5 * time.Second, BackoffMax: 5 * time.Minute, StableResetAfter: 5 * time.Minute}

func (p RestartPolicy) NextBackoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	d := p.BackoffBase
	for i := 0; i < attempt && d < p.BackoffMax; i++ {
		d *= 2
	}
	if d > p.BackoffMax {
		return p.BackoffMax
	}
	return d
}
