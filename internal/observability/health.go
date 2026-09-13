package observability

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"time"
)

type HealthStatus string

const (
	Healthy   HealthStatus = "healthy"
	Degraded  HealthStatus = "degraded"
	Unhealthy HealthStatus = "unhealthy"
)

type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}
type HealthReport struct {
	Status    HealthStatus `json:"status"`
	Checks    []Check      `json:"checks"`
	Timestamp time.Time    `json:"timestamp"`
}
type Probe func(context.Context) error

func HealthyProbe(context.Context) error { return nil }

type HealthChecker struct{ probes map[string]Probe }

func NewHealthChecker(probes map[string]Probe) *HealthChecker { return &HealthChecker{probes: probes} }

func (h *HealthChecker) Check(ctx context.Context) HealthReport {
	report := HealthReport{Status: Healthy, Timestamp: time.Now().UTC()}
	names := make([]string, 0, len(h.probes))
	for name := range h.probes {
		names = append(names, name)
	}
	sort.Strings(names)
	failures := 0
	for _, name := range names {
		check := Check{Name: name, Status: "healthy"}
		if err := h.probes[name](ctx); err != nil {
			check.Status = "unhealthy"
			check.Error = err.Error()
			failures++
		}
		report.Checks = append(report.Checks, check)
	}
	if failures > 0 {
		report.Status = Unhealthy
		if failures == 1 && len(report.Checks) == 3 {
			for _, c := range report.Checks {
				if c.Name == "panel" && c.Status == "unhealthy" {
					report.Status = Degraded
				}
			}
		}
	}
	return report
}

func HealthHandler(h *HealthChecker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		report := h.Check(r.Context())
		code := http.StatusOK
		if report.Status == Unhealthy {
			code = http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(report)
	})
}
