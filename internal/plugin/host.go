package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	pluginpb "github.com/oikos/oikos/gen/go/plugin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type HostOptions struct {
	SocketRoot    string
	OikosVersion  string
	EventTimeout  time.Duration
	HealthTimeout time.Duration
}

type HostStatus struct {
	Healthy bool
	Running bool
	Error   error
}

type Host struct {
	manifest   Manifest
	options    HostOptions
	socketPath string
	cmd        *exec.Cmd
	conn       *grpc.ClientConn
	client     pluginpb.PluginServiceClient
	capability Capability
	status     HostStatus
	mu         sync.RWMutex
	stopOnce   sync.Once
	waitDone   chan struct{}
}

func NewHost(manifest Manifest, options HostOptions) (*Host, error) {
	if err := ValidateManifest(manifest); err != nil {
		return nil, err
	}
	if options.SocketRoot == "" {
		return nil, errors.New("plugin socket root must not be empty")
	}
	if options.EventTimeout <= 0 {
		options.EventTimeout = 5 * time.Second
	}
	if options.HealthTimeout <= 0 {
		options.HealthTimeout = 5 * time.Second
	}
	return &Host{manifest: manifest, options: options}, nil
}

func (h *Host) Start() error {
	h.mu.Lock()
	if h.status.Running {
		h.mu.Unlock()
		return errors.New("plugin already running")
	}
	if err := verifyBinary(h.manifest.Binary, h.manifest.SHA256); err != nil {
		h.mu.Unlock()
		return err
	}
	if err := os.MkdirAll(h.options.SocketRoot, 0o700); err != nil {
		h.mu.Unlock()
		return fmt.Errorf("create plugin socket root: %w", err)
	}
	h.socketPath = filepath.Join(h.options.SocketRoot, h.manifest.ID+".sock")
	_ = os.Remove(h.socketPath)
	cmd := exec.Command(h.manifest.Binary)
	cmd.Env = append(os.Environ(), "OIKOS_PLUGIN_SOCKET="+h.socketPath)
	h.cmd = cmd
	h.waitDone = make(chan struct{})
	if err := cmd.Start(); err != nil {
		h.mu.Unlock()
		return fmt.Errorf("start plugin: %w", err)
	}
	h.status = HostStatus{Running: true, Healthy: false}
	h.mu.Unlock()

	go h.waitProcess(cmd)
	if err := waitForSocket(h.socketPath, time.Second); err != nil {
		_ = h.Stop()
		return err
	}
	if err := os.Chmod(h.socketPath, 0o600); err != nil {
		_ = h.Stop()
		return fmt.Errorf("secure plugin socket: %w", err)
	}
	conn, err := grpc.Dial("unix://"+h.socketPath, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock(), grpc.WithConnectParams(grpc.ConnectParams{MinConnectTimeout: time.Second}))
	if err != nil {
		_ = h.Stop()
		return fmt.Errorf("connect plugin: %w", err)
	}
	client := pluginpb.NewPluginServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), h.options.HealthTimeout)
	response, err := client.Init(ctx, &pluginpb.InitRequest{OikosVersion: h.options.OikosVersion, ConfigPath: h.manifest.Config, PluginId: h.manifest.ID})
	cancel()
	if err == nil {
		capability := Capability{Events: response.GetCapabilities().GetEvents(), Routes: response.GetCapabilities().GetRoutes()}
		err = ValidateCapabilities(h.manifest, capability)
		if err == nil {
			h.mu.Lock()
			h.capability = capability
			h.mu.Unlock()
		}
	}
	if err != nil {
		_ = conn.Close()
		_ = h.Stop()
		return fmt.Errorf("plugin handshake: %w", err)
	}
	h.mu.Lock()
	h.conn, h.client, h.status.Healthy = conn, client, true
	h.mu.Unlock()
	return nil
}

func (h *Host) Stop() error {
	var result error
	h.stopOnce.Do(func() {
		h.mu.RLock()
		client, conn, cmd, socket := h.client, h.conn, h.cmd, h.socketPath
		h.mu.RUnlock()
		if client != nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			_, _ = client.Close(ctx, &emptypb.Empty{})
			cancel()
		}
		if conn != nil {
			_ = conn.Close()
		}
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			if h.waitDone != nil {
				select {
				case <-h.waitDone:
				case <-time.After(time.Second):
				}
			}
		}
		if socket != "" {
			result = os.Remove(socket)
			if errors.Is(result, os.ErrNotExist) {
				result = nil
			}
		}
		h.mu.Lock()
		h.status.Running, h.status.Healthy = false, false
		h.mu.Unlock()
	})
	return result
}

func (h *Host) HandleEvent(event Event) error {
	h.mu.RLock()
	client, healthy, capability := h.client, h.status.Healthy, h.capability
	h.mu.RUnlock()
	if client == nil || !healthy {
		return errors.New("plugin is unhealthy")
	}
	if !contains(capability.Events, event.Type) {
		return fmt.Errorf("event %q is not supported by plugin", event.Type)
	}
	ctx, cancel := context.WithTimeout(context.Background(), h.options.EventTimeout)
	defer cancel()
	_, err := client.HandleEvent(ctx, &pluginpb.Event{Type: event.Type, ServerId: event.ServerID, Metadata: event.Metadata})
	if err != nil {
		h.markUnhealthy(err)
	}
	return err
}

func (h *Host) HealthCheck() error {
	h.mu.RLock()
	client := h.client
	h.mu.RUnlock()
	if client == nil {
		return errors.New("plugin is not running")
	}
	ctx, cancel := context.WithTimeout(context.Background(), h.options.HealthTimeout)
	defer cancel()
	response, err := client.HealthCheck(ctx, &emptypb.Empty{})
	if err == nil && !response.GetHealthy() {
		err = errors.New(response.GetError())
	}
	if err != nil {
		h.markUnhealthy(err)
	}
	return err
}

func (h *Host) Routes() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]string(nil), h.capability.Routes...)
}

func (h *Host) Status() HostStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.status
}

func (h *Host) markUnhealthy(err error) {
	h.mu.Lock()
	h.status.Healthy, h.status.Error = false, err
	h.mu.Unlock()
}

func (h *Host) waitProcess(cmd *exec.Cmd) {
	if err := cmd.Wait(); err != nil {
		h.markUnhealthy(err)
	}
	close(h.waitDone)
	h.mu.Lock()
	h.status.Running = false
	h.mu.Unlock()
}

func verifyBinary(path, expected string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read plugin binary: %w", err)
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != expected {
		return errors.New("plugin binary checksum mismatch")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode()&0o111 == 0 {
		return errors.New("plugin binary is not executable")
	}
	return nil
}

func waitForSocket(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(path); err == nil && info.Mode()&os.ModeSocket != 0 {
			return nil
		}
		time.Sleep(5 * time.Millisecond)
	}
	return errors.New("plugin socket was not created")
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
