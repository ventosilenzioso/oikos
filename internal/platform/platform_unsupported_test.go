//go:build !linux

package platform

import (
	"errors"
	"os/exec"
	"testing"
)

func TestUnsupportedPlatformFeaturesReturnErrNotSupported(t *testing.T) {
	if _, err := SocketPath("unused"); !errors.Is(err, ErrNotSupported) {
		t.Fatalf("SocketPath error = %v, want ErrNotSupported", err)
	}
	group := NewProcessGroup()
	cmd := exec.Command("unused")
	if err := group.Configure(cmd); !errors.Is(err, ErrNotSupported) {
		t.Fatalf("Configure error = %v, want ErrNotSupported", err)
	}
	if err := group.Terminate(cmd); !errors.Is(err, ErrNotSupported) {
		t.Fatalf("Terminate error = %v, want ErrNotSupported", err)
	}
}
