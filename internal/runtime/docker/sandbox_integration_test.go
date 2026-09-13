//go:build integration

package docker

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/oikos/oikos/internal/runtime"
	"github.com/oikos/oikos/internal/security"
)

func TestSeccompDefaultAllowsNormalWorkload(t *testing.T) {
	profilePath := filepath.Join("../../../deploy/docker/seccomp-default.json")
	if _, err := os.Stat(profilePath); err != nil {
		t.Skip("profile tidak tersedia: " + err.Error())
	}
	if out, err := exec.Command("docker", "info").CombinedOutput(); err != nil {
		t.Skip("Docker tidak tersedia: " + string(out))
	}
	profile, err := security.LoadDefaultSandboxOptions()
	if err != nil {
		t.Fatal(err)
	}
	rt, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	id, err := rt.Create(ctx, runtime.ContainerSpec{ServerID: "seccomp", Image: "busybox:stable", Command: []string{"sleep", "30"}, SeccompProfile: profile.SeccompProfile})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Delete(ctx, id, true)
	if err := rt.Start(ctx, id); err != nil {
		if strings.Contains(err.Error(), "operation not permitted") || strings.Contains(err.Error(), "Decoding seccomp profile") {
			t.Skip("host Docker runtime menolak seccomp profile: " + err.Error())
		}
		t.Fatal(err)
	}
	out, err := rt.Exec(ctx, id, []string{"echo", "sandbox-ok"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "sandbox-ok") {
		t.Fatalf("output=%q", out)
	}
}
