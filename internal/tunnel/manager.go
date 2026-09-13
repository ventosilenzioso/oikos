package tunnel

import "context"

type PortMapping struct {
	TunnelID   string
	ServerID   string
	LocalPort  int
	RemotePort int
	Protocol   string
}

type TunnelStatus string

const (
	TunnelPending      TunnelStatus = "pending"
	TunnelConnected    TunnelStatus = "connected"
	TunnelDisconnected TunnelStatus = "disconnected"
	TunnelError        TunnelStatus = "error"
)

type Assignment struct {
	TunnelID   string
	RemotePort int
	Status     TunnelStatus
}

type Manager interface {
	RegisterPort(context.Context, PortMapping) (Assignment, error)
	ReleasePort(context.Context, string, int) error
	ReleaseServer(context.Context, string) error
	Status(context.Context, string) ([]TunnelStatus, error)
	Reload(context.Context) error
	Watch(context.Context)
	Close(context.Context) error
}
