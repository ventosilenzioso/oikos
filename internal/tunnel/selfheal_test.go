package tunnel

import (
	"context"
	"errors"
	"testing"
	"time"
)

type failingTunnelManager struct {
	reloads int
	fail    bool
}

func (m *failingTunnelManager) RegisterPort(context.Context, PortMapping) (Assignment, error) {
	return Assignment{}, nil
}
func (m *failingTunnelManager) ReleasePort(context.Context, string, int) error { return nil }
func (m *failingTunnelManager) ReleaseServer(context.Context, string) error    { return nil }
func (m *failingTunnelManager) Status(context.Context, string) ([]TunnelStatus, error) {
	return nil, nil
}
func (m *failingTunnelManager) Reload(context.Context) error {
	m.reloads++
	if m.fail {
		return errors.New("frpc mati")
	}
	return nil
}
func (m *failingTunnelManager) Watch(context.Context)       {}
func (m *failingTunnelManager) Close(context.Context) error { return nil }

func TestTunnelSelfHealerUsesBoundedPolicy(t *testing.T) {
	m := &failingTunnelManager{fail: true}
	h := NewTunnelSelfHealer(m, func(int) time.Duration { return 0 })
	if err := h.Start(context.Background()); err == nil {
		t.Fatal("recovery harus gagal setelah batas")
	}
	if m.reloads != 6 {
		t.Fatalf("reloads=%d", m.reloads)
	}
}
