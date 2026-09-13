# Oikos Fase 5 Extensibility and Portability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Menambahkan plugin subprocess berbasis gRPC Unix socket, graceful update dengan rollback, portability boundary, dan `oikos doctor`.

**Architecture:** Plugin dijalankan sebagai proses terpisah dan hanya menerima capability yang diizinkan manifest. Update menyimpan binary pada versioned slots dan mengganti symlink `current` secara atomik setelah checksum, signature, dan health-check lulus. Fitur Linux-specific berada di balik interface/build tags.

**Tech Stack:** Go 1.26, gRPC/protobuf existing, Unix sockets, SQLite `modernc.org/sqlite`, YAML, standard library crypto/hash/process APIs.

**Spec:** `docs/superpowers/specs/2026-09-13-oikos-fase5-extensibility-portability-design.md`

## Global Constraints

- Module `github.com/oikos/oikos`; Go minimum 1.23.
- Plugin transport gRPC Unix socket lokal dengan permission 0600.
- Plugin tidak mendapat akses langsung ke SQLite, Docker socket, filesystem host, private key, atau event bus internal.
- Plugin route wajib berada di `/plugins/<id>/`.
- Update hanya manual melalui `oikos update`.
- SHA-256 dan signature wajib diverifikasi sebelum switch.
- Config, database, dan plugin manifest dibackup sebelum update.
- Container existing tidak boleh direstart oleh update.
- `GOOS=linux`, `GOOS=darwin`, dan `GOOS=windows` wajib compile.
- Secret tidak boleh muncul di log, diagnostics, metadata plugin, atau manifest output.

---

## File Map

- Create `proto/plugin/plugin.proto`, `gen/go/plugin/`, `migrations/0005_plugins.sql`.
- Modify `scripts/gen-proto.sh`, `internal/store/models.go`, `internal/store/sqlite.go`.
- Create `internal/plugin/api.go`, `manifest.go`, `host.go`, `registry.go`, `proxy.go` and tests.
- Create `internal/update/release.go`, `slots.go`, `manager.go` and tests.
- Create `internal/platform/` interfaces/build-tag implementations.
- Modify `internal/resource/` build tags and unsupported fallback.
- Create `cmd/oikos/update.go`, `doctor.go` and tests; modify `main.go`/daemon wiring.
- Create `docs/plugins.md`, `docs/update.md`, `docs/architecture.md`.

---

### Task 1: Plugin proto, migration, and store registry

**Files:**
- Create: `proto/plugin/plugin.proto`, `migrations/0005_plugins.sql`, `internal/store/plugin_test.go`
- Modify: `scripts/gen-proto.sh`, `internal/store/models.go`, `internal/store/sqlite.go`
- Generate: `gen/go/plugin/`

**Interfaces:** `pluginpb.PluginServiceClient/Server`; `store.Plugin{ID,Name,Version,BinaryPath string; Enabled bool; SubscribedEvents string; InstalledAt time.Time}`; `MigratePlugins`, `SavePlugin`, `GetPlugin`, `ListPlugins`, `SetPluginEnabled`.

- [ ] **Step 1: Write failing test**

```go
func TestPluginRegistryRoundtrip(t *testing.T) {
    db := openMigrated(t); if err := db.MigratePlugins(); err != nil { t.Fatal(err) }
    want := Plugin{ID:"log-to-file",Name:"Log",Version:"1.0.0",BinaryPath:"/plugins/log",Enabled:true,SubscribedEvents:`["server.crashed"]`,InstalledAt:time.Now().UTC().Truncate(time.Second)}
    if err := db.SavePlugin(want); err != nil { t.Fatal(err) }
    got, err := db.GetPlugin(want.ID); if err != nil || got.Version != want.Version { t.Fatalf("plugin=%+v err=%v",got,err) }
    if err := db.SetPluginEnabled(want.ID,false); err != nil { t.Fatal(err) }; got,_=db.GetPlugin(want.ID); if got.Enabled { t.Fatal("must disable") }
}
```

- [ ] **Step 2:** Run `go test ./internal/store/ -run TestPluginRegistryRoundtrip -v`; expect FAIL on missing API.
- [ ] **Step 3:** Write proto service methods `Init`, `HandleEvent`, `HealthCheck`, `Close`; create idempotent migration and CRUD with JSON validation for subscribed events.
- [ ] **Step 4:** Run `make proto && go test ./internal/store/ -v`; expect PASS.
- [ ] **Step 5:** Commit with `git add proto/plugin migrations/0005_plugins.sql scripts/gen-proto.sh gen/go/plugin internal/store && git commit -m "feat: plugin proto dan registry"`.

---

### Task 2: Manifest and capability model

**Files:** `internal/plugin/api.go`, `manifest.go`, `manifest_test.go`.

**Interfaces:** `Event`, `Capability`, `InitRequest`, `InitResponse`, `PluginClient`, `Manifest`, `LoadManifest(path string) (Manifest,error)`, `ValidateManifest`, `ValidateCapabilities`.

- [ ] **Step 1:** Test invalid ID `bad/id`, global route `/admin`, and capability event outside manifest; expect FAIL because package absent.
- [ ] **Step 2:** Implement YAML fields `id,name,version,binary,enabled,config,allowed_events,allowed_routes,sha256`; validate ID regex `[A-Za-z0-9_-]+`, 64-char SHA-256, nonempty executable, event names, and route prefix `/plugins/<id>/` without `..`.
- [ ] **Step 3:** Run `go test ./internal/plugin/ -run 'TestManifest|TestCapabilities' -v`; expect PASS.
- [ ] **Step 4:** Commit `git add internal/plugin && git commit -m "feat: plugin manifest capability"`.

---

### Task 3: Plugin host process and Unix socket

**Files:** `internal/plugin/host.go`, `fake_plugin_test.go`, `host_test.go`.

**Interfaces:** `HostOptions{SocketRoot,OikosVersion string; EventTimeout,HealthTimeout time.Duration}`; `NewHost(Manifest,HostOptions) (*Host,error)`; `Host.Start`, `Stop`, `HandleEvent`, `HealthCheck`, `Routes`, `Status`.

- [ ] **Step 1:** Test a fake plugin executable that performs Init, receives `server.crashed`, returns healthy, and exits; assert socket mode 0600, event delivery, and cleanup. Test intentional exit does not kill test host.
- [ ] **Step 2:** Implement temporary Unix socket per plugin, `grpc.NewServer`/client handshake, manifest SHA validation, capability intersection, context timeout for event/health calls, and process wait isolation. Never pass host DB/socket paths in `InitRequest`.
- [ ] **Step 3:** Test a plugin requesting undeclared event/route; host must reject it. Test plugin crash marks status unhealthy and host remains usable.
- [ ] **Step 4:** Run `go test ./internal/plugin/ -v`; expect PASS.
- [ ] **Step 5:** Commit `git add internal/plugin && git commit -m "feat: plugin host unix socket"`.

---

### Task 4: Plugin registry lifecycle, event bridge, and route proxy

**Files:** `internal/plugin/registry.go`, `proxy.go`, `registry_test.go`, `proxy_test.go`; modify `internal/orchestrator/eventbus.go` and `internal/api/server.go` only where registration is required.

**Interfaces:** `Registry.LoadEnabled`, `StartAll`, `StopAll`, `DispatchEvent`, `HealthCheckAll`; `Proxy.Register(pluginID, route string, handler http.Handler) error`; `Proxy.Handler() http.Handler`.

- [ ] **Step 1:** Test enabled plugin receives only allowed event and disabled plugin receives none; test route `/plugins/p/status` works while `/admin` is rejected.
- [ ] **Step 2:** Implement registry from SQLite, bridge bus subscription with cancellation, per-plugin timeout, crash disable after 3 failures, and namespace route proxy.
- [ ] **Step 3:** Run `go test ./internal/plugin/ ./internal/orchestrator/ ./internal/api/ -v`; expect PASS.
- [ ] **Step 4:** Commit `git add internal/plugin internal/orchestrator internal/api && git commit -m "feat: plugin event bridge dan route proxy"`.

---

### Task 5: Versioned update slots and release verification

**Files:** `internal/update/release.go`, `slots.go`, `manager.go`, `update_test.go`.

**Interfaces:** `Release{Version,BinaryURL,SHA256 string; Signature []byte; GOOS,GOARCH string}`; `UpdateManager{Check,Apply,Rollback}`; `NewManager(SlotConfig, ReleaseVerifier, HealthRunner) UpdateManager`.

- [ ] **Step 1:** Test checksum mismatch leaves `current` unchanged, valid candidate switches atomically, and rollback restores previous. Use temp slots only.
- [ ] **Step 2:** Implement download to temp, lowercase exact SHA-256 comparison, signature verifier interface, executable permission, slot rename, `current`/`previous` symlink swap via temp symlink+rename, and backup callback for config/database/manifest.
- [ ] **Step 3:** Test health-runner failure before switch and after switch; both must preserve/restore known-good slot. Never delete previous automatically.
- [ ] **Step 4:** Run `go test ./internal/update/ -v`; expect PASS.
- [ ] **Step 5:** Commit `git add internal/update && git commit -m "feat: versioned update slots dan rollback"`.

---

### Task 6: `oikos update` command and daemon handoff

**Files:** `cmd/oikos/update.go`, `update_test.go`; modify `cmd/oikos/main.go`, `internal/agent/daemon.go`.

**Interfaces:** `runUpdate(args []string) error`; flags `--version`, `--url`, `--sha256`, `--signature`, `--versions-dir`, `--config`; `--health-check-only` returns 0 only after config/SQLite/Docker client Ping checks pass. `config.plugins.manifest_path`, when set, is backed up with config and SQLite.

- [ ] **Step 1:** Test update command with fake release and health runner; assert slot switch and config/database backup. Test failed health check keeps old symlink.
- [ ] **Step 2:** Implement manual-only command, no auto-update goroutine, signal/drain hook, and candidate health-check subprocess. Handoff must not call runtime Stop/Restart/Delete.
- [ ] **Review fixes:** Docker readiness uses client Ping rather than fabricated-container Stats; daemon wires cancellation handoff; candidate argv is tested exactly; config, SQLite, and plugin manifest backups are content-verified.
- [ ] **Step 3:** Run `go test ./cmd/oikos/ ./internal/update/ -v`; expect PASS.
- [ ] **Step 4:** Commit `git add cmd/oikos internal/agent && git commit -m "feat: oikos update graceful handoff"`.

---

### Task 7: Portability boundary and cross-build

**Files:** `internal/platform/platform.go`, `platform_unix.go`, `platform_windows.go`, `internal/resource/limiter.go`, `cgroups_linux.go`, `limiter_unsupported.go`, `docs/architecture.md`.

**Interfaces:** `platform.SocketPath`, `platform.ProcessGroup`, `resource.LimitEnforcer`; unsupported methods return `ErrNotSupported`.

- [ ] **Step 1:** Add compile checks via commands, not runtime tests: `GOOS=linux go test ./...`, `GOOS=darwin go test ./...`, `GOOS=windows go test ./...`.
- [ ] **Step 2:** Move Unix/cgroup/syscall imports behind build tags; provide non-Linux compile-safe implementations returning `ErrNotSupported`.
- [ ] **Step 3:** Run all three commands and `go vet ./...`; expect compile success.
- [ ] **Step 4:** Document Linux production target and unsupported feature behavior; commit `git add internal/platform internal/resource docs/architecture.md && git commit -m "feat: portability build boundaries"`.

---

### Task 8: `oikos doctor`

**Files:** `cmd/oikos/doctor.go`, `doctor_test.go`; modify `cmd/oikos/main.go`.

**Interfaces:** `runDoctor(args []string) error`; checks config, SQLite, Docker, cgroup, mTLS files, filesystem roots, plugin manifests, and update slots.

- [ ] **Step 1:** Test fake checker matrix: all healthy exits 0; required Docker/SQLite failure exits nonzero; optional plugin/frp failure prints warning and remains actionable.
- [ ] **Step 2:** Implement deterministic lines `[ok]`, `[warn]`, `[FAIL]`, remediation text, and no secret values. `doctor --config` accepts config path and never mutates runtime state.
- [ ] **Step 3:** Run `go test ./cmd/oikos/ -run TestDoctor -v`; expect PASS.
- [ ] **Step 4:** Commit `git add cmd/oikos && git commit -m "feat: oikos doctor diagnostics"`.

---

### Task 9: Example plugin, docs, and integration verification

**Files:** `cmd/example-plugin/`, `docs/plugins.md`, `docs/update.md`, `docs/architecture.md`, `internal/plugin/integration_test.go`, `internal/update/integration_test.go`.

- [ ] **Step 1:** Add example log-to-file plugin subscribing to `server.crashed`, with manifest and no direct core imports.
- [ ] **Step 2:** Test plugin process crash isolation and event reaction. Test update with Docker container ID/inspect timestamp unchanged.
- [ ] **Step 3:** Document install/manifest/capabilities, update backup/rollback, slots, platform support, and doctor commands.
- [ ] **Step 4:** Run `go test -tags integration ./internal/plugin/ ./internal/update/ -v`; tests skip only when explicit environment prerequisites are absent.
- [ ] **Step 5:** Commit `git add cmd/example-plugin docs internal/plugin internal/update && git commit -m "test: plugin dan update integration"`.

---

### Task 10: Final Fase 5 verification

**Files:** no new implementation files.

- [ ] **Step 1:** Run `make proto && make build && go test -count=1 ./... && go vet ./... && test -z "$(gofmt -l cmd internal gen)"`.
- [ ] **Step 2:** Run `GOOS=linux go test ./...`, `GOOS=darwin go test ./...`, and `GOOS=windows go test ./...`.
- [ ] **Step 3:** Run plugin/update integration tests and record PASS or explicit SKIP reasons; verify no container restart.
- [ ] **Step 4:** Review Fase 5 DoD line by line and commit `git add -A && git commit -m "chore: verifikasi fase 5 extensibility portability"`.

## Self-Review

- Spec coverage: plugin contract/registry/host/proxy (Tasks 1-4), graceful update/rollback (Tasks 5-6), portability (7), doctor (8), examples/docs/integration (9-10).
- All task interfaces are explicit and later tasks consume the names defined earlier.
- No task grants plugins direct access to host internals or allows update to stop containers.
- Cross-build validation is explicit and distinguishes compile portability from production platform support.
