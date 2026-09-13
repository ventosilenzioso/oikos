//go:build integration

package orchestrator

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/oikos/oikos/internal/runtime/docker"
	"github.com/oikos/oikos/internal/store"
)

func TestSelfHealDockerCrash(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker CLI tidak tersedia")
	}
	if out, err := exec.Command("docker", "info").CombinedOutput(); err != nil {
		t.Skip("Docker daemon tidak tersedia: " + string(out))
	}
	if out, err := exec.Command("docker", "pull", "busybox:stable").CombinedOutput(); err != nil {
		t.Skip("busybox image tidak tersedia: " + string(out))
	}
	if out, err := exec.Command("docker", "tag", "busybox:stable", "oikos/observer-egg").CombinedOutput(); err != nil {
		t.Fatal("docker tag: ", string(out))
	}
	rt, err := docker.New("")
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(filepath.Join(t.TempDir(), "o.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateEgg(store.Egg{ID: "observer-egg", Name: "observer", DockerfilePath: "d", MetadataPath: "m"}); err != nil {
		t.Fatal(err)
	}
	lc := New(db, rt, NewEventBus())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	id, err := lc.CreateServer(ctx, "observer", "observer-egg", "sleep 60", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lc.DeleteServer(context.Background(), id)
	if err := lc.StartServer(ctx, id); err != nil {
		t.Fatal(err)
	}
	bus := NewEventBus()
	events := bus.Subscribe("server.crashed")
	observer := NewCrashObserver(lc, rt, bus, 100*time.Millisecond)
	observerCtx, stopObserver := context.WithCancel(ctx)
	defer stopObserver()
	go observer.Run(observerCtx)
	time.Sleep(300 * time.Millisecond)
	srv, err := db.GetServer(id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.Command("docker", "kill", srv.ContainerID).CombinedOutput(); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-events:
		if ev.ServerID != id {
			t.Fatalf("event=%+v", ev)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("server.crashed tidak diterima")
	}
}
