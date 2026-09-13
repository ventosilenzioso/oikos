package tunnel

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/oikos/oikos/internal/store"
)

type fakeTunnelStore struct{ tunnels map[string]store.Tunnel }

func newFakeTunnelStore() *fakeTunnelStore {
	return &fakeTunnelStore{tunnels: map[string]store.Tunnel{}}
}
func (s *fakeTunnelStore) SaveTunnel(t store.Tunnel) error { s.tunnels[t.ID] = t; return nil }
func (s *fakeTunnelStore) GetTunnelByMapping(server string, port int, protocol string) (store.Tunnel, error) {
	for _, t := range s.tunnels {
		if t.ServerID == server && t.LocalPort == port && t.Protocol == protocol {
			return t, nil
		}
	}
	return store.Tunnel{}, errTunnelMissing
}
func (s *fakeTunnelStore) ListTunnels(server string) ([]store.Tunnel, error) {
	var out []store.Tunnel
	for _, t := range s.tunnels {
		if t.ServerID == server {
			out = append(out, t)
		}
	}
	return out, nil
}
func (s *fakeTunnelStore) DeleteTunnelByMapping(server string, port int, protocol string) error {
	for id, t := range s.tunnels {
		if t.ServerID == server && t.LocalPort == port && t.Protocol == protocol {
			delete(s.tunnels, id)
		}
	}
	return nil
}
func (s *fakeTunnelStore) UpdateTunnelStatus(id, status string) error {
	if t, ok := s.tunnels[id]; ok {
		t.Status = status
		s.tunnels[id] = t
	}
	return nil
}

var errTunnelMissing = &missingTunnelError{}

type missingTunnelError struct{}

func (*missingTunnelError) Error() string { return "tunnel missing" }

type fakeAllocator struct {
	next  int
	calls int
}

func (a *fakeAllocator) Register(context.Context, PortMapping) (Assignment, error) {
	a.calls++
	return Assignment{TunnelID: "tun-1", RemotePort: a.next, Status: TunnelConnected}, nil
}
func (a *fakeAllocator) Release(context.Context, PortMapping) error { return nil }

func TestManagerRegisterWritesConfig(t *testing.T) {
	st := newFakeTunnelStore()
	alloc := &fakeAllocator{next: 30001}
	proc := NewFakeProcessController()
	path := filepath.Join(t.TempDir(), "frpc.toml")
	m := NewFrpManager(FRPConfig{ServerAddr: "frps.test", ServerPort: 7000, Token: "secret", ConfigPath: path}, alloc, st, proc, nil)
	a, err := m.RegisterPort(context.Background(), PortMapping{ServerID: "srv", LocalPort: 25565, Protocol: "tcp"})
	if err != nil {
		t.Fatal(err)
	}
	if a.RemotePort != 30001 {
		t.Fatalf("assignment=%+v", a)
	}
	if alloc.calls != 1 {
		t.Fatalf("allocator calls=%d", alloc.calls)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("30001")) {
		t.Fatalf("config=%s", data)
	}
}
