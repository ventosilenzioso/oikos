package plugin

import "context"

type Event struct {
	Type     string
	ServerID string
	Metadata map[string]string
}

type Capability struct {
	Events []string
	Routes []string
}

type InitRequest struct {
	OikosVersion string
	ConfigPath   string
	PluginID     string
}

type InitResponse struct {
	Capabilities Capability
}

type PluginClient interface {
	Init(context.Context, InitRequest) (InitResponse, error)
	HandleEvent(context.Context, Event) error
	HealthCheck(context.Context) error
	Close() error
}
