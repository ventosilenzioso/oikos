package plugin

import (
	"fmt"
	"net/http"
	"strings"
)

type Proxy struct{ routes map[string]http.Handler }

func NewProxy() *Proxy { return &Proxy{routes: map[string]http.Handler{}} }

func (p *Proxy) Register(pluginID, route string, handler http.Handler) error {
	prefix := "/plugins/" + pluginID + "/"
	if pluginID == "" || !strings.HasPrefix(route, prefix) || strings.Contains(route, "..") {
		return fmt.Errorf("invalid plugin route %q", route)
	}
	if handler == nil {
		return fmt.Errorf("plugin route handler is nil")
	}
	if _, exists := p.routes[route]; exists {
		return fmt.Errorf("plugin route already registered: %s", route)
	}
	p.routes[route] = handler
	return nil
}

func (p *Proxy) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handler, ok := p.routes[r.URL.Path]; ok {
			handler.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})
}
