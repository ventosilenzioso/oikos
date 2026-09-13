package agent

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oikos/oikos/internal/config"
	"github.com/oikos/oikos/internal/store"
)

func testConfigDB(t *testing.T) (*config.Config, *store.DB, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Node.DataDir = filepath.Join(dir, "data")
	cfg.Node.CertPath = filepath.Join(dir, "certs", "node.crt")
	cfg.Node.KeyPath = filepath.Join(dir, "certs", "node.key")
	cfg.Panel.Address = "mock:0"
	db, err := store.Open(filepath.Join(dir, "oikos.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	return cfg, db, filepath.Join(dir, "config.yaml")
}

func TestPairWithPanelOK(t *testing.T) {
	tok := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	panel := newMockPanel(t, tok)
	cfg, db, configPath := testConfigDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	nodeID, err := PairWithPanel(ctx, "passthrough:///bufnet", tok, "node-uji", cfg, db, configPath, panel.nodeClient(t))
	if err != nil {
		t.Fatal(err)
	}
	if nodeID == "" {
		t.Fatal("node id kosong")
	}
	for _, p := range []string{cfg.Node.KeyPath, cfg.Node.CertPath} {
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0600 {
			t.Fatalf("%s mode = %o", p, fi.Mode().Perm())
		}
	}
	got, err := db.GetNode()
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != nodeID || got.Name != "node-uji" {
		t.Fatalf("node salah: %+v", got)
	}
	if panel.pairCount() != 1 {
		t.Fatalf("pairCount = %d", panel.pairCount())
	}
}

func TestPairRejectsBadToken(t *testing.T) {
	panel := newMockPanel(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	cfg, db, configPath := testConfigDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := PairWithPanel(ctx, "passthrough:///bufnet", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "n", cfg, db, configPath, panel.nodeClient(t))
	if err == nil {
		t.Fatal("harus error untuk token tak dikenal")
	}
	if _, err := db.GetNode(); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("db harus tetap kosong setelah gagal")
	}
}

func TestPairRefusesWhenAlreadyPaired(t *testing.T) {
	tok1 := "1111111111111111111111111111111111111111111111111111111111111111"
	tok2 := "2222222222222222222222222222222222222222222222222222222222222222"
	panel := newMockPanel(t, tok1, tok2)
	cfg, db, configPath := testConfigDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := PairWithPanel(ctx, "passthrough:///bufnet", tok1, "n", cfg, db, configPath, panel.nodeClient(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := PairWithPanel(ctx, "passthrough:///bufnet", tok2, "n", cfg, db, configPath, panel.nodeClient(t)); err == nil {
		t.Fatal("pairing kedua harus ditolak (sudah paired)")
	}
}
