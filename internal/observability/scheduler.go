package observability

import (
	"context"
	"time"
)

type Scheduler struct {
	Metrics         *Metrics
	Health          *HealthChecker
	MetricsInterval time.Duration
	HealthInterval  time.Duration
}

func (s *Scheduler) Run(ctx context.Context) error {
	if s.MetricsInterval <= 0 {
		s.MetricsInterval = 15 * time.Second
	}
	if s.HealthInterval <= 0 {
		s.HealthInterval = 15 * time.Second
	}
	metricsTicker := time.NewTicker(s.MetricsInterval)
	healthTicker := time.NewTicker(s.HealthInterval)
	defer metricsTicker.Stop()
	defer healthTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-metricsTicker.C:
			if s.Metrics != nil {
				_ = s.Metrics.Refresh(ctx)
			}
		case <-healthTicker.C:
			if s.Health != nil {
				_ = s.Health.Check(ctx)
			}
		}
	}
}
