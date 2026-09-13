package store

import (
	"database/sql"
	"errors"
	"testing"
)

func TestNetworkingMigrationAndTunnelRoundtrip(t *testing.T) {
	db := openMigrated(t)
	if err := db.MigrateNetworking(); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateEgg(Egg{ID: "egg", Name: "egg", DockerfilePath: "d", MetadataPath: "m"}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateServer(Server{ID: "srv", Name: "srv", EggID: "egg", Status: "stopped", StartupCommand: "run", Environment: "{}"}); err != nil {
		t.Fatal(err)
	}
	want := Tunnel{ID: "tun", ServerID: "srv", LocalPort: 25565, RemotePort: 30001, Protocol: "tcp", Status: "connected"}
	if err := db.SaveTunnel(want); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetTunnelByMapping("srv", 25565, "tcp")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != want.ID || got.RemotePort != 30001 || got.Status != "connected" {
		t.Fatalf("tunnel=%+v", got)
	}
	if err := db.DeleteTunnelByMapping("srv", 25565, "tcp"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetTunnelByMapping("srv", 25565, "tcp"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("after delete err=%v", err)
	}
}
