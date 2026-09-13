package observability

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type ServerMetric struct {
	ID, Status  string
	CPU         float64
	MemoryBytes int64
}
type TunnelMetric struct{ ServerID, Status string }
type SnapshotProvider interface {
	Snapshot(context.Context) ([]ServerMetric, []TunnelMetric, error)
}

type StaticSnapshotProvider struct {
	Servers []ServerMetric
	Tunnels []TunnelMetric
}

func (p StaticSnapshotProvider) Snapshot(context.Context) ([]ServerMetric, []TunnelMetric, error) {
	return append([]ServerMetric(nil), p.Servers...), append([]TunnelMetric(nil), p.Tunnels...), nil
}

type Metrics struct {
	reg      prometheus.Registerer
	provider SnapshotProvider
	mu       sync.Mutex
	servers  []ServerMetric
	tunnels  []TunnelMetric
	started  time.Time
	lastErr  error
}

func NewMetrics(reg prometheus.Registerer, provider SnapshotProvider) *Metrics {
	m := &Metrics{reg: reg, provider: provider, started: time.Now()}
	return m
}

func (m *Metrics) Refresh(ctx context.Context) error {
	servers, tunnels, err := m.provider.Snapshot(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		m.lastErr = err
		return fmt.Errorf("refresh metrics: %w", err)
	}
	m.servers, m.tunnels, m.lastErr = servers, tunnels, nil
	return nil
}

func (m *Metrics) Handler() http.Handler { return m.SnapshotHandler() }

// SnapshotHandler membangun collectors dari snapshot terakhir sehingga request
// metrics tidak pernah memanggil provider/Docker secara langsung.
func (m *Metrics) SnapshotHandler() http.Handler {
	reg := prometheus.NewRegistry()
	m.mu.Lock()
	servers, tunnels := append([]ServerMetric(nil), m.servers...), append([]TunnelMetric(nil), m.tunnels...)
	m.mu.Unlock()
	serverGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "oikos_servers_total", Help: "Jumlah server berdasarkan status"}, []string{"status"})
	cpuGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "oikos_server_cpu_usage_percent", Help: "Penggunaan CPU per server"}, []string{"server_id"})
	memGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "oikos_server_memory_usage_bytes", Help: "Penggunaan memory per server"}, []string{"server_id"})
	tunnelGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "oikos_tunnel_status", Help: "Status tunnel"}, []string{"server_id", "status"})
	uptimeGauge := prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "oikos_daemon_uptime_seconds", Help: "Lama daemon berjalan"}, func() float64 { return time.Since(m.started).Seconds() })
	for _, s := range servers {
		serverGauge.WithLabelValues(s.Status).Inc()
		cpuGauge.WithLabelValues(s.ID).Set(s.CPU)
		memGauge.WithLabelValues(s.ID).Set(float64(s.MemoryBytes))
	}
	for _, t := range tunnels {
		tunnelGauge.WithLabelValues(t.ServerID, t.Status).Set(1)
	}
	reg.MustRegister(serverGauge, cpuGauge, memGauge, tunnelGauge, uptimeGauge)
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
}
