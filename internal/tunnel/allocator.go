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

func NewPanelAllocator(client tunnelpb.TunnelServiceClient, nodeID string) PanelAllocator {
	return &panelAllocator{client: client, nodeID: nodeID}
}

func (a *panelAllocator) Register(ctx context.Context, mapping PortMapping) (Assignment, error) {
	if mapping.ServerID == "" || mapping.LocalPort < 1 || mapping.LocalPort > 65535 {
		return Assignment{}, fmt.Errorf("mapping tunnel tidak valid")
	}
	if mapping.Protocol != "tcp" && mapping.Protocol != "udp" {
		return Assignment{}, fmt.Errorf("protocol %q tidak didukung", mapping.Protocol)
	}
	resp, err := a.client.RegisterTunnel(ctx, &tunnelpb.RegisterTunnelRequest{
		NodeId: a.nodeID, ServerId: mapping.ServerID, LocalPort: int32(mapping.LocalPort), Protocol: mapping.Protocol,
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
	_, err := a.client.ReleaseTunnel(ctx, &tunnelpb.ReleaseTunnelRequest{
		NodeId: a.nodeID, ServerId: mapping.ServerID, LocalPort: int32(mapping.LocalPort), Protocol: mapping.Protocol,
	})
	if err != nil {
		return fmt.Errorf("release tunnel ke panel: %w", err)
	}
	return nil
}
