package observability

import (
	"net/http"

	"github.com/oikos/oikos/internal/config"
)

func NewHTTPServer(cfg config.ObservabilityConfig, metrics http.Handler, health http.Handler) *http.Server {
	mux := http.NewServeMux()
	mux.Handle(cfg.MetricsPath, metrics)
	mux.Handle(cfg.HealthPath, health)
	return &http.Server{Addr: cfg.BindAddr, Handler: mux}
}
