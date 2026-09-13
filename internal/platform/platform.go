// Package platform contains operating-system boundaries used by Oikos.
package platform

import (
	"errors"
	"os/exec"
)

// ErrNotSupported indicates that a platform-specific feature is unavailable.
var ErrNotSupported = errors.New("platform feature not supported")

// ProcessGroup isolates and controls a subprocess process group.
type ProcessGroup interface {
	Configure(*exec.Cmd) error
	Terminate(*exec.Cmd) error
}
