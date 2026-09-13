package orchestrator

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/oikos/oikos/internal/runtime"
	"github.com/oikos/oikos/internal/store"
	"github.com/oikos/oikos/internal/tunnel"
)

type lifecycleTunnelFake struct {
	registered []tunnel.PortMapping
	released   []string
	fail       bool
}

func (f *lifecycleTunnelFake) RegisterPort(_ context.Context, m tunnel.PortMapping) (tunnel.Assignment, error) {
	if f.fail {
		return tunnel.Assignment{}, errTestTunnel
	}
	f.registered = append(f.registered, m)
	return tunnel.Assignment{TunnelID: "t", RemotePort: 30001, Status: tunnel.TunnelConnected}, nil
}
func (f *lifecycleTunnelFake) ReleasePort(context.Context, string, int) error { return nil }
func (f *lifecycleTunnelFake) ReleaseServer(_ context.Context, id string) error {
	if f.fail {
		return errTestTunnel
	}
	f.released = append(f.released, id)
	return nil
}
func (f *lifecycleTunnelFake) Status(context.Context, string) ([]tunnel.TunnelStatus, error) {
	return nil, nil
}
func (f *lifecycleTunnelFake) Reload(context.Context) error { return nil }
func (f *lifecycleTunnelFake) Watch(context.Context)        {}
func (f *lifecycleTunnelFake) Close(context.Context) error  { return nil }

type testTunnelError struct{}

func (*testTunnelError) Error() string { return "test tunnel error" }

var errTestTunnel = &testTunnelError{}

func TestCreateDeleteRegistersAndReleasesEggPorts(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "o.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	fake := &lifecycleTunnelFake{}
	ports := PortProviderFunc(func(_ context.Context, eggID string) ([]tunnel.PortMapping, error) {
		return []tunnel.PortMapping{{ServerID: "", LocalPort: 25565, Protocol: "tcp"}}, nil
	})
	lc := NewWithTunnel(db, runtime.NewFake(), NewEventBus(), fake, ports)
	id, err := lc.CreateServer(context.Background(), "srv", "egg-with-port", "run", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.registered) != 1 || fake.registered[0].LocalPort != 25565 || fake.registered[0].ServerID != id {
		t.Fatalf("registered=%+v", fake.registered)
	}
	if err := lc.DeleteServer(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if len(fake.released) != 1 || fake.released[0] != id {
		t.Fatalf("released=%+v", fake.released)
	}
}

func TestCreateRollsBackWhenTunnelRegistrationFails(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "o.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	fake := &lifecycleTunnelFake{fail: true}
	ports := PortProviderFunc(func(context.Context, string) ([]tunnel.PortMapping, error) {
		return []tunnel.PortMapping{{LocalPort: 25565, Protocol: "tcp"}}, nil
	})
	lc := NewWithTunnel(db, runtime.NewFake(), NewEventBus(), fake, ports)
	if _, err := lc.CreateServer(context.Background(), "srv", "egg", "run", nil); err == nil {
		t.Fatal("register gagal harus dikembalikan")
	}
	servers, err := db.ListServers()
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 0 {
		t.Fatalf("rollback meninggalkan server: %+v", servers)
	}
}
