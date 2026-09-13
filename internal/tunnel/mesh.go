package tunnel

import (
	"context"
	"fmt"
	"net/netip"

	tunnelpb "github.com/oikos/oikos/gen/go/tunnel"
	"github.com/oikos/oikos/internal/store"
)

type NetworkManager interface {
	Assign(context.Context, NetworkGroupAssignment) error
	Members(context.Context, string) ([]NetworkGroupMember, error)
	Release(context.Context, string, string) error
}

type NetworkGroupAssignment struct {
	GroupID   string
	Name      string
	RelayAddr string
	RelayPort int
	PrivateIP string
	Members   []NetworkGroupMember
}

type NetworkGroupMember struct {
	NodeID    string
	PrivateIP string
}

type networkStore interface {
	SaveNetworkGroup(store.NetworkGroup) error
	ReplaceNetworkGroupMembers(string, []store.NetworkGroupMember) error
	ListNetworkGroupMembers(string) ([]store.NetworkGroupMember, error)
	DeleteNetworkGroupMember(string, string) error
}

type relayNetworkManager struct {
	client tunnelpb.TunnelServiceClient
	nodeID string
	store  networkStore
}

func NewRelayNetworkManager(client tunnelpb.TunnelServiceClient, nodeID string, st networkStore) NetworkManager {
	return &relayNetworkManager{client: client, nodeID: nodeID, store: st}
}

func (m *relayNetworkManager) Assign(ctx context.Context, a NetworkGroupAssignment) error {
	if err := validateAssignment(a); err != nil {
		return err
	}
	if err := m.store.SaveNetworkGroup(store.NetworkGroup{ID: a.GroupID, Name: a.Name}); err != nil {
		return fmt.Errorf("simpan network group: %w", err)
	}
	members := make([]store.NetworkGroupMember, 0, len(a.Members))
	for _, member := range a.Members {
		if member.NodeID == "" {
			return fmt.Errorf("node member kosong")
		}
		if _, err := netip.ParseAddr(member.PrivateIP); err != nil {
			return fmt.Errorf("private ip %q tidak valid: %w", member.PrivateIP, err)
		}
		members = append(members, store.NetworkGroupMember{GroupID: a.GroupID, NodeID: member.NodeID, PrivateIP: member.PrivateIP})
	}
	if err := m.store.ReplaceNetworkGroupMembers(a.GroupID, members); err != nil {
		return fmt.Errorf("simpan member network group: %w", err)
	}
	return nil
}

func (m *relayNetworkManager) Members(ctx context.Context, groupID string) ([]NetworkGroupMember, error) {
	members, err := m.store.ListNetworkGroupMembers(groupID)
	if err != nil {
		return nil, err
	}
	out := make([]NetworkGroupMember, 0, len(members))
	for _, member := range members {
		out = append(out, NetworkGroupMember{NodeID: member.NodeID, PrivateIP: member.PrivateIP})
	}
	return out, nil
}

func (m *relayNetworkManager) Release(ctx context.Context, groupID, nodeID string) error {
	if groupID == "" || nodeID == "" {
		return fmt.Errorf("group_id dan node_id wajib diisi")
	}
	return m.store.DeleteNetworkGroupMember(groupID, nodeID)
}

func validateAssignment(a NetworkGroupAssignment) error {
	if a.GroupID == "" || a.Name == "" {
		return fmt.Errorf("group_id dan name wajib diisi")
	}
	if a.RelayAddr == "" || a.RelayPort < 1 || a.RelayPort > 65535 {
		return fmt.Errorf("relay endpoint tidak valid")
	}
	if _, err := netip.ParseAddr(a.PrivateIP); err != nil {
		return fmt.Errorf("private ip %q tidak valid: %w", a.PrivateIP, err)
	}
	return nil
}
