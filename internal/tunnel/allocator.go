package tunnel

import (
	"context"
	"fmt"

	tunnelpb "github.com/oikos/oikos/gen/go/tunnel"
)

type PanelAllocator interface {
	Register(context.Context, PortMapping) (Assignment, error)
	Release(context.Context, PortMapping) error
}

type panelAllocator struct {
	client tunnelpb.TunnelServiceClient
	nodeID string
}

func validateLocalPort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("local port %d di luar range", port)
	}
	return nil
}

func protoPort(port int) (int32, error) {
	if err := validateLocalPort(port); err != nil {
		return 0, err
	}
	return int32(port), nil // #nosec G115 -- validated to 1..65535 before protobuf narrowing
}

func NewPanelAllocator(client tunnelpb.TunnelServiceClient, nodeID string) PanelAllocator {
	return &panelAllocator{client: client, nodeID: nodeID}
}

func (a *panelAllocator) Register(ctx context.Context, mapping PortMapping) (Assignment, error) {
	if mapping.ServerID == "" || validateLocalPort(mapping.LocalPort) != nil {
		return Assignment{}, fmt.Errorf("mapping tunnel tidak valid")
	}
	if mapping.Protocol != "tcp" && mapping.Protocol != "udp" {
		return Assignment{}, fmt.Errorf("protocol %q tidak didukung", mapping.Protocol)
	}
	localPort, err := protoPort(mapping.LocalPort)
	if err != nil {
		return Assignment{}, err
	}
	resp, err := a.client.RegisterTunnel(ctx, &tunnelpb.RegisterTunnelRequest{
		NodeId: a.nodeID, ServerId: mapping.ServerID, LocalPort: localPort, Protocol: mapping.Protocol,
	})
	if err != nil {
		return Assignment{}, fmt.Errorf("register tunnel ke panel: %w", err)
	}
	if resp.TunnelId == "" || resp.RemotePort < 1 || resp.RemotePort > 65535 {
		return Assignment{}, fmt.Errorf("assignment panel tidak valid")
	}
	status := TunnelStatus(resp.Status)
	if status == "" {
		status = TunnelPending
	}
	return Assignment{TunnelID: resp.TunnelId, RemotePort: int(resp.RemotePort), Status: status}, nil
}

func (a *panelAllocator) Release(ctx context.Context, mapping PortMapping) error {
	if err := validateLocalPort(mapping.LocalPort); err != nil {
		return err
	}
	localPort, err := protoPort(mapping.LocalPort)
	if err != nil {
		return err
	}
	_, err = a.client.ReleaseTunnel(ctx, &tunnelpb.ReleaseTunnelRequest{
		NodeId: a.nodeID, ServerId: mapping.ServerID, LocalPort: localPort, Protocol: mapping.Protocol,
	})
	if err != nil {
		return fmt.Errorf("release tunnel ke panel: %w", err)
	}
	return nil
}
