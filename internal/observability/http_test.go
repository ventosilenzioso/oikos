package observability

import (
	"context"
	"net"
	"net/http"
	"testing"

	"github.com/oikos/oikos/internal/config"
	"github.com/prometheus/client_golang/prometheus"
)

func TestHTTPServerServesMetricsAndHealth(t *testing.T) {
	cfg := config.Default().Observability
	cfg.BindAddr = "127.0.0.1:0"
	m := NewMetrics(prometheus.NewRegistry(), metricProvider{})
	h := NewHealthChecker(map[string]Probe{"sqlite": func(context.Context) error { return nil }})
	srv := NewHTTPServer(cfg, m.Handler(), HealthHandler(h))
	lis, err := net.Listen("tcp", cfg.BindAddr)
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(lis)
	defer srv.Shutdown(context.Background())
	res, err := http.Get("http://" + lis.Addr().String() + cfg.HealthPath)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatal(res.StatusCode)
	}
}
