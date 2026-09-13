package store

import (
	"path/filepath"
	"testing"
)

func TestMigrateAndServerCRUD(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "oikos.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateEgg(Egg{ID: "minecraft-vanilla", Name: "Minecraft Vanilla", DockerfilePath: "./Dockerfile", MetadataPath: "./egg.yaml"}); err != nil {
		t.Fatal(err)
	}
	s := Server{ID: "srv-1", Name: "mc-1", EggID: "minecraft-vanilla", Status: "stopped", StartupCommand: "java -jar server.jar nogui", Environment: "{}"}
	if err := db.CreateServer(s); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetServer("srv-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "mc-1" || got.Status != "stopped" {
		t.Fatalf("server salah: %+v", got)
	}
	if err := db.UpdateServerStatus("srv-1", "running"); err != nil {
		t.Fatal(err)
	}
	got, _ = db.GetServer("srv-1")
	if got.Status != "running" {
		t.Fatalf("status = %q, mau running", got.Status)
	}
	if err := db.SetResourceLimits(ResourceLimits{ServerID: "srv-1", CPULimit: 2000, MemoryLimitMB: 2048, PIDLimit: 256}); err != nil {
		t.Fatal(err)
	}
	lim, err := db.GetResourceLimits("srv-1")
	if err != nil {
		t.Fatal(err)
	}
	if lim.CPULimit != 2000 || lim.MemoryLimitMB != 2048 {
		t.Fatalf("limits salah: %+v", lim)
	}
}
