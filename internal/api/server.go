// Package api menyediakan gRPC server lokal untuk debugging/CLI
// sebelum Panel terhubung (plaintext, bind localhost saja).
package api

import (
	"context"

	"google.golang.org/grpc"

	obspb "github.com/oikos/oikos/gen/go/observability"
	serverpb "github.com/oikos/oikos/gen/go/server"
	"github.com/oikos/oikos/internal/orchestrator"
	"github.com/oikos/oikos/internal/store"
)

type Server struct {
	serverpb.UnimplementedServerServiceServer
	lc *orchestrator.Lifecycle
}

// NewServer membangun gRPC server lokal di atas orchestrator.
func NewServer(lc *orchestrator.Lifecycle) *grpc.Server {
	srv := grpc.NewServer()
	serverpb.RegisterServerServiceServer(srv, &Server{lc: lc})
	return srv
}

func NewServerWithObservability(lc *orchestrator.Lifecycle, obs *ObservabilityService) *grpc.Server {
	srv := NewServer(lc)
	if obs != nil {
		obspb.RegisterObservabilityServiceServer(srv, obs)
	}
	return srv
}

func toInfo(s store.Server) *serverpb.ServerInfo {
	return &serverpb.ServerInfo{
		Id: s.ID, Name: s.Name, EggId: s.EggID, ContainerId: s.ContainerID,
		Status: s.Status, StartupCommand: s.StartupCommand,
	}
}

func (s *Server) Create(ctx context.Context, req *serverpb.CreateServerRequest) (*serverpb.CreateServerResponse, error) {
	id, err := s.lc.CreateServer(ctx, req.Name, req.EggId, req.StartupCommand, req.Environment)
	if err != nil {
		return nil, err
	}
	return &serverpb.CreateServerResponse{ServerId: id}, nil
}

func (s *Server) Start(ctx context.Context, req *serverpb.StartServerRequest) (*serverpb.ServerOpResponse, error) {
	if err := s.lc.StartServer(ctx, req.ServerId); err != nil {
		return nil, err
	}
	return &serverpb.ServerOpResponse{Ok: true}, nil
}

func (s *Server) Stop(ctx context.Context, req *serverpb.StopServerRequest) (*serverpb.ServerOpResponse, error) {
	// timeout_sec diabaikan di fondasi (lifecycle memakai 10 detik).
	if err := s.lc.StopServer(ctx, req.ServerId); err != nil {
		return nil, err
	}
	return &serverpb.ServerOpResponse{Ok: true}, nil
}

func (s *Server) Delete(ctx context.Context, req *serverpb.DeleteServerRequest) (*serverpb.ServerOpResponse, error) {
	if err := s.lc.DeleteServer(ctx, req.ServerId); err != nil {
		return nil, err
	}
	return &serverpb.ServerOpResponse{Ok: true}, nil
}

func (s *Server) Get(ctx context.Context, req *serverpb.GetServerRequest) (*serverpb.ServerInfo, error) {
	srv, err := s.lc.GetServer(req.ServerId)
	if err != nil {
		return nil, err
	}
	return toInfo(srv), nil
}

func (s *Server) List(ctx context.Context, _ *serverpb.ListServersRequest) (*serverpb.ListServersResponse, error) {
	servers, err := s.lc.ListServers()
	if err != nil {
		return nil, err
	}
	out := &serverpb.ListServersResponse{}
	for _, srv := range servers {
		out.Servers = append(out.Servers, toInfo(srv))
	}
	return out, nil
}
