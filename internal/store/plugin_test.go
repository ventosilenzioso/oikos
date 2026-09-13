package store

import (
	"testing"
	"time"
)

func TestPluginRegistryRoundtrip(t *testing.T) {
	db := openMigrated(t)
	if err := db.MigratePlugins(); err != nil {
		t.Fatal(err)
	}
	want := Plugin{
		ID:               "log-to-file",
		Name:             "Log",
		Version:          "1.0.0",
		BinaryPath:       "/plugins/log",
		Enabled:          true,
		SubscribedEvents: `["server.crashed"]`,
		InstalledAt:      time.Now().UTC().Truncate(time.Second),
	}
	if err := db.SavePlugin(want); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetPlugin(want.ID)
	if err != nil || got.Version != want.Version {
		t.Fatalf("plugin=%+v err=%v", got, err)
	}
	if err := db.SetPluginEnabled(want.ID, false); err != nil {
		t.Fatal(err)
	}
	got, err = db.GetPlugin(want.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Enabled {
		t.Fatal("must disable")
	}
}
