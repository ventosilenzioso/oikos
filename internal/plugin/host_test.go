package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	pluginpb "github.com/oikos/oikos/gen/go/plugin"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestFakePluginProcess(t *testing.T) {
	if os.Getenv("OIKOS_FAKE_PLUGIN") != "1" {
		return
	}
	if os.Getenv("OIKOS_HOST_SECRET_MARKER") != "" {
		os.Exit(5)
	}
	fd, err := strconv.Atoi(os.Getenv("OIKOS_PLUGIN_SOCKET_FD"))
	if err != nil {
		os.Exit(2)
	}
	file := os.NewFile(uintptr(fd), "plugin-socket")
	listener, err := net.FileListener(file)
	if err != nil {
		os.Exit(2)
	}
	server := grpc.NewServer()
	pluginpb.RegisterPluginServiceServer(server, &fakePluginServer{
		crash: os.Getenv("OIKOS_FAKE_PLUGIN_CRASH") == "1",
		slow:  os.Getenv("OIKOS_FAKE_PLUGIN_SLOW") == "1",
	})
	if err := server.Serve(listener); err != nil {
		os.Exit(3)
	}
	os.Exit(0)
}

func TestHostStartsAndCleansUpFakePlugin(t *testing.T) {
	root := t.TempDir()
	eventFile := filepath.Join(root, "event")
	manifest := fakeManifest(t, root, eventFile, "")
	host, err := NewHost(manifest, HostOptions{SocketRoot: root, OikosVersion: "test", EventTimeout: time.Second, HealthTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := host.Start(); err != nil {
		t.Fatal(err)
	}
	defer host.Stop()

	if mode := socketMode(t, host.socketPath); mode != 0o600 {
		t.Fatalf("socket mode = %o, want 600", mode)
	}
	if err := host.HandleEvent(Event{Type: "server.crashed", ServerID: "srv-1"}); err != nil {
		t.Fatal(err)
	}
	if err := host.HealthCheck(); err != nil {
		t.Fatal(err)
	}
	if got := string(readFile(t, eventFile)); !strings.Contains(got, "server.crashed") {
		t.Fatalf("event file = %q", got)
	}
	if err := host.Stop(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(host.socketPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket still exists: %v", err)
	}
}

func TestHostRejectsUndeclaredCapabilities(t *testing.T) {
	root := t.TempDir()
	manifest := fakeManifest(t, root, "", "undeclared")
	host, err := NewHost(manifest, HostOptions{SocketRoot: root, OikosVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := host.Start(); err == nil || !strings.Contains(err.Error(), "capability") {
		t.Fatalf("Start error = %v", err)
	}
}

func TestHostCrashIsIsolatedAndMarkedUnhealthy(t *testing.T) {
	root := t.TempDir()
	manifest := fakeManifest(t, root, "", "crash")
	host, err := NewHost(manifest, HostOptions{SocketRoot: root, OikosVersion: "test", HealthTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := host.Start(); err != nil {
		t.Fatal(err)
	}
	defer host.Stop()
	time.Sleep(50 * time.Millisecond)
	if host.Status().Healthy {
		t.Fatal("crashed plugin reported healthy")
	}
	if err := host.HealthCheck(); err == nil {
		t.Fatal("HealthCheck succeeded after plugin crash")
	}
}

func TestHostAppliesEventAndHealthTimeouts(t *testing.T) {
	root := t.TempDir()
	manifest := fakeManifest(t, root, "", "slow")
	host, err := NewHost(manifest, HostOptions{SocketRoot: root, OikosVersion: "test", EventTimeout: 20 * time.Millisecond, HealthTimeout: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if err := host.Start(); err != nil {
		t.Fatal(err)
	}
	defer host.Stop()
	if err := host.HandleEvent(Event{Type: "server.crashed"}); err == nil {
		t.Fatal("slow event did not time out")
	}
	if err := host.HealthCheck(); err == nil {
		t.Fatal("slow health check did not time out")
	}
}

func TestHostCleanPluginExitIsUnhealthyAndHostRemainsUsable(t *testing.T) {
	root := t.TempDir()
	manifest := fakeManifest(t, root, "", "exit")
	host, err := NewHost(manifest, HostOptions{SocketRoot: root, OikosVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := host.Start(); err != nil {
		t.Fatal(err)
	}
	if err := host.HealthCheck(); err == nil {
		t.Fatal("health check succeeded while plugin exited")
	}
	if !waitUntil(time.Second, func() bool { return !host.Status().Running }) {
		t.Fatal("plugin did not exit")
	}
	if host.Status().Healthy {
		t.Fatal("clean plugin exit remained healthy")
	}
	if err := host.HealthCheck(); err == nil {
		t.Fatal("health check succeeded after clean exit")
	}
	if err := host.Stop(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(host.socketPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket remains after clean exit: %v", err)
	}
}

func TestHostAutomaticallyCleansUpAfterUnexpectedExitAndCanRestart(t *testing.T) {
	root := t.TempDir()
	manifest := fakeManifest(t, root, "", "exit")
	host, err := NewHost(manifest, HostOptions{SocketRoot: root, OikosVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := host.Start(); err != nil {
		t.Fatal(err)
	}
	if err := host.HealthCheck(); err == nil {
		t.Fatal("health check succeeded while plugin exited")
	}
	if !waitUntil(time.Second, func() bool { return !host.Status().Running }) {
		t.Fatal("plugin did not exit")
	}
	if _, err := os.Stat(host.socketPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket remains after unexpected exit: %v", err)
	}

	t.Setenv("OIKOS_FAKE_PLUGIN_MODE", "")
	if err := host.Start(); err != nil {
		t.Fatalf("restart after unexpected exit: %v", err)
	}
	if err := host.HealthCheck(); err != nil {
		t.Fatal(err)
	}
	if err := host.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestHostCanStartAndStopAfterFailedHandshake(t *testing.T) {
	root := t.TempDir()
	bad := fakeManifest(t, root, "", "undeclared")
	host, err := NewHost(bad, HostOptions{SocketRoot: root, OikosVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := host.Start(); err == nil {
		t.Fatal("undeclared capability handshake succeeded")
	}
	if err := host.Stop(); err != nil {
		t.Fatal(err)
	}
	good := fakeManifest(t, root, "", "")
	host.manifest = good
	if err := host.Start(); err != nil {
		t.Fatal(err)
	}
	if err := host.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := host.Start(); err != nil {
		t.Fatal(err)
	}
	if err := host.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestHostSocketIsSecureBeforeChildRuns(t *testing.T) {
	root := t.TempDir()
	manifest := fakeManifest(t, root, "", "")
	host, err := NewHost(manifest, HostOptions{SocketRoot: root, OikosVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := host.Start(); err != nil {
		t.Fatal(err)
	}
	defer host.Stop()
	if mode := socketMode(t, host.socketPath); mode != 0o600 {
		t.Fatalf("socket mode = %o, want 600", mode)
	}
}

type fakePluginServer struct {
	pluginpb.UnimplementedPluginServiceServer
	eventFile string
	crash     bool
	slow      bool
}

func (f *fakePluginServer) Init(_ context.Context, request *pluginpb.InitRequest) (*pluginpb.InitResponse, error) {
	f.eventFile = request.GetConfigPath()
	capability := &pluginpb.Capability{Events: []string{"server.crashed"}}
	if os.Getenv("OIKOS_FAKE_PLUGIN_MODE") == "undeclared" {
		capability.Events = []string{"server.started"}
	}
	if f.crash {
		go func() {
			time.Sleep(20 * time.Millisecond)
			os.Exit(4)
		}()
	}
	if os.Getenv("OIKOS_FAKE_PLUGIN_MODE") == "exit" {
		return &pluginpb.InitResponse{Capabilities: capability}, nil
	}
	return &pluginpb.InitResponse{Capabilities: capability}, nil
}

func (f *fakePluginServer) HandleEvent(ctx context.Context, event *pluginpb.Event) (*emptypb.Empty, error) {
	if f.slow {
		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if f.eventFile != "" {
		_ = os.WriteFile(f.eventFile, []byte(event.GetType()), 0o600)
	}
	return &emptypb.Empty{}, nil
}

func (f *fakePluginServer) HealthCheck(ctx context.Context, _ *emptypb.Empty) (*pluginpb.HealthCheckResponse, error) {
	if os.Getenv("OIKOS_FAKE_PLUGIN_MODE") == "exit" {
		os.Exit(0)
	}
	if f.slow {
		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return &pluginpb.HealthCheckResponse{Healthy: true}, nil
}

func fakeManifest(t *testing.T, root, eventFile, mode string) Manifest {
	t.Setenv("OIKOS_HOST_SECRET_MARKER", "must-not-reach-child")
	t.Setenv("OIKOS_FAKE_PLUGIN", "1")
	t.Setenv("OIKOS_FAKE_PLUGIN_MODE", mode)
	t.Setenv("OIKOS_FAKE_PLUGIN_CRASH", "")
	t.Setenv("OIKOS_FAKE_PLUGIN_SLOW", "")
	if mode == "crash" {
		t.Setenv("OIKOS_FAKE_PLUGIN_CRASH", "1")
	}
	if mode == "slow" {
		t.Setenv("OIKOS_FAKE_PLUGIN_SLOW", "1")
	}
	binary := os.Args[0]
	digest := sha256.Sum256(readFile(t, binary))
	return Manifest{ID: "fake", Name: "fake", Version: "1", Binary: binary, Enabled: true, AllowedEvents: []string{"server.crashed"}, SHA256: hex.EncodeToString(digest[:]), Config: eventFile}
}

func waitUntil(timeout time.Duration, condition func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return condition()
}

func socketMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
