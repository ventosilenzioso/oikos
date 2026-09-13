//go:build integration

package orchestrator

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/oikos/oikos/internal/resource"
	"github.com/oikos/oikos/internal/runtime/docker"
	"github.com/oikos/oikos/internal/store"
)

func TestLimitsEnforcedIntegration(t *testing.T) {
	if resource.DetectCgroupVersion() != "v2" {
		t.Skip("butuh cgroup v2")
	}
	rt, err := docker.New("")
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("docker", "pull", "busybox:stable").CombinedOutput(); err != nil {
		t.Fatalf("docker pull: %v\n%s", err, out)
	}
	if out, err := exec.Command("docker", "tag", "busybox:stable", "oikos/egg-limit").CombinedOutput(); err != nil {
		t.Fatalf("docker tag: %v\n%s", err, out)
	}
	db, err := store.Open(filepath.Join(t.TempDir(), "o.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateEgg(store.Egg{ID: "egg-limit", Name: "limit", DockerfilePath: "d", MetadataPath: "m"}); err != nil {
		t.Fatal(err)
	}
	lc := New(db, rt, NewEventBus())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	id, err := lc.CreateServer(ctx, "limit-test", "egg-limit", "sleep 60", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lc.DeleteServer(context.Background(), id)
	if err := db.SetResourceLimits(store.ResourceLimits{ServerID: id, CPULimit: 500, MemoryLimitMB: 64, PIDLimit: 64}); err != nil {
		t.Fatal(err)
	}
	if err := lc.StartServer(ctx, id); err != nil {
		t.Fatal(err)
	}
	s, err := db.GetServer(id)
	if err != nil {
		t.Fatal(err)
	}
	cg, err := resource.FindContainerCgroup("/sys/fs/cgroup", s.ContainerID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := resource.ReadLimits(cg)
	if err != nil {
		t.Fatal(err)
	}
	want := resource.Limits{CPUMillicores: 500, MemoryBytes: 64 * 1024 * 1024, PIDMax: 64}
	if got != want {
		t.Fatalf("limit cgroup = %+v, mau %+v", got, want)
	}
}
