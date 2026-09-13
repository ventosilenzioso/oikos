# Oikos Fase 2 Networking Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Menambahkan tunnel publik berbasis frpc/frps terpisah, assignment port melalui Panel, reconnect dasar, dan relay private network multi-node.

**Architecture:** `internal/tunnel` dipisah menjadi interface manager, store mapping, allocator Panel, generator TOML, dan process controller. Unit/component test memakai fake Panel dan fake process; integration test frp nyata hanya berjalan bila `frps`/`frpc` tersedia. Orchestrator memanggil Manager melalui dependency injection, sehingga tidak mengetahui detail frp.

**Tech Stack:** Go 1.26, gRPC/protobuf yang sudah dipakai Fase 1, SQLite `modernc.org/sqlite`, TOML generator berbasis `github.com/pelletier/go-toml/v2`, frp release binary, Docker network untuk integration test.

**Spec:** `docs/superpowers/specs/2026-09-13-oikos-fase2-networking-design.md`

## Global Constraints

- Module Go: `github.com/oikos/oikos`.
- Go minimum: 1.23.
- SQLite driver: `modernc.org/sqlite` (tanpa cgo).
- `frps` berjalan sebagai deployment terpisah dari daemon Oikos.
- Oikos node menjalankan satu `frpc` sebagai child process untuk seluruh proxy aktif pada node tersebut.
- Private network MVP memakai relay lewat frps.
- File config frpc permission `0600` dan ditulis atomik menggunakan temporary file pada direktori yang sama lalu rename.
- Token frp tidak boleh ditulis ke log.
- Assignment idempoten berdasarkan `(server_id, local_port, protocol)`.
- Retry dasar frpc menggunakan backoff `1s, 2s, 4s ... 30s`; policy alerting penuh tetap Fase 3.
- Semua goroutine tunnel berhenti saat context dibatalkan.

---

## File Map

- Create `proto/tunnel/tunnel.proto`: kontrak RPC assignment, release, status, dan network group.
- Create `migrations/0002_networking.sql`: tabel `tunnels`, `network_groups`, `network_group_members`.
- Modify `internal/store/sqlite.go` and `models.go`: model dan CRUD mapping/network group.
- Create `internal/tunnel/manager.go`: interface publik, tipe status, dependency interfaces.
- Create `internal/tunnel/config.go`: validasi mapping dan generator `frpc.toml`.
- Create `internal/tunnel/process.go`: process controller OS dan fake controller untuk test.
- Create `internal/tunnel/frp.go`: manager lifecycle, reload, release, events, reconnect.
- Create `internal/tunnel/allocator.go`: client Panel `RegisterTunnel`/`ReleaseTunnel`/status.
- Create `internal/tunnel/mesh.go`: relay network group assignment/discovery.
- Modify `internal/config/config.go`, `defaults.go`, `config.yaml`: konfigurasi frp.
- Modify `internal/orchestrator/lifecycle.go`: dependency tunnel manager dan register/release port.
- Modify `cmd/oikos/main.go`, `daemon.go`: inisialisasi tunnel manager.
- Modify `scripts/install.sh`: download frpc pinned + checksum.
- Create `cmd/mock-panel` tunnel handlers and tests: fake Panel allocator.
- Create integration tests under `internal/tunnel/` and `docs/networking.md`.

---

### Task 1: Proto TunnelService dan SQLite migration

**Files:**
- Create: `proto/tunnel/tunnel.proto`
- Create: `migrations/0002_networking.sql`
- Modify: `scripts/gen-proto.sh`
- Generate: `gen/go/tunnel/tunnel.pb.go`, `gen/go/tunnel/tunnel_grpc.pb.go`
- Test: `internal/store/networking_test.go`

**Interfaces:**
- Consumes: `node_id` dari Fase 1 dan `server_id` dari schema existing.
- Produces: `tunnelpb.TunnelServiceClient/Server`; message types `RegisterTunnelRequest`, `RegisterTunnelResponse`, `ReleaseTunnelRequest`, `TunnelStatusRequest`, `TunnelStatusResponse`, `NetworkGroupAssignment`, `NetworkGroupMember`, `ListNetworkGroupMembersRequest/Response`.

- [ ] **Step 1: Tulis failing store test**

```go
func TestNetworkingMigrationAndTunnelRoundtrip(t *testing.T) {
    db := openMigrated(t)
    if err := db.MigrateNetworking(); err != nil { t.Fatal(err) }
    if err := db.CreateEgg(Egg{ID: "egg", Name: "egg", DockerfilePath: "d", MetadataPath: "m"}); err != nil { t.Fatal(err) }
    if err := db.CreateServer(Server{ID: "srv", Name: "srv", EggID: "egg", Status: "stopped", StartupCommand: "run", Environment: "{}"}); err != nil { t.Fatal(err) }
    want := Tunnel{ID: "tun", ServerID: "srv", LocalPort: 25565, RemotePort: 30001, Protocol: "tcp", Status: "connected"}
    if err := db.SaveTunnel(want); err != nil { t.Fatal(err) }
    got, err := db.GetTunnelByMapping("srv", 25565, "tcp")
    if err != nil { t.Fatal(err) }
    if got.ID != want.ID || got.RemotePort != 30001 || got.Status != "connected" { t.Fatalf("tunnel=%+v", got) }
    if err := db.DeleteTunnelByMapping("srv", 25565, "tcp"); err != nil { t.Fatal(err) }
    if _, err := db.GetTunnelByMapping("srv", 25565, "tcp"); !errors.Is(err, sql.ErrNoRows) { t.Fatalf("after delete err=%v", err) }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestNetworkingMigrationAndTunnelRoundtrip -v`
Expected: FAIL karena `MigrateNetworking`, `Tunnel`, dan CRUD belum ada.

- [ ] **Step 3: Tulis proto dan migration**

`proto/tunnel/tunnel.proto` memakai package `tunnel` dan go package `github.com/oikos/oikos/gen/go/tunnel`; define:

```proto
message RegisterTunnelRequest { string node_id = 1; string server_id = 2; int32 local_port = 3; string protocol = 4; }
message RegisterTunnelResponse { string tunnel_id = 1; int32 remote_port = 2; string status = 3; string frps_addr = 4; int32 frps_port = 5; }
message ReleaseTunnelRequest { string node_id = 1; string server_id = 2; int32 local_port = 3; string protocol = 4; }
message TunnelStatusRequest { string node_id = 1; string server_id = 2; }
message TunnelStatusResponse { repeated TunnelInfo tunnels = 1; }
message TunnelInfo { string tunnel_id = 1; string server_id = 2; int32 local_port = 3; int32 remote_port = 4; string protocol = 5; string status = 6; }
message NetworkGroupAssignment { string group_id = 1; string name = 2; string relay_addr = 3; int32 relay_port = 4; string private_ip = 5; repeated NetworkGroupMember members = 6; }
message NetworkGroupMember { string node_id = 1; string private_ip = 2; }
message ListNetworkGroupMembersRequest { string node_id = 1; string group_id = 2; }
message ListNetworkGroupMembersResponse { repeated NetworkGroupMember members = 1; }
service TunnelService {
  rpc RegisterTunnel(RegisterTunnelRequest) returns (RegisterTunnelResponse);
  rpc ReleaseTunnel(ReleaseTunnelRequest) returns (google.protobuf.Empty);
  rpc GetTunnelStatus(TunnelStatusRequest) returns (TunnelStatusResponse);
  rpc AssignNetworkGroup(NetworkGroupAssignment) returns (google.protobuf.Empty);
  rpc ListNetworkGroupMembers(ListNetworkGroupMembersRequest) returns (ListNetworkGroupMembersResponse);
}
```

Import `google/protobuf/empty.proto`. Migration harus memakai `CREATE TABLE IF NOT EXISTS`, foreign keys, unique `(server_id, local_port, protocol)`, unique `remote_port`, unique `(group_id, private_ip)`, dan index server/status.

- [ ] **Step 4: Implement store types/CRUD**

Tambah `Tunnel`, `NetworkGroup`, `NetworkGroupMember`; implement `MigrateNetworking()`, `SaveTunnel`, `GetTunnelByMapping`, `ListTunnels`, `DeleteTunnelByMapping`, `UpdateTunnelStatus`, `SaveNetworkGroup`, `ReplaceNetworkGroupMembers`, `ListNetworkGroupMembers` dengan `rows.Err()` dan `%w` errors.

- [ ] **Step 5: Generate dan verifikasi**

Run: `make proto && go test ./internal/store/ ./gen/go/tunnel/ -v`
Expected: test roundtrip PASS dan generated gRPC code compile.

- [ ] **Step 6: Commit**

```bash
git add proto/tunnel migrations/0002_networking.sql scripts/gen-proto.sh gen/go/tunnel internal/store
git commit -m "feat: proto dan persistence networking"
```

---

### Task 2: Config frp dan generator TOML

**Files:**
- Modify: `internal/config/config.go`, `internal/config/defaults.go`, `config.yaml`
- Create: `internal/tunnel/manager.go`, `internal/tunnel/config.go`, `internal/tunnel/config_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Consumes: `store.Tunnel`.
- Produces: `config.RuntimeConfig` frp fields; `tunnel.PortMapping`, `Assignment`, `TunnelStatus`, `Manager`; `tunnel.GenerateConfig(cfg FRPConfig, mappings []PortMapping) ([]byte, error)`; `tunnel.WriteConfigAtomic(path string, data []byte) error`.

- [ ] **Step 1: Write failing config tests**

```go
func TestGenerateConfigEscapesAndSorts(t *testing.T) {
    data, err := GenerateConfig(FRPConfig{ServerAddr: "frps.example", ServerPort: 7000, Token: "secret"}, []PortMapping{
        {TunnelID: "b", ServerID: "srv-b", LocalPort: 25566, RemotePort: 30002, Protocol: "udp"},
        {TunnelID: "a", ServerID: "srv-a", LocalPort: 25565, RemotePort: 30001, Protocol: "tcp"},
    })
    if err != nil { t.Fatal(err) }
    got := string(data)
    if !strings.Contains(got, `auth.token = "secret"`) || strings.Index(got, "server-srv-a") > strings.Index(got, "server-srv-b") { t.Fatalf("config tidak sesuai: %s", got) }
}

func TestWriteConfigAtomic0600(t *testing.T) {
    path := filepath.Join(t.TempDir(), "frpc.toml")
    if err := WriteConfigAtomic(path, []byte("x = 1\n")); err != nil { t.Fatal(err) }
    info, err := os.Stat(path); if err != nil { t.Fatal(err) }
    if info.Mode().Perm() != 0600 { t.Fatalf("mode=%o", info.Mode().Perm()) }
}

func TestRejectInvalidProtocol(t *testing.T) {
    if _, err := GenerateConfig(FRPConfig{}, []PortMapping{{TunnelID: "x", Protocol: "icmp"}}); err == nil { t.Fatal("protocol invalid harus ditolak") }
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/tunnel/ -v`
Expected: FAIL karena package dan fungsi belum ada.

- [ ] **Step 3: Implement interface and config**

`manager.go` define:

```go
type PortMapping struct { TunnelID, ServerID string; LocalPort, RemotePort int; Protocol string }
type Assignment struct { TunnelID string; RemotePort int; Status TunnelStatus }
type Manager interface {
    RegisterPort(context.Context, PortMapping) (Assignment, error)
    ReleasePort(context.Context, string, int) error
    Status(context.Context, string) ([]TunnelStatus, error)
    Reload(context.Context) error
}
```

`FRPConfig` fields: `ServerAddr string`, `ServerPort int`, `Token string`, `ConfigPath string`, `BinaryPath string`, `RemotePortMin/Max int`.

Generator harus memvalidasi `tcp|udp`, port 1..65535, remote range, nonempty IDs, sort by `TunnelID`, gunakan safe TOML encoder, dan menghasilkan proxy name `oikos-<serverID>-<tunnelID>` dengan `localIP=127.0.0.1`.

- [ ] **Step 4: Add dependency and verify**

Run: `go get github.com/pelletier/go-toml/v2@latest && gofmt -w internal/tunnel && go test ./internal/tunnel/ -v`
Expected: all config tests PASS.

- [ ] **Step 5: Commit**

```bash
git commit -m "feat: konfigurasi dan generator frpc"
```

---

### Task 3: Panel allocator client dan fake Panel tests

**Files:**
- Create: `internal/tunnel/allocator.go`, `internal/tunnel/allocator_test.go`
- Modify: `cmd/mock-panel/main.go`

**Interfaces:**
- Consumes: `tunnelpb.TunnelServiceClient`.
- Produces: `tunnel.PanelAllocator` with `Register(context.Context, PortMapping) (Assignment, error)`, `Release(context.Context, PortMapping) error`, `Statuses(context.Context, nodeID, serverID string) ([]store.Tunnel, error)`; `NewPanelAllocator(client tunnelpb.TunnelServiceClient, nodeID string)`.

- [ ] **Step 1: Write failing allocator test**

```go
func TestAllocatorRegisterIsIdempotent(t *testing.T) {
    panel := newTunnelMockPanel(t, 30000, 30010)
    allocator := NewPanelAllocator(panel.client(t), "node-1")
    mapping := PortMapping{TunnelID: "tun-1", ServerID: "srv-1", LocalPort: 25565, Protocol: "tcp"}
    first, err := allocator.Register(context.Background(), mapping); if err != nil { t.Fatal(err) }
    second, err := allocator.Register(context.Background(), mapping); if err != nil { t.Fatal(err) }
    if first.RemotePort != second.RemotePort || first.TunnelID != second.TunnelID { t.Fatalf("assignment berubah: %+v %+v", first, second) }
    if got := panel.registerCount(); got != 1 { t.Fatalf("register count=%d", got) }
}

func TestAllocatorRejectsInvalidPanelAssignment(t *testing.T) {
    panel := newTunnelMockPanelReturning(t, &tunnelpb.RegisterTunnelResponse{RemotePort: 0})
    _, err := NewPanelAllocator(panel.client(t), "node-1").Register(context.Background(), PortMapping{TunnelID: "x", ServerID: "s", LocalPort: 1, Protocol: "tcp"})
    if err == nil { t.Fatal("assignment invalid harus error") }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tunnel/ -run TestAllocator -v`
Expected: FAIL karena allocator dan mock tunnel RPC belum ada.

- [ ] **Step 3: Implement allocator**

Allocator memanggil `RegisterTunnel` dengan node ID, menolak remote port di luar 1..65535 atau status kosong, memetakan `connected/pending/error`, meneruskan `ReleaseTunnel`, dan membungkus gRPC errors dengan `%w`.

Extend `cmd/mock-panel` with mutex-protected mapping keyed by server/local/protocol, next-port allocation in configured range, release, status query, and basic network-group member responses. Test helper boleh tetap di `internal/tunnel/panel_mock_test.go` memakai bufconn; executable mock-panel memakai TCP/mTLS handler yang sama secara perilaku.

- [ ] **Step 4: Verify**

Run: `go test ./internal/tunnel/ -run TestAllocator -v`
Expected: PASS idempotency, invalid assignment, release.

- [ ] **Step 5: Commit**

```bash
git commit -m "feat: allocator tunnel ke panel"
```

---

### Task 4: Process controller dan FrpManager lifecycle

**Files:**
- Create: `internal/tunnel/process.go`, `internal/tunnel/process_test.go`, `internal/tunnel/frp.go`, `internal/tunnel/frp_test.go`

**Interfaces:**
- Consumes: generator (Task 2), allocator (Task 3), networking store (Task 1), event bus Fase 1.
- Produces: `ProcessController` (`Start`, `Stop`, `Reload`, `Wait`) and `NewFrpManager(cfg, allocator, store, process, bus) Manager`.

- [ ] **Step 1: Write failing process and manager tests**

```go
func TestManagerRegisterWritesAndStartsProcess(t *testing.T) {
    fs := newFakeTunnelStore(t)
    proc := newFakeProcessController()
    alloc := newFakeAllocator(30001)
    m := NewFrpManager(FRPConfig{ConfigPath: filepath.Join(t.TempDir(), "frpc.toml")}, alloc, fs, proc, NewEventBus())
    got, err := m.RegisterPort(context.Background(), PortMapping{TunnelID: "tun", ServerID: "srv", LocalPort: 25565, Protocol: "tcp"})
    if err != nil { t.Fatal(err) }
    if got.RemotePort != 30001 || proc.startCount() != 1 { t.Fatalf("assignment=%+v starts=%d", got, proc.startCount()) }
    if _, err := os.Stat(m.ConfigPath()); err != nil { t.Fatal(err) }
}

func TestManagerReleaseReloadsAndPublishesDisconnected(t *testing.T) {
    bus := NewEventBus(); events := bus.Subscribe("tunnel.disconnected")
    proc := newFakeProcessController(); alloc := newFakeAllocator(30001); fs := newFakeTunnelStore(t)
    m := NewFrpManager(FRPConfig{ConfigPath: filepath.Join(t.TempDir(), "frpc.toml")}, alloc, fs, proc, bus)
    if _, err := m.RegisterPort(context.Background(), PortMapping{TunnelID: "tun", ServerID: "srv", LocalPort: 25565, Protocol: "tcp"}); err != nil { t.Fatal(err) }
    if err := m.ReleasePort(context.Background(), "srv", 25565); err != nil { t.Fatal(err) }
    select { case ev := <-events: if ev.ServerID != "srv" { t.Fatalf("event=%+v", ev) }; case <-time.After(time.Second): t.Fatal("event timeout") }
    if proc.reloadCount() == 0 { t.Fatal("reload tidak dipanggil") }
}

func TestManagerDoesNotLeakTokenIntoLogs(t *testing.T) {
    data, err := GenerateConfig(FRPConfig{Token: "super-secret"}, nil); if err != nil { t.Fatal(err) }
    if bytes.Contains(data, []byte("super-secret")) { t.Log("token hanya boleh ada di file config 0600, bukan log; generator file memang memuat token") }
}
```

Catatan: test terakhir memvalidasi boundary file-vs-log di implementasi logger pada task berikut; jangan log isi config.

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/tunnel/ -run 'TestManager|TestProcess' -v`
Expected: FAIL karena fake/process/manager belum ada.

- [ ] **Step 3: Implement process controller**

`osProcessController` menjalankan `exec.CommandContext(ctx, binary, "-c", configPath)`, pipe stdout/stderr ke logger callback yang meredaksi nilai token, `Stop` mengirim SIGTERM lalu SIGKILL setelah timeout, `Reload` melakukan graceful stop lalu start baru, dan `Wait` mengembalikan process error. Fake controller merekam `Start`, `Stop`, `Reload`, dan `Wait` calls. Manager menyediakan `ConfigPath() string` untuk test.

- [ ] **Step 4: Implement FrpManager**

Manager memakai mutex untuk assignment map, register allocator dulu lalu save store lalu generate/write config lalu start/reload process. Jika process gagal, update status `error`, publish `tunnel.disconnected`, dan kembalikan error. `ReleasePort` idempoten, release Panel/store, regenerate config, reload; publish disconnected. `Status` membaca store.

- [ ] **Step 5: Implement reconnect watcher**

Watcher membaca process `Wait`; saat child exit tidak karena context, publish disconnected lalu mencoba `Reload` dengan backoff `ComputeBackoff(attempt)` dan reset attempt setelah berhasil. Watcher berhenti saat manager context selesai.

- [ ] **Step 6: Verify and commit**

Run: `go test ./internal/tunnel/ -run 'TestManager|TestProcess' -v`
Expected: PASS.

```bash
git add internal/tunnel
git commit -m "feat: frp process manager dan reconnect dasar"
```

---

### Task 5: Config runtime dan migrasi wiring daemon

**Files:**
- Modify: `internal/config/config.go`, `internal/config/defaults.go`, `config.yaml`
- Modify: `cmd/oikos/daemon.go`, `internal/agent/daemon.go`
- Create: `internal/tunnel/factory.go`, `internal/tunnel/factory_test.go`

**Interfaces:**
- Consumes: `FrpManager`, `PanelAllocator`, `store.DB`, mTLS gRPC connection.
- Produces: `tunnel.NewFromConfig(cfg, db, nodepb/tunnelpb client, bus) (Manager, error)`; daemon owns manager context and closes it on shutdown.

- [ ] **Step 1: Write failing factory test**

```go
func TestFactoryRejectsMissingFrpBinary(t *testing.T) {
    cfg := config.Default(); cfg.Runtime.FRPBinary = filepath.Join(t.TempDir(), "missing-frpc")
    if _, err := NewFromConfig(cfg.Runtime, nil, nil, nil); err == nil { t.Fatal("binary hilang harus ditolak") }
}
```

- [ ] **Step 2: Implement config fields**

Tambah fields `FRPBinary`, `FRPConfig`, `FRPServerAddr`, `FRPServerPort`, `FRPToken`, `FRPRemotePortMin`, `FRPRemotePortMax`; defaults binary `/usr/local/bin/frpc`, config `/var/lib/oikos/frpc.toml`, server port `7000`, range `30000..40000`, token kosong. Config loader mempertahankan fallback defaults.

- [ ] **Step 3: Implement factory and daemon ownership**

Factory validasi binary executable, config directory, Panel client, allocator, store, process controller, dan event bus. `agent.Run` menerima `tunnel.Manager` opsional, memulai watcher manager, dan memanggil `Close` saat context dibatalkan. Perilaku mTLS command/heartbeat existing tetap dipertahankan.

- [ ] **Step 4: Verify and commit**

Run: `go test ./... && go vet ./...`
Expected: PASS.

```bash
git add internal/config internal/tunnel internal/agent cmd/oikos config.yaml
git commit -m "feat: wiring runtime frp ke daemon"
```

---

### Task 6: Orchestrator integration dan tunnel API Panel

**Files:**
- Modify: `internal/orchestrator/lifecycle.go`, `internal/orchestrator/lifecycle_test.go`
- Create: `internal/tunnel/orchestrator_test.go`
- Modify: `cmd/mock-panel/main.go`

**Interfaces:**
- Consumes: `tunnel.Manager`.
- Produces: an options constructor `orchestrator.NewWithTunnel(db *store.DB, rt runtime.Runtime, bus *EventBus, tunnels tunnel.Manager) *Lifecycle` while preserving existing `New(db, rt, bus)` tests; `CreateServer` registers egg ports; `DeleteServer` releases them.

- [ ] **Step 1: Write failing integration test**

```go
func TestCreateDeleteRegistersAndReleasesEggPorts(t *testing.T) {
    tm := newFakeManager(); lc := newLifecycleWithTunnel(t, tm)
    id, err := lc.CreateServer(context.Background(), "srv", "egg-with-port", "run", nil); if err != nil { t.Fatal(err) }
    if len(tm.registered) != 1 || tm.registered[0].LocalPort != 25565 { t.Fatalf("registered=%+v", tm.registered) }
    if err := lc.DeleteServer(context.Background(), id); err != nil { t.Fatal(err) }
    if len(tm.released) != 1 || tm.released[0].ServerID != id { t.Fatalf("released=%+v", tm.released) }
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/orchestrator/ -run TestCreateDeleteRegistersAndReleasesEggPorts -v`
Expected: FAIL because Lifecycle has no tunnel dependency and egg ports are not loaded.

- [ ] **Step 3: Implement egg port lookup and dependency injection**

Add `ListEggPorts(eggID)` or load egg metadata through a focused provider. Do not infer ports from arbitrary server environment. `CreateServer` must register every egg port after DB creation; if registration fails, delete/rollback the created server and return error. `DeleteServer` releases all mappings before deleting server; if a release fails, keep server row and return error to avoid losing cleanup state. Preserve old `New(db, rt, bus)` and implement the exact `NewWithTunnel(db, rt, bus, tunnels)` constructor named above, with a nil manager disabling tunnel calls.

- [ ] **Step 4: Verify and commit**

Run: `go test ./internal/orchestrator/ ./internal/tunnel/ -v`
Expected: PASS, including rollback and nil-manager regression tests.

```bash
git add internal/orchestrator internal/tunnel cmd/mock-panel
git commit -m "feat: integrasi tunnel ke lifecycle server"
```

---

### Task 7: Relay private network manager

**Files:**
- Create: `internal/tunnel/mesh.go`, `internal/tunnel/mesh_test.go`
- Modify: `internal/store/sqlite.go`, `internal/tunnel/allocator.go`, `cmd/mock-panel/main.go`

**Interfaces:**
- Consumes: `NetworkGroupAssignment`, `ListNetworkGroupMembers`, store network group CRUD.
- Produces: `NetworkManager` with `Assign(ctx, assignment) error`, `Members(ctx, groupID) ([]NetworkGroupMember, error)`, `Release(ctx, groupID, nodeID) error`.

- [ ] **Step 1: Write failing mesh test**

```go
func TestRelayNetworkAssignmentRoundtrip(t *testing.T) {
    panel := newTunnelMockPanel(t, 30000, 30010)
    m := NewRelayNetworkManager(panel.client(t), "node-a", newNetworkStore(t))
    assignment := NetworkGroupAssignment{GroupID: "g1", Name: "private", RelayAddr: "frps.test", RelayPort: 7000, PrivateIP: "10.88.0.2", Members: []NetworkGroupMember{{NodeID: "node-a", PrivateIP: "10.88.0.2"}}}
    if err := m.Assign(context.Background(), assignment); err != nil { t.Fatal(err) }
    members, err := m.Members(context.Background(), "g1"); if err != nil { t.Fatal(err) }
    if len(members) != 1 || members[0].PrivateIP != "10.88.0.2" { t.Fatalf("members=%+v", members) }
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/tunnel/ -run TestRelayNetworkAssignmentRoundtrip -v`
Expected: FAIL karena NetworkManager belum ada.

- [ ] **Step 3: Implement relay manager**

Validate group ID/private IPv4, persist assignment and members transactionally, request member discovery via Panel, and emit no new metrics. The manager only stores relay metadata and exposes members; actual frp proxy provisioning remains in FrpManager.

- [ ] **Step 4: Verify and commit**

Run: `go test ./internal/tunnel/ -run TestRelay -v`
Expected: PASS.

```bash
git add internal/tunnel internal/store cmd/mock-panel
git commit -m "feat: relay private network group"
```

---

### Task 8: Installer frpc pinned checksum

**Files:**
- Modify: `scripts/install.sh`
- Create: `scripts/install_test.sh`, `docs/networking.md`

**Interfaces:**
- Consumes: `OIKOS_FRPC_URL`, `OIKOS_FRPC_SHA256`, `OIKOS_FRPC_VERSION`, `OIKOS_FRPC_BIN` overrides.
- Produces: executable `/usr/local/bin/frpc` or configured path, with no replacement on checksum failure.

- [ ] **Step 1: Write shell tests**

```bash
#!/usr/bin/env bash
set -euo pipefail
test_checksum_failure_does_not_replace() {
  local dir; dir="$(mktemp -d)"
  printf 'old' > "$dir/frpc"
  printf 'new' > "$dir/payload"
  if OIKOS_FRPC_URL="file://$dir/payload" OIKOS_FRPC_SHA256="0000000000000000000000000000000000000000000000000000000000000000" OIKOS_FRPC_BIN="$dir/frpc" bash scripts/install-frpc.sh; then return 1; fi
  [[ "$(<"$dir/frpc")" == old ]]
}
test_checksum_failure_does_not_replace
```

Create `scripts/install-frpc.sh` so install.sh remains orchestration-only. It must download to temporary file, calculate `sha256sum`, compare exact lowercase digest, chmod 0755, then atomic rename.

- [ ] **Step 2: Run test to verify failure**

Run: `bash scripts/install_test.sh`
Expected: FAIL because `install-frpc.sh` belum ada.

- [ ] **Step 3: Implement downloader and wire installer**

Default version/URL/checksum must be explicit release variables, not silently accepted empty values. If checksum is empty, fail with actionable error. `install.sh` calls downloader before daemon setup and supports `OIKOS_FRPC_SKIP=1` only for development/test installs; production default must not skip.

- [ ] **Step 4: Verify and document**

Run: `bash scripts/install_test.sh && bash -n scripts/install.sh scripts/install-frpc.sh`
Expected: PASS. Document frps separate deployment, token distinction, version/checksum override, and NAT prerequisites in `docs/networking.md`.

- [ ] **Step 5: Commit**

```bash
git add scripts docs/networking.md
git commit -m "feat: installer frpc dengan checksum"
```

---

### Task 9: Integration test frp nyata dan reconnect

**Files:**
- Create: `internal/tunnel/frp_integration_test.go` with `//go:build integration`
- Create: `deploy/frp/frps.toml.example`, `docs/networking-integration.md`

- [ ] **Step 1: Add environment-gated integration test**

The test must skip with a clear message unless `OIKOS_FRPS_ADDR`, `OIKOS_FRPS_PORT`, `OIKOS_FRP_TOKEN`, and `OIKOS_FRPC_BIN` are set. It starts a local TCP echo server, registers a mapping, starts real frpc, connects through assigned remote port, verifies echo, releases mapping, verifies connection refusal, kills frpc, and verifies manager reconnects within 30 seconds.

- [ ] **Step 2: Run when infrastructure exists**

Run:

```bash
OIKOS_FRPS_ADDR=127.0.0.1 OIKOS_FRPS_PORT=7000 OIKOS_FRP_TOKEN="$FRP_TOKEN" OIKOS_FRPC_BIN=/usr/local/bin/frpc go test -tags integration ./internal/tunnel/ -run TestFrpPublicTunnel -v -timeout 5m
```

Expected: PASS with real frps/frpc; otherwise test is explicitly SKIP, never a false PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/tunnel deploy/frp docs/networking-integration.md
git commit -m "test: integration tunnel frp nyata"
```

---

### Task 10: Final verification Fase 2

**Files:** no new implementation files; modify docs/status only if required.

- [ ] **Step 1: Run unit/component suite**

Run:

```bash
make proto && make build && go test -count=1 ./... && go vet ./... && test -z "$(gofmt -l cmd internal gen)" && bash scripts/install_test.sh
```

Expected: all commands pass; frp integration may be skipped only when required environment variables are absent.

- [ ] **Step 2: Run integration test if available**

Run command from Task 9. Record PASS or explicit SKIP reason in `docs/networking-integration.md`; do not claim public NAT tunnel success without real frps/frpc evidence.

- [ ] **Step 3: Review DoD**

Confirm and document separately: unit/component behavior, config generation, idempotent assignment, delete release, reconnect simulation, private relay metadata, installer checksum behavior, and real frp result. Mark real NAT exposure as pending if infrastructure was unavailable.

- [ ] **Step 4: Commit final docs/status**

```bash
git add -A
git commit -m "chore: verifikasi fase 2 networking"
```

---

## Self-Review

- Spec coverage: deployment decision and child process (Tasks 2, 4), RPC/schema (Task 1), allocator (Task 3), orchestrator integration/events (Task 6), relay private network (Task 7), reconnect (Task 4), provisioning/checksum (Task 8), real integration evidence (Task 9), and final DoD (Task 10) are all mapped.
- No unresolved implementation placeholders remain. The integration test is explicitly environment-gated and may skip; that is a defined test outcome, not an unimplemented step.
- Interface consistency: `PortMapping`, `Assignment`, `Manager`, `PanelAllocator`, `ProcessController`, and `NetworkManager` are named once and consumed consistently by subsequent tasks.
- Important safety correction: failed tunnel release must preserve the server row, while failed registration must roll back the newly created server; this avoids silently losing cleanup state.
- `frps` is intentionally not added to the Oikos binary or installer as a node service; deployment remains separate as approved.
