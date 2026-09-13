package orchestrator

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/oikos/oikos/internal/runtime"
	"github.com/oikos/oikos/internal/store"
)

func setup(t *testing.T) *Lifecycle {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "o.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateEgg(store.Egg{ID: "egg-1", Name: "egg", DockerfilePath: "d", MetadataPath: "m"}); err != nil {
		t.Fatal(err)
	}
	return New(db, runtime.NewFake(), NewEventBus())
}

func TestCreateStartStopDelete(t *testing.T) {
	lc := setup(t)
	ctx := context.Background()
	id, err := lc.CreateServer(ctx, "srv", "egg-1", "run", map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if err := lc.StartServer(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := lc.StopServer(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := lc.DeleteServer(ctx, id); err != nil {
		t.Fatal(err)
	}
}
