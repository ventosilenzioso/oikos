package agent

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/oikos/oikos/internal/orchestrator"
	"github.com/oikos/oikos/internal/runtime"
	"github.com/oikos/oikos/internal/store"

	nodepb "github.com/oikos/oikos/gen/go/node"
)

func TestStreamDispatch(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "o.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateEgg(store.Egg{ID: "egg-1", Name: "e", DockerfilePath: "d", MetadataPath: "m"}); err != nil {
		t.Fatal(err)
	}
	lc := orchestrator.New(db, runtime.NewFake(), orchestrator.NewEventBus())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srvID, err := lc.CreateServer(ctx, "srv", "egg-1", "run", map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	envJSON, _ := json.Marshal(map[string]string{"A": "1"})
	panel := newMockPanel(t)
	panel.queue(&nodepb.NodeCommand{CommandId: "c1", Action: "create", Args: map[string]string{
		"name": "srv2", "egg_id": "egg-1", "startup_command": "run", "environment": string(envJSON)}})
	panel.queue(&nodepb.NodeCommand{CommandId: "c2", Action: "start", ServerId: srvID})
	panel.queue(&nodepb.NodeCommand{CommandId: "c3", Action: "stop", ServerId: srvID})
	panel.queue(&nodepb.NodeCommand{CommandId: "c4", Action: "bogus", ServerId: srvID})
	panel.queue(&nodepb.NodeCommand{CommandId: "c5", Action: "delete", ServerId: srvID})
	done := make(chan error, 1)
	go func() { done <- RunCommandStream(ctx, panel.mtlsClient(t), lc) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		servers, err := db.ListServers()
		if err != nil {
			t.Fatal(err)
		}
		names := map[string]bool{}
		for _, s := range servers {
			names[s.Name] = true
		}
		if names["srv2"] && len(servers) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("stream belum selesai diproses: %+v", servers)
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done
}
