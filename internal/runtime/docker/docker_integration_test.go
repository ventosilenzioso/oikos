//go:build integration

package docker

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/oikos/oikos/internal/runtime"
)

func ensureImage(t *testing.T, ref string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "pull", ref)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("docker pull %s: %v\n%s", ref, err, out)
	}
}

func TestDockerLifecycleIntegration(t *testing.T) {
	if os.Getenv("DOCKER_HOST") == "" {
		if _, err := os.Stat("/var/run/docker.sock"); err != nil {
			t.Skip("docker daemon tidak tersedia, skip test integrasi")
		}
	}
	rt, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	ensureImage(t, "busybox:stable")
	id, err := rt.Create(ctx, runtime.ContainerSpec{
		ServerID: "integ-1",
		Image:    "busybox:stable",
		Command:  []string{"sleep", "60"},
		Env:      map[string]string{"OIKOS_TEST": "1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Delete(ctx, id, true)
	if err := rt.Start(ctx, id); err != nil {
		t.Fatal(err)
	}
	st, err := rt.Status(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if st != runtime.StatusRunning {
		t.Fatalf("status = %q, mau running", st)
	}
	out, err := rt.Exec(ctx, id, []string{"echo", "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "hi" {
		t.Fatalf("exec output = %q, mau hi", out)
	}
	stats, err := rt.Stats(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if stats.MemoryLimitMB <= 0 {
		t.Fatalf("memory limit harus > 0, dapat %+v", stats)
	}
	if err := rt.Stop(ctx, id, 5); err != nil {
		t.Fatal(err)
	}
	st, err = rt.Status(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if st != runtime.StatusStopped {
		t.Fatalf("status = %q, mau stopped", st)
	}
}
