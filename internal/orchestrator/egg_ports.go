package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/oikos/oikos/internal/egg"
	"github.com/oikos/oikos/internal/tunnel"
)

type EggPortProvider struct{ Root string }

func (p EggPortProvider) Ports(_ context.Context, eggID string) ([]tunnel.PortMapping, error) {
	e, err := egg.LoadDir(filepath.Join(p.Root, eggID))
	if err != nil {
		return nil, fmt.Errorf("load egg %s: %w", eggID, err)
	}
	if err := egg.Validate(e); err != nil {
		return nil, err
	}
	out := make([]tunnel.PortMapping, 0, len(e.Ports))
	for _, port := range e.Ports {
		if port.Protocol != "tcp" && port.Protocol != "udp" {
			return nil, fmt.Errorf("protocol egg %q tidak valid", port.Protocol)
		}
		out = append(out, tunnel.PortMapping{LocalPort: port.Default, Protocol: port.Protocol})
	}
	return out, nil
}
