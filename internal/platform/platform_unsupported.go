//go:build !linux && !windows

package platform

import (
	"net"
	"os/exec"
)

// SocketPath is unavailable because Oikos currently requires Unix sockets.
func SocketPath(string) (net.Listener, error) { return nil, ErrNotSupported }

type unsupportedProcessGroup struct{}

func (unsupportedProcessGroup) Configure(*exec.Cmd) error { return ErrNotSupported }
func (unsupportedProcessGroup) Terminate(*exec.Cmd) error { return ErrNotSupported }

// NewProcessGroup returns the unsupported implementation on non-Linux Unix.
func NewProcessGroup() ProcessGroup { return unsupportedProcessGroup{} }
