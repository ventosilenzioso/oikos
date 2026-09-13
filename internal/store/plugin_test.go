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
		ConfigPath:       "/etc/oikos/log.yaml",
		SHA256:           "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Enabled:          true,
		SubscribedEvents: `["server.crashed"]`,
		AllowedRoutes:    `["/plugins/log-to-file/status"]`,
		InstalledAt:      time.Now().UTC().Truncate(time.Second),
	}
	if err := db.SavePlugin(want); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetPlugin(want.ID)
	if err != nil || got.Version != want.Version || got.ConfigPath != want.ConfigPath || got.SHA256 != want.SHA256 || got.AllowedRoutes != want.AllowedRoutes {
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

func TestMigratePluginsIsIdempotentAndUpgradesExistingRows(t *testing.T) {
	db := openMigrated(t)
	if _, err := db.sql.Exec(`DROP TABLE plugins`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.sql.Exec(`CREATE TABLE plugins (id TEXT PRIMARY KEY, name TEXT NOT NULL, version TEXT NOT NULL, binary_path TEXT NOT NULL, enabled INTEGER NOT NULL DEFAULT 0, subscribed_events TEXT NOT NULL DEFAULT '[]', installed_at DATETIME NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.sql.Exec(`INSERT INTO plugins(id,name,version,binary_path,enabled,subscribed_events,installed_at) VALUES('old','Old','1','/old',1,'[]','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := db.MigratePlugins(); err != nil {
		t.Fatal(err)
	}
	if err := db.MigratePlugins(); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetPlugin("old")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "old" || got.SHA256 != "" || got.AllowedRoutes != "[]" {
		t.Fatalf("upgraded plugin = %+v", got)
	}
}
