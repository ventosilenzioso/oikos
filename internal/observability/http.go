package observability

import (
	"net/http"

	"github.com/oikos/oikos/internal/config"
)

func NewHTTPServer(cfg config.ObservabilityConfig, metrics http.Handler, health http.Handler) *http.Server {
	return NewHTTPServerWithPlugins(cfg, metrics, health, nil)
}

func NewHTTPServerWithPlugins(cfg config.ObservabilityConfig, metrics http.Handler, health http.Handler, plugins http.Handler) *http.Server {
	mux := http.NewServeMux()
	mux.Handle(cfg.MetricsPath, metrics)
	mux.Handle(cfg.HealthPath, health)
	if plugins != nil {
		mux.Handle("/plugins/", plugins)
	}
	return &http.Server{Addr: cfg.BindAddr, Handler: mux}
}
