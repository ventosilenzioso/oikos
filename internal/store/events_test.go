package store

import (
	"context"
	"testing"
	"time"
)

func TestEventHistoryQueryAndPrune(t *testing.T) {
	db := openMigrated(t)
	if err := db.MigrateNetworking(); err != nil {
		t.Fatal(err)
	}
	if err := db.MigrateObservability(); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateEgg(Egg{ID: "egg-events", Name: "events", DockerfilePath: "d", MetadataPath: "m"}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateServer(Server{ID: "s1", Name: "server-1", EggID: "egg-events", Status: "stopped", StartupCommand: "sleep", Environment: "{}"}); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour).UTC().Truncate(time.Second)
	recent := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	for _, ev := range []Event{
		{ID: "old", Type: "server.crashed", ServerID: "s1", Severity: "error", Message: "old", Metadata: `{"x":1}`, CreatedAt: old},
		{ID: "new", Type: "server.started", ServerID: "s1", Severity: "info", Message: "new", Metadata: `{}`, CreatedAt: recent},
	} {
		if err := db.SaveEvent(ev); err != nil {
			t.Fatal(err)
		}
	}
	got, err := db.ListEvents(EventFilter{ServerID: "s1", Type: "server.started"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "new" {
		t.Fatalf("events=%+v", got)
	}
	if err := db.PruneEvents(context.Background(), time.Now().Add(-24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, err = db.ListEvents(EventFilter{ServerID: "s1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "new" {
		t.Fatalf("after prune=%+v", got)
	}
}
