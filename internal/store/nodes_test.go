package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func openMigrated(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "oikos.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestNodeRoundtrip(t *testing.T) {
	db := openMigrated(t)
	if _, err := db.GetNode(); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("db kosong harus ErrNoRows, dapat %v", err)
	}
	n := Node{ID: "node-1", Name: "node-uji", PanelURL: "panel.test:9091",
		CertPath: "/c/node.crt", KeyPath: "/c/node.key", PairedAt: time.Now().UTC().Truncate(time.Second)}
	if err := db.SaveNode(n); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetNode()
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "node-1" || got.Name != "node-uji" || got.PanelURL != "panel.test:9091" {
		t.Fatalf("node salah: %+v", got)
	}
	if !got.PairedAt.Equal(n.PairedAt) {
		t.Fatalf("paired_at = %v, mau %v", got.PairedAt, n.PairedAt)
	}
}
