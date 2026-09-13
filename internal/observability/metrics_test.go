package observability

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"net/http/httptest"
	"strings"
	"testing"
)

type metricProvider struct{ fail bool }

func (p metricProvider) Snapshot(context.Context) ([]ServerMetric, []TunnelMetric, error) {
	if p.fail {
		return nil, nil, context.DeadlineExceeded
	}
	return []ServerMetric{{ID: "s1", Status: "running", CPU: 12.5, MemoryBytes: 1024}}, []TunnelMetric{{ServerID: "s1", Status: "connected"}}, nil
}

func TestMetricsExposeServerAndDaemon(t *testing.T) {
	m := NewMetrics(prometheus.NewRegistry(), metricProvider{})
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	m.SnapshotHandler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body := rec.Body.String()
	for _, needle := range []string{"oikos_servers_total", `status="running"`, "oikos_server_cpu_usage_percent", "oikos_daemon_uptime_seconds"} {
		if !strings.Contains(body, needle) {
			t.Fatalf("missing %s: %s", needle, body)
		}
	}
}

func TestMetricsRefreshErrorKeepsHttpAlive(t *testing.T) {
	m := NewMetrics(prometheus.NewRegistry(), metricProvider{fail: true})
	if err := m.Refresh(context.Background()); err == nil {
		t.Fatal("refresh harus melaporkan error")
	}
	rec := httptest.NewRecorder()
	m.SnapshotHandler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	if rec.Code != 200 {
		t.Fatal(rec.Code)
	}
}

func TestMetricsExposeTunnelStatus(t *testing.T) {
	m := NewMetrics(prometheus.NewRegistry(), metricProvider{})
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	if !strings.Contains(rec.Body.String(), "oikos_tunnel_status") {
		t.Fatal("tunnel metric missing")
	}
}
