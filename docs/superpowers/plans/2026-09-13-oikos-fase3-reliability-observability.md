# Oikos Fase 3 Reliability and Observability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Menambahkan structured logging, Prometheus metrics, health checks, event history, dan self-healing bounded untuk container serta tunnel.

**Architecture:** Observability dipisah dari domain: logger hanya menerima event/fields, metrics membaca snapshot cache, health memakai probe yang diinjeksi, dan event recorder menjadi subscriber bus yang menulis SQLite. Self-healer bekerja sebagai controller terpisah yang menerima crash event, mengevaluasi policy, lalu memanggil `Lifecycle.RestartServer`; tidak ada retry tanpa batas.

**Tech Stack:** Go 1.26, `zerolog`, `prometheus/client_golang`, `net/http`, SQLite `modernc.org/sqlite`, gRPC/protobuf existing.

**Spec:** `docs/superpowers/specs/2026-09-13-oikos-fase3-reliability-observability-design.md`

## Global Constraints

- Module Go: `github.com/oikos/oikos`.
- Go minimum: 1.23.
- SQLite driver: `modernc.org/sqlite` (tanpa cgo).
- Metrics default bind: `127.0.0.1:9191`.
- Metrics path: `/metrics`; health path: `/healthz`.
- Event retention default: 30 hari.
- Self-healing default: max 5 retry, base 5 detik, max 5 menit, stable reset 5 menit.
- Token pairing, private key, frp token, dan credential tidak boleh muncul di log atau metric label.
- Handler HTTP tidak boleh melakukan Docker API call langsung; collector/probe mengisi snapshot terpisah.
- Integration Docker boleh environment-gated; unit suite default harus berjalan tanpa Docker.

---

## File Map

- Create `internal/observability/logger.go`: structured logger dan redaction.
- Create `internal/observability/logger_test.go`: JSON fields dan secret redaction.
- Create `internal/observability/metrics.go`: metric registry, snapshot, collector.
- Create `internal/observability/metrics_test.go`: exposition dan runtime error behavior.
- Create `internal/observability/health.go`: probe aggregation dan HTTP handler.
- Create `internal/observability/health_test.go`: status matrix dan HTTP code.
- Create `internal/observability/http.go`: HTTP server lifecycle `/metrics` + `/healthz`.
- Create `migrations/0003_observability.sql`: events dan server restart columns.
- Modify `internal/store/models.go`, `sqlite.go`: event/restart CRUD dan migration.
- Create `internal/store/events_test.go`: event query/prune tests.
- Modify `internal/orchestrator/eventbus.go`: cancelable subscriptions dan recorder hook.
- Create `internal/orchestrator/restart_policy.go`: policy math.
- Create `internal/orchestrator/selfheal.go`: container self-healer.
- Create `internal/orchestrator/selfheal_test.go`: bounded retry and stable reset.
- Modify `internal/runtime/runtime.go`, Docker runtime: exit/restart observation contract.
- Create `proto/observability/observability.proto`: optional Panel adapter.
- Modify `scripts/gen-proto.sh`: generate observability code.
- Modify `internal/agent/daemon.go`, `cmd/oikos/daemon.go`: start observability/self-healer schedulers.
- Create `docs/observability.md`: configuration and operations.

---

### Task 1: Observability dependencies and configuration

**Files:**
- Modify: `internal/config/config.go`, `internal/config/defaults.go`, `config.yaml`, `go.mod`, `go.sum`
- Create: `internal/config/observability_test.go`

**Interfaces:**
- Consumes: existing `Config` loader/default merge behavior.
- Produces: `config.ObservabilityConfig{BindAddr, MetricsPath, HealthPath string; EventRetentionDays int; MetricsInterval, HealthInterval time.Duration}`.

- [ ] **Step 1: Write the failing test**

```go
func TestObservabilityDefaults(t *testing.T) {
    cfg := Default()
    if cfg.Observability.BindAddr != "127.0.0.1:9191" { t.Fatal(cfg.Observability.BindAddr) }
    if cfg.Observability.EventRetentionDays != 30 { t.Fatal(cfg.Observability.EventRetentionDays) }
    if cfg.Observability.MetricsInterval != 15*time.Second { t.Fatal(cfg.Observability.MetricsInterval) }
}

func TestObservabilityPartialYamlPreservesDefaults(t *testing.T) {
    path := filepath.Join(t.TempDir(), "config.yaml")
    os.WriteFile(path, []byte("observability:\n  bind_addr: 127.0.0.1:9292\n"), 0600)
    cfg, err := Load(path); if err != nil { t.Fatal(err) }
    if cfg.Observability.BindAddr != "127.0.0.1:9292" || cfg.Observability.MetricsPath != "/metrics" { t.Fatalf("cfg=%+v", cfg.Observability) }
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/config/ -run TestObservability -v`
Expected: FAIL karena `Observability` belum ada.

- [ ] **Step 3: Implement config and dependencies**

Tambahkan struct dengan YAML duration parsing melalui `time.Duration` custom field atau loader helper yang menerima string `15s`. Default interval `15s`, paths persis spec. Tambahkan `github.com/rs/zerolog` dan `github.com/prometheus/client_golang` sebagai direct dependencies. Update `config.yaml` dengan blok observability.

- [ ] **Step 4: Run test to verify pass**

Run: `go test ./internal/config/ -run TestObservability -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config config.yaml go.mod go.sum
git commit -m "feat: konfigurasi observability"
```

---

### Task 2: Structured logger

**Files:**
- Create: `internal/observability/logger.go`, `internal/observability/logger_test.go`

**Interfaces:**
- Consumes: no domain dependencies.
- Produces: `NewLogger(component string, out io.Writer) *Logger`; methods `Info`, `Warn`, `Error`; `WithServer(serverID string) *Logger`; `RedactSecret(value string) string`.

- [ ] **Step 1: Write failing tests**

```go
func TestLoggerWritesRequiredJsonFields(t *testing.T) {
    var buf bytes.Buffer
    NewLogger("test.component", &buf).Info("hello")
    var got map[string]any
    if err := json.Unmarshal(buf.Bytes(), &got); err != nil { t.Fatal(err) }
    for _, key := range []string{"time", "level", "component", "message"} { if got[key] == nil { t.Fatalf("missing %s: %v", key, got) } }
    if got["component"] != "test.component" || got["message"] != "hello" { t.Fatalf("log=%v", got) }
}

func TestLoggerDoesNotWriteSecret(t *testing.T) {
    var buf bytes.Buffer
    NewLogger("security", &buf).Info("pairing token rejected", "token", "super-secret")
    if strings.Contains(buf.String(), "super-secret") { t.Fatal("secret leaked") }
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/observability/ -run TestLogger -v`
Expected: FAIL karena package belum ada.

- [ ] **Step 3: Implement logger**

Gunakan `zerolog.New(out).With().Timestamp().Str("component", component).Logger()`. Key bernama `token`, `secret`, `password`, `private_key`, `key`, `credential` harus dihilangkan atau diganti `"[REDACTED]"`; jangan pernah menulis value asli. `WithServer` menambah field `server_id` setelah validasi nonempty.

- [ ] **Step 4: Run test to verify pass**

Run: `go test ./internal/observability/ -run TestLogger -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/observability/logger.go internal/observability/logger_test.go
git commit -m "feat: structured logger dengan redaction"
```

---

### Task 3: Event history SQLite

**Files:**
- Create: `migrations/0003_observability.sql`, `internal/store/events_test.go`
- Modify: `internal/store/sqlite.go`, `internal/store/models.go`

**Interfaces:**
- Consumes: migrations `0001` dan `0002`.
- Produces: `store.Event{ID, Type, ServerID, Severity, Message, Metadata string; CreatedAt time.Time}`; `MigrateObservability() error`; `SaveEvent(Event) error`; `ListEvents(EventFilter) ([]Event,error)`; `PruneEvents(context.Context,time.Time) error`; `IncrementRestartCount`; `ResetRestartCount`.

- [ ] **Step 1: Write failing tests**

```go
func TestEventHistoryQueryAndPrune(t *testing.T) {
    db := openMigrated(t); if err := db.MigrateNetworking(); err != nil { t.Fatal(err) }; if err := db.MigrateObservability(); err != nil { t.Fatal(err) }
    old := time.Now().Add(-48*time.Hour); recent := time.Now().Add(-time.Hour)
    for _, ev := range []Event{{ID:"old",Type:"server.crashed",ServerID:"s1",Severity:"error",Message:"old",Metadata:`{"x":1}`,CreatedAt:old},{ID:"new",Type:"server.started",ServerID:"s1",Severity:"info",Message:"new",Metadata:`{}`,CreatedAt:recent}} { if err := db.SaveEvent(ev); err != nil { t.Fatal(err) } }
    got, err := db.ListEvents(EventFilter{ServerID:"s1", Type:"server.started"}); if err != nil { t.Fatal(err) }; if len(got) != 1 || got[0].ID != "new" { t.Fatalf("events=%+v", got) }
    if err := db.PruneEvents(context.Background(), time.Now().Add(-24*time.Hour)); err != nil { t.Fatal(err) }
    got, err = db.ListEvents(EventFilter{ServerID:"s1"}); if err != nil { t.Fatal(err) }; if len(got) != 1 || got[0].ID != "new" { t.Fatalf("after prune=%+v", got) }
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/store/ -run TestEventHistoryQueryAndPrune -v`
Expected: FAIL karena migration/model/API belum ada.

- [ ] **Step 3: Implement migration and store**

Migration memakai `CREATE TABLE IF NOT EXISTS`, indexes, dan `ALTER TABLE` yang aman untuk database baru. `SaveEvent` menolak metadata invalid JSON dan severity/type/message kosong. `ListEvents` mendukung filter optional type/server/from/to, order `created_at DESC`, dan limit default 1000. `PruneEvents` memakai context-aware ExecContext. Restart methods update `servers.restart_count` dan `last_crash_at`.

- [ ] **Step 4: Run test to verify pass**

Run: `go test ./internal/store/ -v`
Expected: PASS seluruh store tests.

- [ ] **Step 5: Commit**

```bash
git add migrations/0003_observability.sql internal/store
git commit -m "feat: event history dan restart state"
```

---

### Task 4: Event bus cancellation dan recorder

**Files:**
- Modify: `internal/orchestrator/eventbus.go`
- Create: `internal/orchestrator/eventbus_test.go`, `internal/orchestrator/eventrecorder.go`

**Interfaces:**
- Consumes: `store.SaveEvent`.
- Produces: `Subscribe(typ string) (events <-chan Event, cancel func())`; `Publish(Event)` remains compatible; `NewEventRecorder(bus *EventBus, db EventWriter, logger *observability.Logger) *EventRecorder`; `EventRecorder.Start(context.Context)`, `Stop()`.

- [ ] **Step 1: Write failing tests**

```go
func TestSubscriptionCancelRemovesSubscriber(t *testing.T) {
    bus := NewEventBus(); ch, cancel := bus.Subscribe("x"); cancel(); bus.Publish(Event{Type:"x"})
    select { case <-ch: t.Fatal("cancelled subscriber menerima event"); default: }
}

func TestRecorderPersistsBeforeBroadcast(t *testing.T) {
    db := newEventStoreSpy(); bus := NewEventBus(); rec := NewEventRecorder(bus, db, nil); ctx, cancel := context.WithCancel(context.Background()); defer cancel(); go rec.Start(ctx)
    ch, _ := bus.Subscribe("server.crashed"); bus.Publish(Event{Type:"server.crashed", ServerID:"s1"})
    <-ch; if len(db.events) != 1 { t.Fatalf("persisted=%d", len(db.events)) }
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/orchestrator/ -run 'TestSubscriptionCancel|TestRecorder' -v`
Expected: FAIL karena Subscribe belum mengembalikan cancel dan recorder belum ada.

- [ ] **Step 3: Implement cancellation and recorder**

Perubahan signature `Subscribe` harus memperbarui semua call sites di agent/tunnel/self-healer. Publish menyalin subscriber list di bawah lock lalu mengirim tanpa memegang lock; gunakan channel buffer 64 dan nonblocking drop dengan warning agar slow subscriber tidak deadlock daemon. Recorder menerima event penting, menulis ke store sebelum meneruskan broadcast melalui method `PublishRecorded` atau subscription internal yang tidak menggandakan event.

- [ ] **Step 4: Run tests and commit**

Run: `go test ./internal/orchestrator/ ./...`
Expected: PASS tanpa deadlock.

```bash
git add internal/orchestrator internal/agent internal/tunnel
git commit -m "feat: event bus cancellation dan history recorder"
```

---

### Task 5: Prometheus metrics snapshot and HTTP handler

**Files:**
- Create: `internal/observability/metrics.go`, `internal/observability/metrics_test.go`, `internal/observability/http.go`

**Interfaces:**
- Consumes: `store.Server`, `store.Tunnel`, runtime stats provider.
- Produces: `SnapshotProvider`; `NewMetrics(reg prometheus.Registerer, provider SnapshotProvider) *Metrics`; `(*Metrics).Refresh(context.Context) error`; `NewHTTPServer(cfg config.ObservabilityConfig, metrics http.Handler, health http.Handler) *http.Server`.

- [ ] **Step 1: Write failing tests**

```go
func TestMetricsExposeServerAndDaemon(t *testing.T) {
    reg := prometheus.NewRegistry(); m := NewMetrics(reg, fakeSnapshotProvider{servers: []ServerMetric{{ID:"s1",Status:"running",CPU:12.5,MemoryBytes:1024}}})
    if err := m.Refresh(context.Background()); err != nil { t.Fatal(err) }
    req := httptest.NewRequest("GET", "/metrics", nil); rec := httptest.NewRecorder(); m.Handler().ServeHTTP(rec, req)
    body := rec.Body.String(); for _, needle := range []string{"oikos_servers_total", `status="running"`, "oikos_server_cpu_usage_percent", "oikos_daemon_uptime_seconds"} { if !strings.Contains(body, needle) { t.Fatalf("missing %s: %s", needle, body) } }
}

func TestMetricsRefreshErrorKeepsHttpAlive(t *testing.T) {
    reg := prometheus.NewRegistry(); m := NewMetrics(reg, failingSnapshotProvider{})
    if err := m.Refresh(context.Background()); err == nil { t.Fatal("refresh harus melaporkan error") }
    rec := httptest.NewRecorder(); m.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil)); if rec.Code != http.StatusOK { t.Fatal(rec.Code) }
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/observability/ -run TestMetrics -v`
Expected: FAIL karena metrics package belum ada.

- [ ] **Step 3: Implement metrics**

Gunakan custom collectors pada registry milik Oikos agar test tidak memakai global registry. GaugeVec/counter sesuai nama spec. `Refresh` mengganti snapshot atomik dan mengembalikan error gabungan/logged; handler memakai `promhttp.HandlerFor(reg, ...)`. Uptime berasal dari time source saat constructor, bukan counter yang bisa reset tiap refresh.

- [ ] **Step 4: Verify and commit**

Run: `go test ./internal/observability/ -run TestMetrics -v`
Expected: PASS.

```bash
git add internal/observability
git commit -m "feat: prometheus metrics snapshot"
```

---

### Task 6: Health probes and `/healthz`

**Files:**
- Create: `internal/observability/health.go`, `internal/observability/health_test.go`

**Interfaces:**
- Consumes: injected `Probe func(context.Context) error` for SQLite, Docker, Panel.
- Produces: `HealthStatus`, `Check`, `HealthReport`, `NewHealthChecker(probes map[string]Probe)`, `(*HealthChecker).Check(context.Context) HealthReport`, `HealthHandler(*HealthChecker) http.Handler`.

- [ ] **Step 1: Write failing tests**

```go
func TestHealthStatusMatrix(t *testing.T) {
    cases := []struct{name string; probes map[string]Probe; want HealthStatus; code int}{
        {"all", map[string]Probe{"sqlite":okProbe,"docker":okProbe,"panel":okProbe}, Healthy, 200},
        {"panel", map[string]Probe{"sqlite":okProbe,"docker":okProbe,"panel":failProbe}, Degraded, 200},
        {"docker", map[string]Probe{"sqlite":okProbe,"docker":failProbe,"panel":okProbe}, Unhealthy, 503},
    }
    for _, tc := range cases { h := NewHealthChecker(tc.probes); report := h.Check(context.Background()); if report.Status != tc.want { t.Fatalf("%s=%s", tc.name, report.Status) }; rec := httptest.NewRecorder(); HealthHandler(h).ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil)); if rec.Code != tc.code { t.Fatalf("%s code=%d", tc.name, rec.Code) } }
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/observability/ -run TestHealthStatusMatrix -v`
Expected: FAIL karena types/handler belum ada.

- [ ] **Step 3: Implement health aggregation**

SQLite failure is `unhealthy`; Docker failure is `unhealthy`; Panel-only failure is `degraded`; multiple failures are `unhealthy`. Every report includes all probe names, error text safe for operators, and UTC timestamp. Encode JSON content type and no sensitive probe details.

- [ ] **Step 4: Verify and commit**

Run: `go test ./internal/observability/ -run TestHealth -v`
Expected: PASS.

```bash
git add internal/observability
git commit -m "feat: health check endpoint"
```

---

### Task 7: Restart policy and self-healing controller

**Files:**
- Create: `internal/orchestrator/restart_policy.go`, `internal/orchestrator/selfheal.go`, `internal/orchestrator/selfheal_test.go`
- Modify: `internal/store/sqlite.go`, `internal/orchestrator/lifecycle.go`

**Interfaces:**
- Consumes: event bus, `Lifecycle.RestartServer`, store restart state.
- Produces: `RestartPolicy`; `DefaultRestartPolicy`; `(*RestartPolicy).NextBackoff(attempt int) time.Duration`; `NewSelfHealer(bus, lifecycle, store, policy, clock) *SelfHealer`; `Start(context.Context)`; `MarkManualStop(serverID)`.

- [ ] **Step 1: Write failing tests**

```go
func TestRestartPolicyCapsBackoff(t *testing.T) {
    p := DefaultRestartPolicy
    if p.NextBackoff(0) != 5*time.Second || p.NextBackoff(1) != 10*time.Second || p.NextBackoff(99) != 5*time.Minute { t.Fatal("backoff salah") }
}

func TestSelfHealerStopsAtMaxRetries(t *testing.T) {
    bus := NewEventBus(); fake := newLifecycleSpy(); healer := NewSelfHealer(bus, fake, newRestartStore(), RestartPolicy{MaxRetries:2, BackoffBase:0, BackoffMax:0, StableResetAfter:time.Hour}, immediateClock{})
    ctx, cancel := context.WithCancel(context.Background()); defer cancel(); go healer.Start(ctx)
    bus.Publish(Event{Type:"server.crashed", ServerID:"s1"}); bus.Publish(Event{Type:"server.crashed", ServerID:"s1"}); bus.Publish(Event{Type:"server.crashed", ServerID:"s1"})
    eventually(t, func() bool { return fake.restarts == 2 && fake.crashLooping })
}

func TestManualStopDoesNotRestart(t *testing.T) {
    bus := NewEventBus(); fake := newLifecycleSpy(); healer := NewSelfHealer(bus, fake, newRestartStore(), RestartPolicy{MaxRetries:2, BackoffBase:0}, immediateClock{})
    ctx, cancel := context.WithCancel(context.Background()); defer cancel(); go healer.Start(ctx); healer.MarkManualStop("s1"); bus.Publish(Event{Type:"server.crashed", ServerID:"s1"}); time.Sleep(20*time.Millisecond); if fake.restarts != 0 { t.Fatal(fake.restarts) }
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/orchestrator/ -run 'TestRestartPolicy|TestSelfHealer|TestManualStop' -v`
Expected: FAIL karena policy/healer belum ada.

- [ ] **Step 3: Implement policy and healer**

Use injectable clock interface with `Now()` and `After(context.Context,duration) <-chan time.Time`; production clock wraps timers, tests use immediate clock. On crash, atomically read restart count, skip if manually stopped, mark crash metadata, publish recovery-failed after max, otherwise wait and restart. Increment only after success; publish `server.recovered`. A successful stable observation calls `ResetRestartCount`. Do not use `time.AfterFunc` with a context that may already be canceled.

- [ ] **Step 4: Add crash observation hook**

Extend runtime with an optional `ExitObserver` rather than changing every existing Runtime implementation abruptly. Docker observer polls `ContainerInspect` at a configured interval and publishes `server.crashed` only for unexpected nonzero exits; lifecycle manual stop records a marker. Fake runtime exposes a test method to emit exit.

- [ ] **Step 5: Verify and commit**

Run: `go test ./internal/orchestrator/ ./internal/runtime/... -v`
Expected: PASS.

```bash
git add internal/orchestrator internal/runtime internal/store
git commit -m "feat: bounded container self healing"
```

---

### Task 8: Tunnel self-healing policy adapter

**Files:**
- Modify: `internal/tunnel/frp.go`, `internal/tunnel/manager.go`
- Create: `internal/tunnel/selfheal.go`, `internal/tunnel/selfheal_test.go`

**Interfaces:**
- Consumes: `RestartPolicy.NextBackoff`, `FrpManager.Watch`, event bus.
- Produces: `NewTunnelSelfHealer(manager Manager, policy orchestrator.RestartPolicy, logger *observability.Logger)`, bounded reconnect with the same caps as container policy.

- [ ] **Step 1: Write failing test**

```go
func TestTunnelSelfHealerUsesBoundedPolicy(t *testing.T) {
    manager := newFailingTunnelManager(2); healer := NewTunnelSelfHealer(manager, RestartPolicy{MaxRetries:2, BackoffBase:0, BackoffMax:0}, nil)
    ctx, cancel := context.WithCancel(context.Background()); defer cancel(); go healer.Start(ctx)
    manager.emitExit(errors.New("frpc died")); eventually(t, func() bool { return manager.reloads == 2 && manager.failed })
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/tunnel/ -run TestTunnelSelfHealer -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

Make tunnel manager expose a bounded `Watch` event channel or keep watcher internal and inject policy. Stop all timers on context cancel; mark disconnected on failure; stop retrying after max and log a final error. Do not duplicate policy math.

- [ ] **Step 4: Verify and commit**

Run: `go test ./internal/tunnel/ -v`
Expected: PASS.

```bash
git add internal/tunnel
git commit -m "feat: bounded tunnel self healing"
```

---

### Task 9: HTTP observability server and daemon scheduling

**Files:**
- Modify: `internal/agent/daemon.go`, `cmd/oikos/daemon.go`
- Create: `internal/observability/scheduler.go`, `internal/observability/http_test.go`

**Interfaces:**
- Consumes: metrics/health handlers, `Config.Observability`, event recorder, self-healer.
- Produces: `RunSchedulers(context.Context, Config, Metrics, EventRecorder, SelfHealer) error`; HTTP server starts on configured localhost address and shuts down with context.

- [ ] **Step 1: Write failing HTTP integration test**

```go
func TestHTTPServerServesMetricsAndHealth(t *testing.T) {
    cfg := config.Default().Observability; cfg.BindAddr = "127.0.0.1:0"
    srv := NewHTTPServer(cfg, staticMetricsHandler(), staticHealthHandler())
    lis, err := net.Listen("tcp", cfg.BindAddr); if err != nil { t.Fatal(err) }
    go srv.Serve(lis); defer srv.Shutdown(context.Background())
    base := "http://" + lis.Addr().String(); res, err := http.Get(base + "/metrics"); if err != nil { t.Fatal(err) }; res.Body.Close(); if res.StatusCode != 200 { t.Fatal(res.StatusCode) }
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/observability/ -run TestHTTPServer -v`
Expected: FAIL karena server lifecycle belum ada.

- [ ] **Step 3: Implement scheduling and daemon wiring**

Start HTTP server, metrics refresh ticker, health cache refresh ticker, event recorder, daily prune ticker, and self-healers under one derived context. On shutdown call `Shutdown` with 5-second timeout and close all subscriptions. Existing Panel heartbeat/command stream remains independent; observability failure must be logged and must not terminate the agent connection.

- [ ] **Step 4: Verify and commit**

Run: `go test ./... && go vet ./...`
Expected: PASS.

```bash
git add internal/agent cmd/oikos internal/observability
git commit -m "feat: daemon observability scheduling"
```

---

### Task 10: Observability proto adapter

**Files:**
- Create: `proto/observability/observability.proto`
- Modify: `scripts/gen-proto.sh`
- Generate: `gen/go/observability/`
- Create: `internal/api/observability.go`, `internal/api/observability_test.go`

**Interfaces:**
- Consumes: metrics snapshot, health checker, store event query.
- Produces: `ObservabilityService` RPCs `StreamEvents`, `GetServerMetrics`, `GetHealthStatus`; adapter is optional and does not replace HTTP endpoints.

- [ ] **Step 1: Write proto and failing adapter test**

Test must call `GetHealthStatus` through bufconn and assert status/check names; test `GetServerMetrics` asserts server ID and CPU value.

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/api/ -run TestObservability -v`
Expected: FAIL karena proto/service belum ada.

- [ ] **Step 3: Implement and generate**

Use protobuf messages for `HealthStatus`, `HealthCheck`, `ServerMetric`, `EventRecord`, request filters, and server-streaming `StreamEvents`. Register adapter on the local gRPC server and ensure no secrets appear in metadata.

- [ ] **Step 4: Verify and commit**

Run: `make proto && go test ./internal/api/ ./...`
Expected: PASS.

```bash
git add proto/observability scripts/gen-proto.sh gen/go/observability internal/api
git commit -m "feat: proto observability adapter"
```

---

### Task 11: Structured log migration Fase 1-2

**Files:**
- Modify: `internal/agent/*.go`, `internal/orchestrator/*.go`, `internal/tunnel/*.go`, `internal/runtime/docker/*.go`, `cmd/oikos/*.go`
- Create: `internal/observability/migration_test.go`

- [ ] **Step 1: Add static check test**

The test walks operational Go files and rejects `fmt.Print`, `fmt.Println`, `log.Print`, and `log.Println` except CLI user-facing output and test files. Allowed `fmt.Errorf` remains valid.

- [ ] **Step 2: Run check to identify violations**

Run: `go test ./internal/observability/ -run TestOperationalLoggingUsesStructuredLogger -v`
Expected: FAIL listing exact files/lines that still log unstructured.

- [ ] **Step 3: Migrate operational logs**

Inject component logger into agent/orchestrator/tunnel/runtime constructors; replace operational prints with `Info/Warn/Error`, attach `server_id`/`tunnel_id`, and redact secrets. Keep CLI command result output on stdout because it is user-facing, not an operational log.

- [ ] **Step 4: Verify and commit**

Run: `go test ./... && go vet ./...`
Expected: PASS and no static logging violations.

```bash
git add internal cmd
git commit -m "refactor: migrasi operational logs ke structured logger"
```

---

### Task 12: Docker crash integration and documentation

**Files:**
- Create: `internal/orchestrator/selfheal_integration_test.go` with `//go:build integration`
- Create: `docs/observability.md`

- [ ] **Step 1: Add environment-gated Docker test**

Start a real short-lived/crashing container through Docker runtime, register its server state, run observer, assert `server.crashed`, wait bounded restart, assert restart count and running state, then kill repeatedly until `crash_looping`. Skip only when Docker unavailable and print the exact reason.

- [ ] **Step 2: Run integration test**

Run: `go test -tags integration ./internal/orchestrator/ -run TestSelfHealDockerCrash -v -timeout 10m`
Expected: PASS on Docker host; explicit SKIP otherwise.

- [ ] **Step 3: Document operations**

`docs/observability.md` must include config, endpoint bind/security, metric names, health status meaning, event retention/prune behavior, restart policy, manual stop semantics, and commands for querying logs/events. State clearly that HTTP metrics/health are localhost by default.

- [ ] **Step 4: Commit**

```bash
git add internal/orchestrator docs/observability.md
git commit -m "docs: observability dan crash recovery docker"
```

---

### Task 13: Final Fase 3 verification

**Files:** no new implementation files; update status documentation only if needed.

- [ ] **Step 1: Run full verification**

```bash
make proto && make build && go test -count=1 ./... && go vet ./... && test -z "$(gofmt -l cmd internal gen)"
```

Expected: all unit/component tests pass, vet clean, no formatting output.

- [ ] **Step 2: Run integration verification**

```bash
```

Expected: Docker crash/recovery and tunnel tests PASS when dependencies exist; otherwise explicit SKIP reasons. No external alerting/dashboard claims.

- [ ] **Step 3: Verify DoD line by line**

Record evidence for: bounded restart, max retry termination, manual stop exclusion, valid Prometheus exposition, health matrix, structured log migration, event query/prune, tunnel policy reuse, and localhost security defaults.

- [ ] **Step 4: Commit final status**

```bash
git commit -m "chore: verifikasi fase 3 reliability observability"
```

---

## Self-Review

- Spec coverage: logger/metrics/health (Tasks 2, 5, 6), event history (3, 4), container self-healing (7, 12), tunnel policy reuse (8), HTTP/daemon wiring (9), proto adapter (10), structured migration (11), and final DoD (13) are mapped.
- Existing interface risk is isolated: `EventBus.Subscribe` changes are handled in Task 4 and all consumers are explicitly listed; runtime crash observation is optional rather than breaking every runtime implementation.
- No dependency calls are hidden in HTTP handlers; metrics and health are refreshed by schedulers and served from cached state.
- Event recorder ordering is explicit: persistence precedes broadcast, while cancellation and slow subscribers cannot deadlock the daemon.
- Manual stop, max retries, and context cancellation are explicit state boundaries, preventing false-positive crash recovery and goroutine leaks.
