// Command example-plugin adalah plugin contoh minimal untuk development.
package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	"net"
	"os"

	pluginpb "github.com/oikos/oikos/gen/go/plugin"
)

type plugin struct {
	pluginpb.UnimplementedPluginServiceServer
}

func (plugin) Init(context.Context, *pluginpb.InitRequest) (*pluginpb.InitResponse, error) {
	return &pluginpb.InitResponse{Capabilities: &pluginpb.Capability{Events: []string{"server.crashed"}}}, nil
}
func (plugin) HandleEvent(_ context.Context, e *pluginpb.Event) (*emptypb.Empty, error) {
	fmt.Fprintf(os.Stdout, "event type=%s server_id=%s\n", e.Type, e.ServerId)
	return &emptypb.Empty{}, nil
}
func (plugin) HealthCheck(context.Context, *emptypb.Empty) (*pluginpb.HealthCheckResponse, error) {
	return &pluginpb.HealthCheckResponse{Healthy: true}, nil
}
func (plugin) Close(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
func main() {
	socket := os.Getenv("OIKOS_PLUGIN_SOCKET")
	if socket == "" {
		fmt.Fprintln(os.Stderr, "OIKOS_PLUGIN_SOCKET wajib diisi")
		os.Exit(1)
	}
	lis, err := net.Listen("unix", socket)
	if err != nil {
		panic(err)
	}
	defer lis.Close()
	srv := grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
	pluginpb.RegisterPluginServiceServer(srv, plugin{})
	if err := srv.Serve(lis); err != nil {
		panic(err)
	}
}
