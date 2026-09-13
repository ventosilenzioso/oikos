package observability

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
)

func TestHealthStatusMatrix(t *testing.T) {
	okProbe := func(context.Context) error { return nil }
	failProbe := func(context.Context) error { return errors.New("down") }
	cases := []struct {
		name   string
		probes map[string]Probe
		want   HealthStatus
		code   int
	}{
		{"all", map[string]Probe{"sqlite": okProbe, "docker": okProbe, "panel": okProbe}, Healthy, 200},
		{"panel", map[string]Probe{"sqlite": okProbe, "docker": okProbe, "panel": failProbe}, Degraded, 200},
		{"docker", map[string]Probe{"sqlite": okProbe, "docker": failProbe, "panel": okProbe}, Unhealthy, 503},
	}
	for _, tc := range cases {
		h := NewHealthChecker(tc.probes)
		report := h.Check(context.Background())
		if report.Status != tc.want {
			t.Fatalf("%s=%s", tc.name, report.Status)
		}
		rec := httptest.NewRecorder()
		HealthHandler(h).ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
		if rec.Code != tc.code {
			t.Fatalf("%s code=%d", tc.name, rec.Code)
		}
	}
}
