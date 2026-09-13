package store

import (
	"testing"
	"time"
)

func TestBackupMetadataRoundtrip(t *testing.T) {
	db := openMigrated(t)
	if err := db.CreateEgg(Egg{ID: "backup-egg", Name: "backup", DockerfilePath: "d", MetadataPath: "m"}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateServer(Server{ID: "backup-server", Name: "s", EggID: "backup-egg", Status: "stopped", StartupCommand: "run", Environment: "{}"}); err != nil {
		t.Fatal(err)
	}
	want := Backup{ID: "b1", ServerID: "backup-server", FilePath: "/tmp/b.tar.gz", SizeBytes: 12, ChecksumSHA: "abc", Status: "completed", CreatedAt: time.Now().UTC().Truncate(time.Second)}
	if err := db.SaveBackup(want); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetBackup("b1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != want.ID || got.SizeBytes != want.SizeBytes || got.Status != want.Status {
		t.Fatalf("backup=%+v", got)
	}
	if err := db.UpdateBackupStatus("b1", "failed", "def", 20); err != nil {
		t.Fatal(err)
	}
	got, _ = db.GetBackup("b1")
	if got.Status != "failed" || got.ChecksumSHA != "def" {
		t.Fatalf("updated=%+v", got)
	}
}
