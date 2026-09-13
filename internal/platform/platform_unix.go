//go:build linux

package platform

import (
	"net"
	"os/exec"
)

// SocketPath creates a Unix-domain listener at path.
func SocketPath(path string) (net.Listener, error) {
	return net.Listen("unix", path)
}

type unixProcessGroup struct{}

func (unixProcessGroup) Configure(cmd *exec.Cmd) error {
	return configureProcessGroup(cmd)
}

func (unixProcessGroup) Terminate(cmd *exec.Cmd) error {
	return terminateProcessGroup(cmd)
}

// NewProcessGroup returns the native process-group implementation.
func NewProcessGroup() ProcessGroup { return unixProcessGroup{} }
