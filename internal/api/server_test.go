package api

import (
	"context"
	"net"
	"path/filepath"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	serverpb "github.com/oikos/oikos/gen/go/server"
	"github.com/oikos/oikos/internal/orchestrator"
	"github.com/oikos/oikos/internal/runtime"
	"github.com/oikos/oikos/internal/store"
)

func TestServerServiceCRUD(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "o.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	lc := orchestrator.New(db, runtime.NewFake(), orchestrator.NewEventBus())
	lis := bufconn.Listen(1 << 20)
	srv := NewServer(lc)
	go srv.Serve(lis)
	defer srv.Stop()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := serverpb.NewServerServiceClient(conn)
	ctx := context.Background()

	created, err := client.Create(ctx, &serverpb.CreateServerRequest{
		Name: "srv", EggId: "egg-1", StartupCommand: "run",
		Environment: map[string]string{"A": "1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ServerId == "" {
		t.Fatal("server id kosong")
	}
	got, err := client.Get(ctx, &serverpb.GetServerRequest{ServerId: created.ServerId})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "srv" || got.Status != "stopped" {
		t.Fatalf("info salah: %+v", got)
	}
	listed, err := client.List(ctx, &serverpb.ListServersRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Servers) != 1 {
		t.Fatalf("jumlah = %d", len(listed.Servers))
	}
	if _, err := client.Start(ctx, &serverpb.StartServerRequest{ServerId: created.ServerId}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Stop(ctx, &serverpb.StopServerRequest{ServerId: created.ServerId}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Delete(ctx, &serverpb.DeleteServerRequest{ServerId: created.ServerId}); err != nil {
		t.Fatal(err)
	}
	listed, err = client.List(ctx, &serverpb.ListServersRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Servers) != 0 {
		t.Fatalf("harus kosong, dapat %d", len(listed.Servers))
	}
}
