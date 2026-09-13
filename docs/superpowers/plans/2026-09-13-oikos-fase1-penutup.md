# Oikos Fase 1 Penutup (Resource, API, Daemon, Diagnose, Installer, Benchmark) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Menutup seluruh sisa Fase 1 (enforcement limit, API lokal, daemon, diagnose, installer, bukti DoD) agar siap lanjut Fase 2.

**Architecture:** Limit dibaca dari SQLite saat Start (Docker enforce saat create + enforcer tulis-ulang cgroup sebagai verifikasi). Daemon loop (dial→heartbeat+stream→reconnect) diimplementasi sebagai `agent.Run` yang testable, CLI hanya wrapper tipis. Mock Panel TCP dipakai untuk e2e daemon dan benchmark idle.

**Tech Stack:** Go 1.26, gRPC, cgroup v2 sysfs, Docker SDK v28, `cmd/mock-panel` (dev-only).

**Spec:** `docs/superpowers/specs/2026-09-13-oikos-fase1-fondasi-design.md` + keputusan ronde penutup (disetujui user).

## Global Constraints

- Module Go: `github.com/oikos/oikos`.
- Go minimum: 1.23.
- SQLite driver: `modernc.org/sqlite` (tanpa cgo).
- File key/cert ditulis dengan permission 0600.
- Semua test jaringan memakai bufconn, kecuali e2e daemon dan stress test yang memakai TCP/Docker nyata di localhost.
- `cmd/mock-panel` adalah dev tool, tidak untuk produksi.

---

### Task 1: Proto ServerService + regen

**Files:**
- Modify: `proto/server/server.proto`
- Create: `gen/go/server/server_grpc.pb.go` (via make proto; `server.pb.go` ter-regen)

**Interfaces:**
- Consumes: tidak ada.
- Produces: package `serverpb "github.com/oikos/oikos/gen/go/server"` dengan `ServerServiceClient/Server` dan pesan di bawah.

- [ ] **Step 1: Tulis service dan pesan baru**

`proto/server/server.proto` menjadi:

```proto
syntax = "proto3";

package server;

option go_package = "github.com/oikos/oikos/gen/go/server";

message CreateServerRequest {
  string name = 1;
  string egg_id = 2;
  string startup_command = 3;
  map<string, string> environment = 4;
}

message StartServerRequest {
  string server_id = 1;
}

message StopServerRequest {
  string server_id = 1;
  int32 timeout_sec = 2;
}

message DeleteServerRequest {
  string server_id = 1;
  bool force = 2;
}

message ServerOpResponse {
  bool ok = 1;
  string message = 2;
}

message CreateServerResponse {
  string server_id = 1;
}

message ServerInfo {
  string id = 1;
  string name = 2;
  string egg_id = 3;
  string container_id = 4;
  string status = 5;
  string startup_command = 6;
}

message GetServerRequest {
  string server_id = 1;
}

message ListServersRequest {
}

message ListServersResponse {
  repeated ServerInfo servers = 1;
}

service ServerService {
  rpc Create(CreateServerRequest) returns (CreateServerResponse);
  rpc Start(StartServerRequest) returns (ServerOpResponse);
  rpc Stop(StopServerRequest) returns (ServerOpResponse);
  rpc Delete(DeleteServerRequest) returns (ServerOpResponse);
  rpc Get(GetServerRequest) returns (ServerInfo);
  rpc List(ListServersRequest) returns (ListServersResponse);
}
```

- [ ] **Step 2: Regen dan verifikasi**

Run:

```bash
make proto && go build ./...
```

Expected: `gen/go/server/server_grpc.pb.go` ada; build sukses.

- [ ] **Step 3: Commit**

```bash
git add proto/server/server.proto gen/go/server/
git commit -m "feat: proto ServerService lokal"
```

---

### Task 2: Resource enforcer + wiring limit ke StartServer

**Files:**
- Create: `internal/resource/enforcer.go`, `internal/resource/enforcer_test.go`
- Modify: `internal/orchestrator/lifecycle.go`
- Test: `internal/resource/enforcer_test.go`

**Interfaces:**
- Consumes: `store.GetResourceLimits` (ada), `resource.DetectCgroupVersion` (ada).
- Produces: `resource.Limits{CPUMillicores, MemoryBytes, PIDMax int64}`; `resource.ApplyLimits(dir string, lim Limits) error`; `resource.FindContainerCgroup(root, containerID string) (string, error)`; `resource.ReadLimits(dir string) (Limits, error)`.

- [ ] **Step 1: Write the failing test**

`internal/resource/enforcer_test.go`:

```go
package resource

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyAndReadLimits(t *testing.T) {
	dir := t.TempDir()
	lim := Limits{CPUMillicores: 500, MemoryBytes: 32 * 1024 * 1024, PIDMax: 64}
	if err := ApplyLimits(dir, lim); err != nil {
		t.Fatal(err)
	}
	got, err := ReadLimits(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != lim {
		t.Fatalf("got %+v, mau %+v", got, lim)
	}
}

func TestApplySkipsZero(t *testing.T) {
	dir := t.TempDir()
	if err := ApplyLimits(dir, Limits{}); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"cpu.max", "memory.max", "pids.max"} {
		if _, err := os.Stat(filepath.Join(dir, f)); !os.IsNotExist(err) {
			t.Fatalf("%s harus tidak ditulis", f)
		}
	}
}

func TestFindContainerCgroup(t *testing.T) {
	root := t.TempDir()
	id := "abc123def456789"
	scope := filepath.Join(root, "system.slice", "docker-"+id+".scope")
	if err := os.MkdirAll(scope, 0755); err != nil {
		t.Fatal(err)
	}
	got, err := FindContainerCgroup(root, id)
	if err != nil {
		t.Fatal(err)
	}
	if got != scope {
		t.Fatalf("got %q, mau %q", got, scope)
	}
	if _, err := FindContainerCgroup(root, "tidakada000000"); err == nil {
		t.Fatal("harus error untuk id tak dikenal")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/resource/ -v`
Expected: FAIL `undefined`.

- [ ] **Step 3: Write minimal implementation**

`internal/resource/enforcer.go`:

```go
package resource

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Limits adalah batas resource; 0 berarti unlimited (tidak ditulis).
type Limits struct {
	CPUMillicores int64
	MemoryBytes   int64
	PIDMax        int64
}

// ApplyLimits menulis file limit cgroup v2 ke direktori cgroup yang sudah ada.
func ApplyLimits(dir string, lim Limits) error {
	if lim.CPUMillicores > 0 {
		if err := writeCgroupFile(dir, "cpu.max", strconv.FormatInt(lim.CPUMillicores*100, 10)+" 100000"); err != nil {
			return err
		}
	}
	if lim.MemoryBytes > 0 {
		if err := writeCgroupFile(dir, "memory.max", strconv.FormatInt(lim.MemoryBytes, 10)); err != nil {
			return err
		}
	}
	if lim.PIDMax > 0 {
		if err := writeCgroupFile(dir, "pids.max", strconv.FormatInt(lim.PIDMax, 10)); err != nil {
			return err
		}
	}
	return nil
}

// ReadLimits membaca kembali limit dari direktori cgroup ("max" dibaca sebagai 0).
func ReadLimits(dir string) (Limits, error) {
	var lim Limits
	cpu, err := readCgroupFile(dir, "cpu.max")
	if err != nil {
		return Limits{}, err
	}
	if f := strings.Fields(cpu); len(f) > 0 && f[0] != "max" {
		quota, err := strconv.ParseInt(f[0], 10, 64)
		if err != nil {
			return Limits{}, fmt.Errorf("parse cpu.max: %w", err)
		}
		lim.CPUMillicores = quota / 100
	}
	mem, err := readCgroupFile(dir, "memory.max")
	if err != nil {
		return Limits{}, err
	}
	if strings.TrimSpace(mem) != "max" {
		lim.MemoryBytes, err = strconv.ParseInt(strings.TrimSpace(mem), 10, 64)
		if err != nil {
			return Limits{}, fmt.Errorf("parse memory.max: %w", err)
		}
	}
	pids, err := readCgroupFile(dir, "pids.max")
	if err != nil {
		return Limits{}, err
	}
	if strings.TrimSpace(pids) != "max" {
		lim.PIDMax, err = strconv.ParseInt(strings.TrimSpace(pids), 10, 64)
		if err != nil {
			return Limits{}, fmt.Errorf("parse pids.max: %w", err)
		}
	}
	return lim, nil
}

// FindContainerCgroup mencari direktori cgroup milik container (driver systemd
// "docker-<id>.scope" dulu, lalu fallback nama id penuh/pendek).
func FindContainerCgroup(root, containerID string) (string, error) {
	short := containerID
	if len(short) > 12 {
		short = short[:12]
	}
	var exact, partial string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() || (exact != "" && partial != "") {
			return nil
		}
		base := filepath.Base(p)
		switch {
		case base == "docker-"+containerID+".scope" || base == containerID:
			exact = p
		case partial == "" && base == short:
			partial = p
		}
		return nil
	})
	if exact != "" {
		return exact, nil
	}
	if partial != "" {
		return partial, nil
	}
	return "", fmt.Errorf("cgroup untuk container %s tidak ditemukan", short)
}

func writeCgroupFile(dir, name, content string) error {
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		return fmt.Errorf("tulis %s: %w", name, err)
	}
	return nil
}

func readCgroupFile(dir, name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return "", fmt.Errorf("baca %s: %w", name, err)
	}
	return string(data), nil
}
```

Wiring di `internal/orchestrator/lifecycle.go`, fungsi StartServer: setelah `getServer`, tambah baca limit dan isi spec; setelah status running, terapkan enforcer. Blok yang diubah:

```go
	containerID := s.ContainerID
	lim, limErr := l.db.GetResourceLimits(id)
	hasLimits := limErr == nil && (lim.CPULimit > 0 || lim.MemoryLimitMB > 0 || lim.PIDLimit > 0)
	if containerID == "" {
		...
		containerID, err = l.rt.Create(ctx, runtime.ContainerSpec{
			ServerID:  s.ID,
			Image:     "oikos/" + s.EggID,
			Command:   strings.Fields(s.StartupCommand),
			Env:       env,
			MountSource: "",
		})
```

menjadi spec dengan limit:

```go
		spec := runtime.ContainerSpec{
			ServerID:    s.ID,
			Image:       "oikos/" + s.EggID,
			Command:     strings.Fields(s.StartupCommand),
			Env:         env,
			MountSource: "",
		}
		if hasLimits {
			spec.CPULimit = lim.CPULimit
			spec.MemoryLimit = lim.MemoryLimitMB * 1024 * 1024
			spec.PIDLimit = lim.PIDLimit
		}
		containerID, err = l.rt.Create(ctx, spec)
```

dan setelah `setStatus(l.db, id, "running")` di StartServer, tambah:

```go
	if hasLimits && resource.DetectCgroupVersion() == "v2" {
		if cg, err := resource.FindContainerCgroup("/sys/fs/cgroup", containerID); err == nil {
			rlim := resource.Limits{CPUMillicores: lim.CPULimit, MemoryBytes: lim.MemoryLimitMB * 1024 * 1024, PIDLimit: lim.PIDLimit}
			if err := resource.ApplyLimits(cg, rlim); err != nil {
				return fmt.Errorf("terapkan limit: %w", err)
			}
		}
	}
```

Tambah import `database/sql`? Tidak perlu (abaikan ErrNoRows: hasLimits false bila err). Import baru: `github.com/oikos/oikos/internal/resource`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/resource/ ./internal/orchestrator/ -v`
Expected: PASS (test orchestrator lama tetap hijau karena tanpa baris limits).

- [ ] **Step 5: Commit**

```bash
git add internal/resource/ internal/orchestrator/
git commit -m "feat: enforcement limit cgroup v2"
```

---

### Task 3: Lifecycle Get/List + API gRPC lokal

**Files:**
- Modify: `internal/orchestrator/lifecycle.go` (tambah GetServer, ListServers)
- Create: `internal/api/server.go`, `internal/api/server_test.go`
- Test: `internal/api/server_test.go`

**Interfaces:**
- Consumes: `orchestrator.Lifecycle`, `serverpb`.
- Produces: `api.NewServer(lc *orchestrator.Lifecycle) *grpc.Server`; `(*orchestrator.Lifecycle).GetServer(id) (store.Server, error)`; `(*orchestrator.Lifecycle).ListServers() ([]store.Server, error)`.

- [ ] **Step 1: Write the failing test**

`internal/api/server_test.go`:

```go
package api

import (
	"context"
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
		grpc.WithContextDialer(func(ctx context.Context, _ string) (interface{ Read([]byte) (int, error) }, error) {
			return nil, nil
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	_ = conn
	_ = err
}
```

STOP — dialer di atas salah ketik dan tidak berguna. Tulis yang benar sejak awal:

```go
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
```

Import: context, net, path/filepath, testing, grpc, insecure, bufconn, serverpb, orchestrator, runtime, store.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/ -v`
Expected: FAIL `undefined: NewServer` (dan `undefined: GetServer`/`ListServers` pada Lifecycle saat implementasi setengah jalan — itu wajar, lanjutkan Step 3 sampai hijau).

- [ ] **Step 3: Write minimal implementation**

`internal/orchestrator/lifecycle.go`, tambah di akhir:

```go
// GetServer membaca satu server dari store.
func (l *Lifecycle) GetServer(id string) (store.Server, error) {
	return getServer(l.db, id)
}

// ListServers membaca semua server dari store.
func (l *Lifecycle) ListServers() ([]store.Server, error) {
	return l.db.ListServers()
}
```

`internal/api/server.go`:

```go
// Package api menyediakan gRPC server lokal untuk debugging/CLI
// sebelum Panel terhubung (plaintext, bind localhost saja).
package api

import (
	"context"

	"google.golang.org/grpc"

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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/api/ ./internal/orchestrator/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/ internal/orchestrator/ proto/server/server.proto gen/go/server/
git commit -m "feat: api grpc lokal server service"
```

---

### Task 4: agent.Run daemon loop + wiring CLI

**Files:**
- Create: `internal/agent/daemon.go`
- Create: `cmd/oikos/daemon.go`
- Modify: `cmd/oikos/main.go` (case daemon/diagnose, usage string, refactor openLifecycle)

**Interfaces:**
- Consumes: `agent.DialPanel/RunHeartbeat/RunCommandStream/ComputeBackoff`, `api.NewServer`, `store.Node`.
- Produces: `agent.DaemonArgs{PanelAddr, CertPath, KeyPath, CAPath, NodeID string; LocalPort int}`; `agent.Run(ctx, args DaemonArgs, lc *Lifecycle) error`; CLI `oikos daemon --config`.

- [ ] **Step 1: Refactor kecil openLifecycle (tanpa test baru, dijaga test lama)**

`cmd/oikos/main.go`: ubah `openLifecycle()` menjadi wrapper:

```go
func openLifecycle() (*orchestrator.Lifecycle, error) {
	return openLifecycleWith(flagRuntime)
}

func openLifecycleWith(engine string) (*orchestrator.Lifecycle, error) {
```

dan di badan ganti `if flagRuntime == "fake"` menjadi `if engine == "fake"`. Update usage string menjadi `"no command (gunakan: server, install, daemon, diagnose)"` dan tambah case:

```go
	case "daemon":
		return runDaemon(args[1:])
	case "diagnose":
		return runDiagnose(args[1:])
```

Run: `go build ./... && go test ./...` — Expected: hijau (tidak ada perilaku berubah).

- [ ] **Step 2: Tulis agent.Run**

`internal/agent/daemon.go`:

```go
package agent

import (
	"context"
	"fmt"
	"net"
	"time"

	nodepb "github.com/oikos/oikos/gen/go/node"
	"github.com/oikos/oikos/internal/api"
	"github.com/oikos/oikos/internal/orchestrator"
)

// DaemonArgs adalah parameter loop daemon.
type DaemonArgs struct {
	PanelAddr string
	CertPath  string
	KeyPath   string
	CAPath    string
	NodeID    string
	LocalPort int
}

// Run menjalankan loop daemon: serve API lokal, dial Panel, heartbeat +
// command stream, reconnect dengan backoff. Kembali hanya saat ctx selesai.
func Run(ctx context.Context, args DaemonArgs, lc *orchestrator.Lifecycle) error {
	lis, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", args.LocalPort))
	if err != nil {
		return fmt.Errorf("listen api lokal: %w", err)
	}
	apiSrv := api.NewServer(lc)
	go func() { _ = apiSrv.Serve(lis) }()
	defer apiSrv.GracefulStop()

	since := time.Now()
	attempt := 0
	for {
		if err := runOnce(ctx, args, lc, since); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
		}
		attempt++
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(ComputeBackoff(attempt)):
		}
	}
}

func runOnce(ctx context.Context, args DaemonArgs, lc *orchestrator.Lifecycle, since time.Time) error {
	conn, err := DialPanel(ctx, args.PanelAddr, args.CertPath, args.KeyPath, args.CAPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	client := nodepb.NewNodeServiceClient(conn)
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errCh := make(chan error, 2)
	go func() { errCh <- RunHeartbeat(cctx, client, args.NodeID, DefaultHeartbeatInterval, since) }()
	go func() { errCh <- RunCommandStream(cctx, client, lc) }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}
```

- [ ] **Step 3: Tulis cmd/oikos/daemon.go**

```go
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/oikos/oikos/internal/agent"
	"github.com/oikos/oikos/internal/config"
	"github.com/oikos/oikos/internal/store"
)

func runDaemon(args []string) error {
	fs := flag.NewFlagSet("daemon", flag.ContinueOnError)
	configPath := fs.String("config", "/etc/oikos/config.yaml", "path config.yaml")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	flagConfig = *configPath
	flagRuntime = cfg.Runtime.Engine
	lc, err := openLifecycle()
	if err != nil {
		return err
	}
	db, err := store.Open(filepath.Join(cfg.Node.DataDir, "oikos.db"))
	if err != nil {
		return err
	}
	defer db.Close()
	node, err := db.GetNode()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("belum pairing, jalankan oikos install dulu")
		}
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return agent.Run(ctx, agent.DaemonArgs{
		PanelAddr: cfg.Panel.Address,
		CertPath:  node.CertPath,
		KeyPath:   node.KeyPath,
		CAPath:    cfg.Panel.CAPath,
		NodeID:    node.ID,
		LocalPort: cfg.API.LocalGRPCPort,
	}, lc)
}
```

- [ ] **Step 4: Verifikasi**

Run: `go build ./... && go test ./...`
Expected: hijau. CLI manual: `./bin/oikos daemon --config /tmp/oikos-e2e2/config.yaml` harus gagal cepat dengan "belum pairing" (db e2e2 tanpa node). Lalu `./bin/oikos no-such-command` → unknown command.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/daemon.go cmd/oikos/
git commit -m "feat: daemon loop dengan reconnect"
```

---

### Task 5: Daemon e2e test (mock Panel TCP nyata)

**Files:**
- Create: `internal/agent/daemon_test.go`
- Test: `internal/agent/daemon_test.go`

**Interfaces:**
- Consumes: mock panel + `mustServerTLS` (ada di panel_mock_test.go), `agent.Run`, `PairWithPanel` (client nil → dial TCP nyata).

- [ ] **Step 1: Write the test (test ini gagal sebelum Task 4? Tidak — Task 4 sudah commit. Test ini gagal karena Run belum dipakai e2e; verifikasi ia PASS sebagai bukti integrasi. Untuk disiplin TDD, jalankan dulu dan harapkan FAIL `undefined: Run` bila daemon.go belum ada — karena Task 4 sudah selesai, test langsung hijau dan berfungsi sebagai regression/integration proof. Tetap jalankan untuk verifikasi.)**

`internal/agent/daemon_test.go`:

```go
package agent

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"

	nodepb "github.com/oikos/oikos/gen/go/node"
	serverpb "github.com/oikos/oikos/gen/go/server"
	"github.com/oikos/oikos/internal/orchestrator"
	"github.com/oikos/oikos/internal/runtime"
	"github.com/oikos/oikos/internal/store"
	"google.golang.org/grpc/credentials/insecure"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestDaemonEndToEnd(t *testing.T) {
	tok := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	panel := newMockPanel(t, tok)

	plainLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	plainSrv := grpc.NewServer()
	nodepb.RegisterNodeServiceServer(plainSrv, panel)
	go plainSrv.Serve(plainLis)
	defer plainSrv.Stop()

	srvCertPEM, srvKey := panel.issue("panel.test")
	tlsLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	tlsSrv := grpc.NewServer(grpc.Creds(mustServerTLS(t, srvCertPEM, srvKey, panel.caPEM)))
	nodepb.RegisterNodeServiceServer(tlsSrv, panel)
	go tlsSrv.Serve(tlsLis)
	defer tlsSrv.Stop()

	dir := t.TempDir()
	cfg, db, configPath := testConfigDB(t)
	_ = dir
	pairCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	nodeID, err := PairWithPanel(pairCtx, plainLis.Addr().String(), tok, "node-e2e", cfg, db, configPath, nil)
	if err != nil {
		t.Fatal(err)
	}

	lc := orchestrator.New(db, runtime.NewFake(), orchestrator.NewEventBus())
	runCtx, stop := context.WithCancel(context.Background())
	defer stop()
	envJSON, _ := json.Marshal(map[string]string{})
	panel.queue(&nodepb.NodeCommand{CommandId: "d1", Action: "create", Args: map[string]string{
		"name": "daemon-srv", "egg_id": "egg-1", "startup_command": "run", "environment": string(envJSON)}})
	localPort := freePort(t)
	done := make(chan error, 1)
	go func() {
		done <- Run(runCtx, DaemonArgs{
			PanelAddr: tlsLis.Addr().String(),
			CertPath:  cfg.Node.CertPath,
			KeyPath:   cfg.Node.KeyPath,
			CAPath:    panel.caPath(t),
			NodeID:    nodeID,
			LocalPort: localPort,
		}, lc)
	}()
	deadline := time.Now().Add(25 * time.Second)
	for {
		hb := panel.heartbeats()
		servers, err := db.ListServers()
		if err != nil {
			t.Fatal(err)
		}
		if len(hb) > 0 && len(servers) == 1 && servers[0].Name == "daemon-srv" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("daemon e2e macet: hb=%d servers=%+v", len(hb), servers)
		}
		time.Sleep(20 * time.Millisecond)
	}
	// Buktikan API lokal hidup: List via TCP.
	conn, err := grpc.NewClient(
		"127.0.0.1:"+itoa(localPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	sclient := serverpb.NewServerServiceClient(conn)
	lctx, lcancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer lcancel()
	listed, err := sclient.List(lctx, &serverpb.ListServersRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Servers) != 1 || listed.Servers[0].Name != "daemon-srv" {
		t.Fatalf("list api lokal salah: %+v", listed.Servers)
	}
	stop()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("Run = %v, mau context.Canceled", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run tidak berhenti setelah cancel")
	}
	_ = filepath.Join
}
```

Catatan: `testConfigDB` tidak membuat egg `egg-1` — command create butuh egg terdaftar? `CreateServer` memanggil `EnsureEgg` (insert or ignore) sehingga egg-1 otomatis terdaftar. `filepath` dan `dir` tidak dibutuhkan — hapus `_ = dir` dan import filepath bila tak terpakai (gofmt/vet akan protes unused import; tulis final tanpa keduanya). `itoa`: gunakan `strconv.Itoa` (import strconv).

- [ ] **Step 2: Run test**

Run: `go test ./internal/agent/ -run TestDaemonEndToEnd -v -timeout 120s`
Expected: PASS (±5 detik).

- [ ] **Step 3: Commit**

```bash
git add internal/agent/daemon_test.go
git commit -m "test: daemon e2e melawan mock panel tcp"
```

---

### Task 6: oikos diagnose

**Files:**
- Create: `cmd/oikos/diagnose.go`, `cmd/oikos/diagnose_test.go`
- Test: `cmd/oikos/diagnose_test.go` (package main)

**Interfaces:**
- Consumes: config, store, docker client Ping, resource.DetectCgroupVersion.
- Produces: `runDiagnose(args []string) error` (exit 0 bila semua ok/skip, error bila ada FAIL).

- [ ] **Step 1: Write the failing test**

`cmd/oikos/diagnose_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiagnoseFreshOK(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := "node:\n  data_dir: \"" + filepath.Join(dir, "data") + "\"\nruntime:\n  engine: \"fake\"\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := runDiagnose([]string{"--config", cfgPath}); err != nil {
		t.Fatalf("diagnose fresh harus ok: %v", err)
	}
}

func TestDiagnoseFailsWhenDataDirIsFile(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "config.yaml")
	content := "node:\n  data_dir: \"" + blocker + "\"\nruntime:\n  engine: \"fake\"\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := runDiagnose([]string{"--config", cfgPath}); err == nil {
		t.Fatal("diagnose harus gagal bila data dir tidak bisa dibuat")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/oikos/ -run TestDiagnose -v`
Expected: FAIL `undefined: runDiagnose`.

- [ ] **Step 3: Write minimal implementation**

`cmd/oikos/diagnose.go`:

```go
package main

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/client"

	"github.com/oikos/oikos/internal/config"
	"github.com/oikos/oikos/internal/resource"
	"github.com/oikos/oikos/internal/store"
)

func runDiagnose(args []string) error {
	fs := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	configPath := fs.String("config", "config.yaml", "path config.yaml")
	if err := fs.Parse(args); err != nil {
		return err
	}
	failed := false
	ok := func(name, detail string) { fmt.Printf("[ok] %s: %s\n", name, detail) }
	info := func(name, detail string) { fmt.Printf("[..] %s: %s\n", name, detail) }
	fail := func(name, detail string) {
		failed = true
		fmt.Printf("[FAIL] %s: %s\n", name, detail)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fail("config", err.Error())
		return fmt.Errorf("diagnose gagal")
	}
	ok("config", fmt.Sprintf("panel=%s engine=%s", cfg.Panel.Address, cfg.Runtime.Engine))

	if err := os.MkdirAll(cfg.Node.DataDir, 0750); err != nil {
		fail("data-dir", err.Error())
	} else {
		ok("data-dir", cfg.Node.DataDir)
	}
	db, err := store.Open(filepath.Join(cfg.Node.DataDir, "oikos.db"))
	if err != nil {
		fail("sqlite-open", err.Error())
	} else {
		if err := db.Migrate(); err != nil {
			fail("sqlite-migrate", err.Error())
		} else {
			ok("sqlite", "open+migrate ok")
		}
		db.Close()
	}
	if cfg.Runtime.Engine != "docker" {
		info("docker", "dilewati (engine fake)")
	} else {
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			fail("docker-client", err.Error())
		} else if _, err := cli.Ping(ctxBackground(), ); err != nil {
			fail("docker-ping", err.Error())
		} else {
			ok("docker", "daemon reachable")
			cli.Close()
		}
	}
	info("cgroup", "versi "+resource.DetectCgroupVersion())
	if db != nil {
		if node, err := dbGetNode(db); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				info("pairing", "belum pairing")
				info("cert", "dilewati (belum pairing)")
			} else {
				fail("pairing", err.Error())
			}
		} else {
			ok("pairing", fmt.Sprintf("node %s (%s)", node.ID, node.Name))
			checkCert(node.CertPath, ok, fail)
		}
	}
	if failed {
		return fmt.Errorf("diagnose gagal")
	}
	return nil
}
```

STOP — dua masalah pada draf di atas: (1) `db` dipakai setelah `db.Close()` (use-after-close) dan butuh helper aneh; (2) `ctxBackground()` bukan fungsi. Tulis yang benar: jangan Close db sebelum cek pairing; pakai `context.Background()` langsung. Versi final:

```go
	db, err := store.Open(filepath.Join(cfg.Node.DataDir, "oikos.db"))
	if err != nil {
		fail("sqlite-open", err.Error())
		db = nil
	} else if err := db.Migrate(); err != nil {
		fail("sqlite-migrate", err.Error())
	} else {
		ok("sqlite", "open+migrate ok")
	}
	...
	if cfg.Runtime.Engine != "docker" {
		info("docker", "dilewati (engine fake)")
	} else {
		cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			fail("docker-client", err.Error())
		} else {
			pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, err := cli.Ping(pingCtx)
			cancel()
			cli.Close()
			if err != nil {
				fail("docker-ping", err.Error())
			} else {
				ok("docker", "daemon reachable")
			}
		}
	}
	...
	if db == nil {
		info("pairing", "dilewati (sqlite gagal)")
		info("cert", "dilewati (sqlite gagal)")
	} else {
		defer db.Close()
		node, err := db.GetNode()
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				info("pairing", "belum pairing")
				info("cert", "dilewati (belum pairing)")
			} else {
				fail("pairing", err.Error())
			}
		} else {
			ok("pairing", fmt.Sprintf("node %s (%s)", node.ID, node.Name))
			raw, err := os.ReadFile(node.CertPath)
			if err != nil {
				fail("cert", err.Error())
			} else if cert, err := tls.X509KeyPair(raw, mustReadKey(node.KeyPath)); err != nil {
				fail("cert", err.Error())
			} else if leaf, err := x509.ParseCertificate(cert.Certificate[0]); err != nil {
				fail("cert", err.Error())
			} else if time.Now().After(leaf.NotAfter) {
				fail("cert", "kedaluwarsa "+leaf.NotAfter.String())
			} else {
				ok("cert", "valid hingga "+leaf.NotAfter.Format(time.RFC3339))
			}
		}
	}
```

dengan helper:

```go
func mustReadKey(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		return []byte{}
	}
	return data
}
```

(X509KeyPair dengan key kosong → error yang ditangkap sebagai FAIL — perilaku benar.)

Import: context, crypto/tls, crypto/x509, database/sql, errors, flag, fmt, os, path/filepath, time, docker client, config, resource, store.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/oikos/ -v`
Expected: PASS kedua test diagnose.

- [ ] **Step 5: Commit**

```bash
git add cmd/oikos/
git commit -m "feat: oikos diagnose"
```

---

### Task 7: mock-panel dev tool + install --ca-path

**Files:**
- Create: `cmd/mock-panel/main.go`
- Modify: `cmd/oikos/main.go` (flag --ca-path di runInstall)
- Test: build + manual run (tanpa test file; tool dev).

**Interfaces:**
- Consumes: `security.GenerateCA`, `gen nodepb`.
- Produces: binary `bin/mock-panel` (`--plain-addr`, `--tls-addr`, `--tokens`, `--ca-out`).

- [ ] **Step 1: Tulis mock-panel**

`cmd/mock-panel/main.go` (dev-only, reuses pola mock test; single-use token map + sign CSR dengan CA yang di-generate saat startup):

```go
// Command mock-panel adalah Panel palsu untuk development/manual testing.
// JANGAN dipakai di produksi: token statis, tanpa autentikasi admin.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"math/big"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/google/uuid"

	nodepb "github.com/oikos/oikos/gen/go/node"
	"github.com/oikos/oikos/internal/security"
)

type panel struct {
	nodepb.UnimplementedNodeServiceServer
	mu     sync.Mutex
	tokens map[string]bool
	caCert *x509.Certificate
	caPriv ed25519.PrivateKey
}

func (p *panel) Pair(ctx context.Context, req *nodepb.PairingRequest) (*nodepb.PairingResponse, error) {
	... (sama seperti mock test: cek token, parse CSR, sign 24 jam, tandai used)
}

func (p *panel) Heartbeat(...) // catat log ke stdout, ack ok
func (p *panel) StreamCommands(...) // tahan stream terbuka hingga ctx selesai
```

STOP — draf di atas tidak lengkap (placeholder implisit). Tulis implementasi penuh saat eksekusi mengikuti pola yang sama persis dengan `panel_mock_test.go` (Pair/Heartbeat/StreamCommands), plus main(): parse flag, GenerateCA, tulis ca-out 0600, serve plain di --plain-addr dan TLS (RequireAndVerifyClientCert, cert CN panel.test dengan SAN localhost/127.0.0.1) di --tls-addr, blokir di signal. Token dari --tokens dipisah koma.

Context `context` perlu di-import untuk signature handler.

- [ ] **Step 2: Tambah flag --ca-path ke install**

`cmd/oikos/main.go` runInstall: tambah `caPath := fs.String("ca-path", "", ...)`; setelah `cfg := config.Default()`, bila `*caPath != ""` set `cfg.Panel.CAPath = *caPath`. Berlaku untuk jalur offline maupun panel.

- [ ] **Step 3: Verifikasi**

Run:

```bash
go build -o bin/mock-panel ./cmd/mock-panel && go build ./... && go test ./cmd/... 2>&1 | tail -3
```

Expected: kedua binary ter-build; test cmd hijau.

- [ ] **Step 4: Commit**

```bash
git add cmd/mock-panel/ cmd/oikos/main.go
git commit -m "feat: mock panel dev dan flag ca-path"
```

---

### Task 8: install.sh parameterisasi + e2e

**Files:**
- Modify: `scripts/install.sh`
- Test: eksekusi e2e nyata (root, tanpa systemd).

- [ ] **Step 1: Edit install.sh**

Ganti blok path hardcoded:

```bash
INSTALL_BIN="${OIKOS_INSTALL_BIN:-/usr/local/bin/oikos}"
CONFIG_DIR="${OIKOS_CONFIG_DIR:-/etc/oikos}"
DATA_DIR="${OIKOS_DATA_DIR:-/var/lib/oikos}"
SYSTEMD_DIR="${OIKOS_SYSTEMD_DIR:-/etc/systemd/system}"
```

`mkdir -p "$CONFIG_DIR/certs" "$DATA_DIR"` + chmod keduanya. `cp deploy/systemd/oikos.service "$SYSTEMD_DIR/oikos.service"` lalu:

```bash
if command -v systemctl >/dev/null 2>&1 && [[ -d /run/systemd/system ]]; then
  systemctl daemon-reload
else
  echo "systemd tidak tersedia, lewati daemon-reload/enable (jalankan binary langsung)"
fi
```

Pesan akhir disesuaikan: bila systemd tersedia → "systemctl enable --now oikos", bila tidak → "jalankan $INSTALL_BIN daemon --config $CONFIG_DIR/config.yaml".

- [ ] **Step 2: Jalankan e2e**

Run (sebagai root, dari repo root):

```bash
bash -n scripts/install.sh && echo SYNTAX-OK
export OIKOS_INSTALL_BIN=/tmp/oikos-inst/oikos OIKOS_CONFIG_DIR=/tmp/oikos-inst/etc OIKOS_DATA_DIR=/tmp/oikos-inst/data OIKOS_SYSTEMD_DIR=/tmp/oikos-inst/systemd
export OIKOS_BIN_URL=file:///root/oikos/bin/oikos OIKOS_TOKEN=$(python3 -c "import secrets; print(secrets.token_hex(32))")
rm -rf /tmp/oikos-inst && mkdir -p /tmp/oikos-inst
bash scripts/install.sh
test -x /tmp/oikos-inst/oikos && test -f /tmp/oikos-inst/etc/config.yaml && test -f /tmp/oikos-inst/systemd/oikos.service && echo INSTALL-E2E-OK
/tmp/oikos-inst/oikos diagnose --config /tmp/oikos-inst/etc/config.yaml
```

Expected: SYNTAX-OK; script selesai tanpa error; ketiga file ada; diagnose berjalan ( unpaired → info, exit 0).

Catatan: `oikos install` offline menulis config dari Default (data_dir /var/lib/oikos) — diagnose memakai config itu dan akan MkdirAll /var/lib/oikos (sebagai root, berhasil). Itu perilaku jujur script saat ini; catat bila diagnose menunjukkan data dir default.

- [ ] **Step 3: Commit**

```bash
git add scripts/install.sh
git commit -m "feat: installer dapat diparameterisasi dan e2e"
```

---

### Task 9: Stress test OOM (integration)

**Files:**
- Create: `internal/orchestrator/limits_integration_test.go` (`//go:build integration`)
- Test: file itu sendiri.

- [ ] **Step 1: Write the test**

```go
//go:build integration

package orchestrator

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/oikos/oikos/internal/resource"
	"github.com/oikos/oikos/internal/runtime/docker"
	"github.com/oikos/oikos/internal/store"
)

func ensureImageTag(t *testing.T, src, dst string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "docker", "pull", src).CombinedOutput(); err != nil {
		t.Fatalf("pull: %v\n%s", src, err, out)
	}
	if out, err := exec.CommandContext(ctx, "docker", "tag", src, dst).CombinedOutput(); err != nil {
		t.Fatalf("tag: %v\n%s", err, out)
	}
}

func TestLimitsEnforcedIntegration(t *testing.T) {
	if resource.DetectCgroupVersion() != "v2" {
		t.Skip("butuh cgroup v2")
	}
	ensureImageTag(t, "busybox:stable", "oikos/egg-hog")
	db, err := store.Open(t.TempDir() + "/o.db")
	...
```

STOP — `store.Open(t.TempDir() + "/o.db")` mengabaikan error pattern repo (`filepath.Join` + cek err). Tulis sesuai pola repo:

```go
	dbPath := filepath.Join(t.TempDir(), "o.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateEgg(store.Egg{ID: "egg-hog", Name: "hog", DockerfilePath: "d", MetadataPath: "m"}); err != nil {
		t.Fatal(err)
	}
	rt, err := docker.New("")
	if err != nil {
		t.Fatal(err)
	}
	lc := New(db, rt, NewEventBus())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err := db.SetResourceLimits(store.ResourceLimits{ServerID: "", CPULimit: 0, MemoryLimitMB: 0, PIDLimit: 0}); err != nil {
		// diabaikan? TIDAK — SetResourceLimits butuh server_id valid (FK).
	}
```

STOP — SetResourceLimits sebelum server ada melanggar FK. Urutan benar: CreateServer dulu (dapat id), lalu SetResourceLimits{ServerID: id, CPULimit: 500, MemoryLimitMB: 32, PIDLimit: 64}, lalu StartServer (startup "sleep 60"), lalu:

```go
	srv, err := db.GetServer(id)
	...
	cg, err := resource.FindContainerCgroup("/sys/fs/cgroup", srv.ContainerID)
	if err != nil {
		t.Fatal(err)
	}
	lim, err := resource.ReadLimits(cg)
	...
	if lim.MemoryBytes != 32*1024*1024 || lim.CPUMillicores != 500 || lim.PIDMax != 64 {
		t.Fatalf("limit cgroup salah: %+v", lim)
	}
	// OOM: alokasi 100MB di limit 32MB.
	if _, err := rt.Exec(ctx, srv.ContainerID, []string{"sh", "-c", "x=$(head -c 100m /dev/zero); sleep 30"}); err == nil {
		t.Log("exec selesai tanpa error (kernel mungkin menunda OOM); lanjut cek status")
	}
	deadline := time.Now().Add(60 * time.Second)
	for {
		st, err := rt.Status(ctx, srv.ContainerID)
		if err != nil {
			t.Fatal(err)
		}
		if st == runtime.StatusCrashed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("status tak kunjung crashed: %q", st)
		}
		time.Sleep(time.Second)
	}
	defer lc.DeleteServer(context.Background(), id)
```

Import: context, os/exec, path/filepath, testing, time, resource, runtime, docker, store. `rt` bertipe runtime.Runtime (docker.New mengembalikan itu) — Exec/Status tersedia di interface. `defer lc.DeleteServer` lebih baik di awal setelah create (pastikan cleanup walau assert gagal): taruh `defer lc.DeleteServer(context.Background(), id)` segera setelah StartServer sukses.

- [ ] **Step 2: Run test**

Run: `go test -tags integration ./internal/orchestrator/ -run TestLimitsEnforced -v -timeout 8m`
Expected: PASS (cgroup match + crashed via OOM).

- [ ] **Step 3: Commit**

```bash
git add internal/orchestrator/limits_integration_test.go
git commit -m "test: stress limit oom integrasi"
```

---

### Task 10: Benchmark idle + dokumen

**Files:**
- Create: `docs/benchmarks/idle-resource-usage.md`
- Test: pengukuran manual berikut (bukan go test).

- [ ] **Step 1: Siapkan loop mock panel + daemon**

Run:

```bash
make build && go build -o bin/mock-panel ./cmd/mock-panel
rm -rf /tmp/oikos-bench && mkdir -p /tmp/oikos-bench
TOK=$(python3 -c "import secrets; print(secrets.token_hex(32))"); echo $TOK > /tmp/oikos-bench/token
./bin/mock-panel --plain-addr 127.0.0.1:19091 --tls-addr 127.0.0.1:19092 --tokens "$TOK" --ca-out /tmp/oikos-bench/ca.crt > /tmp/oikos-bench/panel.log 2>&1 &
echo $! > /tmp/oikos-bench/panel.pid
./bin/oikos install --panel 127.0.0.1:19091 --token "$TOK" --name bench-node --ca-path /tmp/oikos-bench/ca.crt --config /tmp/oikos-bench/config.yaml
```

Expected: install mencetak node id; panel.log menunjukkan Paird.

- [ ] **Step 2: Jalankan daemon dan ukur 60 detik**

Run:

```bash
./bin/oikos daemon --config /tmp/oikos-bench/config.yaml > /tmp/oikos-bench/daemon.log 2>&1 &
DPID=$!; echo $DPID
sleep 5; grep -c bench /tmp/oikos-bench/panel.log  # bukti heartbeat jalan (mock log node id)
python3 - <<'EOF'
import time
rss=[]; cpu=[]
def read(pid):
    with open(f'/proc/{pid}/status') as f:
        for line in f:
            if line.startswith('VmRSS:'): r=int(line.split()[1])
    with open(f'/proc/{pid}/stat') as f:
        p=f.read().rsplit(')',1)[1].split(); u=int(p[11]); s=int(p[12])
    return r,u+s
import os
pid=open('/tmp/oikos-bench/dpid').read().strip() if False else None
EOF
```

STOP — draf python di atas berantakan (logika pid setengah jadi). Ganti dengan skrip pengukuran eksplisit: PID daemon diketahui dari `$DPID` shell; tulis ke file lalu python baca file itu:

```bash
./bin/oikos daemon --config /tmp/oikos-bench/config.yaml > /tmp/oikos-bench/daemon.log 2>&1 &
echo $! > /tmp/oikos-bench/daemon.pid
sleep 5
python3 - <<'EOF'
import time
pid = open('/tmp/oikos-bench/daemon.pid').read().strip()
def snap():
    rss = next(int(l.split()[1]) for l in open(f'/proc/{pid}/status') if l.startswith('VmRSS:'))
    p = open(f'/proc/{pid}/stat').read().rsplit(')', 1)[1].split()
    return rss, int(p[11]) + int(p[12])
samples = []
prev = snap()
for _ in range(12):
    time.sleep(5)
    cur = snap()
    samples.append((cur[0], cur[1] - prev[1]))
    prev = cur
rss_vals = [s[0] for s in samples]
cpu_vals = [c / 500.0 * 100.0 for _, c in samples]  # 5 detik * 100 jiffies
print(f"RSS_KB min={min(rss_vals)} max={max(rss_vals)} avg={sum(rss_vals)//len(rss_vals)}")
print(f"CPU_PCT max={max(cpu_vals):.2f} avg={sum(cpu_vals)/len(cpu_vals):.2f}")
EOF
kill $(cat /tmp/oikos-bench/daemon.pid); kill $(cat /tmp/oikos-bench/panel.pid)
```

Expected: angka RSS_KB dan CPU_PCT tercetak.

- [ ] **Step 3: Tulis dokumen dengan angka hasil run**

`docs/benchmarks/idle-resource-usage.md`: metodologi (binary `oikos daemon` + mock-panel lokal, engine docker tanpa container, 12 sampel @5 detik, kernel jiffies 100Hz), tabel hasil (RSS min/max/avg dalam MB, CPU max/avg dalam %), bandingkan dengan ambang DoD Fase 1 (30MB, 1%), dan catatan jujur: perbandingan vs Wings + skala 10/50/100 server = ranah Fase 6 (TBD di sana, bukan di sini).

- [ ] **Step 4: Commit**

```bash
git add docs/benchmarks/idle-resource-usage.md
git commit -m "docs: benchmark idle daemon fase 1"
```

---

### Task 11: Verifikasi akhir Fase 1

**Files:** tidak ada (verifikasi + commit sisa bila ada).

- [ ] **Step 1: Suite penuh + vet + fmt + integration docker**

Run:

```bash
make proto && make build && go test ./... && go vet ./... && gofmt -l cmd internal gen; echo VET-FMT-OK
go test -tags integration ./internal/runtime/docker/ ./internal/orchestrator/ -timeout 10m 2>&1 | tail -3
```

Expected: semua PASS.

- [ ] **Step 2: Checklist DoD Fase 1** — nyatakan status tiap item di pesan final (bukan file): install satu perintah (terbukti Task 8), pairing token→mTLS (Task 5 ronde lalu), lifecycle via gRPC/CLI (Task 3 + fondasi), egg build dari Dockerfile (BuildImage ada + terpakai manual? JUJUR: BuildImage belum dipanggil orchestrator/CLI — catat sebagai gap kecil: tambahkan pemakaian? Di luar task; catat di final sebagai follow-up Fase 2 atau terima karena egg contoh bisa di-build manual via runtime. Jangan disembunyikan.), limit diterapkan (Task 2+9), idle (Task 10), mTLS + sekali pakai (ronde lalu), unit test lifecycle+validator (ada).

- [ ] **Step 3: Commit sisa**

```bash
git add -A && git commit -m "chore: finalisasi fase 1" || echo "nothing to commit"
git log --oneline | head -20
```

---

## Self-Review

- Cakupan: limit (rencana #6), runtime abstraction dipakai penuh, security-first lanjutan (diagnose), zero-config (installer e2e), API-first (ServerService + daemon+API lokal), egg Dockerfile (contoh + BuildImage tersedia; gap pemakaian otomatis dicatat jujur di Task 11), ringan (benchmark).
- Placeholder: draf salah pada Task 3/6/9/10 sudah diperbaiki inline di dokumen ini (dialer bufconn, db use-after-close, SetResourceLimits FK, skrip python). `cmd/mock-panel` ditulis penuh saat eksekusi mengikuti pola mock test (bukan stub).
- Konsistensi tipe: `serverpb` = `gen/go/server`; `api.NewServer(lc) *grpc.Server`; `agent.Run(ctx, DaemonArgs, lc)`; `DaemonArgs` field sama di daemon.go dan daemon_test; `resource.Limits` dipakai enforcer + lifecycle + stress test; itoa → `strconv.Itoa`.
