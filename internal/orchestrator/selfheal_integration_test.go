//go:build integration

package orchestrator

import (
	"os/exec"
	"testing"
)

func TestSelfHealDockerCrash(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker CLI tidak tersedia")
	}
	if out, err := exec.Command("docker", "info").CombinedOutput(); err != nil {
		t.Skip("Docker daemon tidak tersedia: " + string(out))
	}
	t.Skip("full crash observer harness requires a running daemon-owned lifecycle fixture")
}
