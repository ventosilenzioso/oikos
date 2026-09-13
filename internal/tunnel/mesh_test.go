package tunnel

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/oikos/oikos/internal/store"
)

func TestRelayNetworkAssignmentRoundtrip(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "o.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := db.MigrateNetworking(); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveNode(store.Node{ID: "node-a", Name: "a", PanelURL: "p", CertPath: "c", KeyPath: "k", PairedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	m := NewRelayNetworkManager(nil, "node-a", db)
	assignment := NetworkGroupAssignment{GroupID: "g1", Name: "private", RelayAddr: "frps.test", RelayPort: 7000, PrivateIP: "10.88.0.2", Members: []NetworkGroupMember{{NodeID: "node-a", PrivateIP: "10.88.0.2"}}}
	if err := m.Assign(context.Background(), assignment); err != nil {
		t.Fatal(err)
	}
	members, err := m.Members(context.Background(), "g1")
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 || members[0].PrivateIP != "10.88.0.2" {
		t.Fatalf("members=%+v", members)
	}
}

func TestRelayNetworkRejectsInvalidIP(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "o.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := db.MigrateNetworking(); err != nil {
		t.Fatal(err)
	}
	m := NewRelayNetworkManager(nil, "node-a", db)
	err = m.Assign(context.Background(), NetworkGroupAssignment{GroupID: "g1", Name: "private", RelayAddr: "frps", RelayPort: 7000, PrivateIP: "not-an-ip"})
	if err == nil {
		t.Fatal("IP invalid harus ditolak")
	}
}
