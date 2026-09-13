//go:build linux

package platform

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestProcessGroupTerminatesConfiguredGroup(t *testing.T) {
	cmd := exec.Command("sh", "-c", "sleep 30 & wait")
	if err := NewProcessGroup().Configure(cmd); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	if pgid, err := syscall.Getpgid(cmd.Process.Pid); err != nil {
		t.Fatal(err)
	} else if pgid != cmd.Process.Pid {
		t.Fatalf("process group id = %d, want pid %d", pgid, cmd.Process.Pid)
	}
	if err := NewProcessGroup().Terminate(cmd); err != nil {
		t.Fatal(err)
	}
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	select {
	case <-wait:
	case <-time.After(time.Second):
		t.Fatal("process group did not terminate")
	}
}
