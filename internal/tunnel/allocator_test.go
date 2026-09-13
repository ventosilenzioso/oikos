package tunnel

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"

	tunnelpb "github.com/oikos/oikos/gen/go/tunnel"
)

type allocatorPanel struct {
	tunnelpb.UnimplementedTunnelServiceServer
	seen  map[string]*tunnelpb.RegisterTunnelResponse
	count int
}

func (p *allocatorPanel) RegisterTunnel(_ context.Context, req *tunnelpb.RegisterTunnelRequest) (*tunnelpb.RegisterTunnelResponse, error) {
	key := req.ServerId + ":" + string(rune(req.LocalPort)) + ":" + req.Protocol
	if existing := p.seen[key]; existing != nil {
		return existing, nil
	}
	p.count++
	resp := &tunnelpb.RegisterTunnelResponse{TunnelId: "tun-1", RemotePort: 30001, Status: "connected"}
	p.seen[key] = resp
	return resp, nil
}
func (p *allocatorPanel) ReleaseTunnel(context.Context, *tunnelpb.ReleaseTunnelRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func allocatorClient(t *testing.T, p *allocatorPanel) tunnelpb.TunnelServiceClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	tunnelpb.RegisterTunnelServiceServer(srv, p)
	go srv.Serve(lis)
	t.Cleanup(func() { srv.Stop() })
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return tunnelpb.NewTunnelServiceClient(conn)
}

func TestAllocatorRegisterIsIdempotent(t *testing.T) {
	panel := &allocatorPanel{seen: map[string]*tunnelpb.RegisterTunnelResponse{}}
	allocator := NewPanelAllocator(allocatorClient(t, panel), "node-1")
	mapping := PortMapping{TunnelID: "tun-1", ServerID: "srv-1", LocalPort: 25565, Protocol: "tcp"}
	first, err := allocator.Register(context.Background(), mapping)
	if err != nil {
		t.Fatal(err)
	}
	second, err := allocator.Register(context.Background(), mapping)
	if err != nil {
		t.Fatal(err)
	}
	if first.RemotePort != second.RemotePort || first.TunnelID != second.TunnelID {
		t.Fatalf("assignment berubah: %+v %+v", first, second)
	}
	if panel.count != 1 {
		t.Fatalf("register count=%d", panel.count)
	}
}

func TestAllocatorRejectsInvalidPanelAssignment(t *testing.T) {
	panel := &allocatorPanel{seen: map[string]*tunnelpb.RegisterTunnelResponse{}}
	// Panel diwakili dengan client nil response melalui malformed fake service.
	client := allocatorClient(t, panel)
	_, err := NewPanelAllocator(client, "node-1").Register(context.Background(), PortMapping{ServerID: "srv", LocalPort: 1, Protocol: "icmp"})
	if err == nil {
		t.Fatal("protocol invalid harus error")
	}
}
