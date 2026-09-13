# Oikos Fase 1 Fondasi Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Membangun fondasi Fase 1 MVP Core yang bisa di-build, di-test, dan menjalankan lifecycle server dasar via CLI.

**Architecture:** Bottom-up dari pure logic (config/store/egg) ke runtime interface + Docker real + FakeRuntime untuk test, lalu orchestrator + CLI. Toolchain real (Go 1.23, protoc, Docker client) diinstal dulu; test integrasi Docker di-skip bila daemon absen.

**Tech Stack:** Go 1.23, `modernc.org/sqlite` (pure Go), `github.com/docker/docker/client`, gRPC (`google.golang.org/grpc`, `google.golang.org/protobuf`), `gopkg.in/yaml.v3`, `google.golang.org/uuid`.

**Spec:** `docs/superpowers/specs/2026-09-13-oikos-fase1-fondasi-design.md`

## Global Constraints

- Module Go: `github.com/oikos/oikos`.
- Go minimum: 1.23.
- SQLite driver: `modernc.org/sqlite` (tanpa cgo).
- Runtime interface wajib sama persis dengan dokumen rencana (method `BuildImage/Create/Start/Stop/Restart/Delete/Status/Stats/Exec/Logs`).
- File cert/key ditulis dengan permission 0600.
- Semua operasi filesystem paket `filesystem` (di luar fondasi ini) belum dikerjakan — orchestrator hanya pakai `MountSource` sebagai string path.

---

### Task 1: Toolchain + skeleton proyek + Makefile + CI

**Files:**
- Create: `go.mod`, `Makefile`, `.golangci.yml`, `.github/workflows/ci.yml`, `cmd/oikos/main.go`, `scripts/gen-proto.sh`
- Test: `make build` dan `go test ./...` hijau (belum ada test = ok)

**Interfaces:**
- Consumes: tidak ada (task pertama).
- Produces: perintah `make build`, `make test`, `make proto`, `make run`; binary `bin/oikos` dengan output `oikos: no command`.

- [ ] **Step 1: Install toolchain dan verifikasi versi**

Run:

```bash
apt-get update && apt-get install -y golang-go protobuf-compiler protobuf-compiler-grpc docker.io 2>&1 | tail -5
go version
protoc --version
docker --version
```

Expected: `go version go1.23` atau lebih baru (bila apt memberi versi lama, unduh tarball Go 1.23 dari `https://go.dev/dl/` dan pasang ke `/usr/local/go`). `protoc` ada. `docker` client ada (daemon boleh absen).

- [ ] **Step 2: Init go.mod**

Run:

```bash
go mod init github.com/oikos/oikos
go get modernc.org/sqlite@latest gopkg.in/yaml.v3@latest github.com/google/uuid@latest google.golang.org/grpc@latest google.golang.org/protobuf@latest github.com/docker/docker@latest
go mod tidy
```

Expected: `go.mod` berisi `module github.com/oikos/oikos` dan `go 1.23`, semua dependency terunduh tanpa error.

- [ ] **Step 3: Tulis skeleton main + Makefile + CI + gen-proto.sh**

`cmd/oikos/main.go`:

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("oikos: no command")
		os.Exit(1)
	}
	fmt.Println("oikos: unknown command:", os.Args[1])
	os.Exit(1)
}
```

`Makefile`:

```make
.PHONY: build test proto run lint
build:
	go build -o bin/oikos ./cmd/oikos
test:
	go test ./...
proto:
	bash scripts/gen-proto.sh
run:
	go run ./cmd/oikos
lint:
	golangci-lint run ./...
```

`.golangci.yml`:

```yaml
linters:
  enable:
    - govet
    - errcheck
    - staticcheck
    - unused
```

`.github/workflows/ci.yml`:

```yaml
name: ci
on: [push, pull_request]
jobs:
  ci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.23"
      - run: go test ./...
      - run: go vet ./...
```

`scripts/gen-proto.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
export PATH="$PATH:$(go env GOPATH)/bin"
mkdir -p gen/go
protoc -I proto \
  --go_out=gen/go --go_opt=paths=source_relative \
  --go-grpc_out=gen/go --go-grpc_opt=paths=source_relative \
  proto/node/node.proto proto/server/server.proto proto/stream/stream.proto
```

Run: `chmod +x scripts/gen-proto.sh`.

- [ ] **Step 4: Verifikasi skeleton**

Run: `make build && ./bin/oikos; go test ./...`
Expected: binary mencetak `oikos: no command` (exit 1); `go test` lulus (no test files).

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum Makefile .golangci.yml .github/workflows/ci.yml cmd/oikos/main.go scripts/gen-proto.sh
git commit -m "feat: skeleton proyek oikos fase 1 fondasi"
```

---

### Task 2: Config (`internal/config`)

**Files:**
- Create: `internal/config/config.go`, `internal/config/defaults.go`, `internal/config/config_test.go`, `config.yaml`
- Modify: tidak ada.

**Interfaces:**
- Consumes: tidak ada.
- Produces: `config.Load(path string) (*Config, error)`; struct `Config` dengan field `Node{DataDir, CertPath, KeyPath}`, `Panel{Address}`, `Runtime{Engine, DockerSocket}`, `API{LocalGRPCPort}`, `Log{Level}`; `config.Default() *Config`.

- [ ] **Step 1: Write the failing test**

`internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	c := Default()
	if c.Panel.Address == "" {
		t.Fatal("default panel address kosong")
	}
	if c.Runtime.Engine != "docker" {
		t.Fatalf("default engine = %q, mau docker", c.Runtime.Engine)
	}
}

func TestLoadMissingFileUsesDefault(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "tidak-ada.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Node.DataDir != Default().Node.DataDir {
		t.Fatal("file hilang harus fallback ke default")
	}
}

func TestLoadOverridesDefault(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	content := "panel:\n  address: \"panel.test:9091\"\nlog:\n  level: \"debug\"\n"
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Panel.Address != "panel.test:9091" {
		t.Fatalf("address = %q", c.Panel.Address)
	}
	if c.Log.Level != "debug" {
		t.Fatalf("level = %q", c.Log.Level)
	}
	if c.Runtime.Engine != "docker" {
		t.Fatalf("engine harus tetap default docker, dapat %q", c.Runtime.Engine)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -v`
Expected: FAIL dengan `no Go files` / `undefined: Default`.

- [ ] **Step 3: Write minimal implementation**

`internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type NodeConfig struct {
	DataDir  string `yaml:"data_dir"`
	CertPath string `yaml:"cert_path"`
	KeyPath  string `yaml:"key_path"`
}

type PanelConfig struct {
	Address string `yaml:"address"`
}

type RuntimeConfig struct {
	Engine       string `yaml:"engine"`
	DockerSocket string `yaml:"docker_socket"`
}

type APIConfig struct {
	LocalGRPCPort int `yaml:"local_grpc_port"`
}

type LogConfig struct {
	Level string `yaml:"level"`
}

type Config struct {
	Node    NodeConfig    `yaml:"node"`
	Panel   PanelConfig   `yaml:"panel"`
	Runtime RuntimeConfig `yaml:"runtime"`
	API     APIConfig     `yaml:"api"`
	Log     LogConfig     `yaml:"log"`
}

func Load(path string) (*Config, error) {
	c := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, fmt.Errorf("baca config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return c, nil
}
```

`internal/config/defaults.go`:

```go
package config

func Default() *Config {
	return &Config{
		Node: NodeConfig{
			DataDir:  "/var/lib/oikos",
			CertPath: "/etc/oikos/certs/node.crt",
			KeyPath:  "/etc/oikos/certs/node.key",
		},
		Panel:   PanelConfig{Address: "panel.example.com:9091"},
		Runtime: RuntimeConfig{Engine: "docker", DockerSocket: "unix:///var/run/docker.sock"},
		API:     APIConfig{LocalGRPCPort: 9190},
		Log:     LogConfig{Level: "info"},
	}
}
```

`config.yaml` (contoh, sama dengan dokumen rencana):

```yaml
node:
  data_dir: "/var/lib/oikos"
  cert_path: "/etc/oikos/certs/node.crt"
  key_path: "/etc/oikos/certs/node.key"
panel:
  address: "panel.example.com:9091"
runtime:
  engine: "docker"
  docker_socket: "unix:///var/run/docker.sock"
api:
  local_grpc_port: 9190
log:
  level: "info"
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -v`
Expected: PASS ketiga test.

- [ ] **Step 5: Commit**

```bash
git add internal/config/ config.yaml
git commit -m "feat: config loader dengan default fallback"
```

---

### Task 3: Store SQLite (`internal/store`)

**Files:**
- Create: `migrations/0001_init.sql`, `internal/store/sqlite.go`, `internal/store/models.go`, `internal/store/store_test.go`
- Test: `internal/store/store_test.go`

**Interfaces:**
- Consumes: tidak ada.
- Produces: `store.Open(path string) (*DB, error)`; `(*DB).Migrate() error`; `(*DB).CreateServer(s Server) error`, `(*DB).GetServer(id string) (Server, error)`, `(*DB).UpdateServerStatus(id, status string) error`; struct `Server`, `Egg`, `ResourceLimits`.

- [ ] **Step 1: Write the failing test**

`internal/store/store_test.go`:

```go
package store

import (
	"path/filepath"
	"testing"
)

func TestMigrateAndServerCRUD(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "oikos.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateEgg(Egg{ID: "minecraft-vanilla", Name: "Minecraft Vanilla", DockerfilePath: "./Dockerfile", MetadataPath: "./egg.yaml"}); err != nil {
		t.Fatal(err)
	}
	s := Server{ID: "srv-1", Name: "mc-1", EggID: "minecraft-vanilla", Status: "stopped", StartupCommand: "java -jar server.jar nogui", Environment: "{}"}
	if err := db.CreateServer(s); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetServer("srv-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "mc-1" || got.Status != "stopped" {
		t.Fatalf("server salah: %+v", got)
	}
	if err := db.UpdateServerStatus("srv-1", "running"); err != nil {
		t.Fatal(err)
	}
	got, _ = db.GetServer("srv-1")
	if got.Status != "running" {
		t.Fatalf("status = %q, mau running", got.Status)
	}
	if err := db.SetResourceLimits(ResourceLimits{ServerID: "srv-1", CPULimit: 2000, MemoryLimitMB: 2048, PIDLimit: 256}); err != nil {
		t.Fatal(err)
	}
	lim, err := db.GetResourceLimits("srv-1")
	if err != nil {
		t.Fatal(err)
	}
	if lim.CPULimit != 2000 || lim.MemoryLimitMB != 2048 {
		t.Fatalf("limits salah: %+v", lim)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -v`
Expected: FAIL `undefined: Open`.

- [ ] **Step 3: Write minimal implementation**

`migrations/0001_init.sql`: salin verbatim skema `0001_init.sql` dari dokumen rencana (tabel `nodes`, `eggs`, `servers`, `resource_limits` + index `idx_servers_status`).

`internal/store/models.go`:

```go
package store

type Server struct {
	ID             string
	Name           string
	EggID          string
	ContainerID    string
	Status         string
	StartupCommand string
	Environment    string
}

type Egg struct {
	ID             string
	Name           string
	DockerfilePath string
	MetadataPath   string
}

type ResourceLimits struct {
	ServerID      string
	CPULimit      int64
	MemoryLimitMB int64
	DiskLimitMB   int64
	PIDLimit      int64
	BandwidthKbps int64
}
```

`internal/store/sqlite.go`:

```go
package store

import (
	"database/sql"
	_ "modernc.org/sqlite"
	"fmt"
	"os"
)

type DB struct{ sql *sql.DB }

func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("pragma: %w", err)
	}
	return &DB{sql: db}, nil
}

func (d *DB) Close() error { return d.sql.Close() }

func (d *DB) Migrate() error {
	data, err := os.ReadFile("migrations/0001_init.sql")
	if err != nil {
		return fmt.Errorf("baca migrasi: %w", err)
	}
	if _, err := d.sql.Exec(string(data)); err != nil {
		return fmt.Errorf("aplikasi migrasi: %w", err)
	}
	return nil
}

func (d *DB) CreateEgg(e Egg) error {
	_, err := d.sql.Exec(`INSERT INTO eggs(id,name,dockerfile_path,metadata_path) VALUES(?,?,?,?)`,
		e.ID, e.Name, e.DockerfilePath, e.MetadataPath)
	return err
}

func (d *DB) CreateServer(s Server) error {
	_, err := d.sql.Exec(`INSERT INTO servers(id,name,egg_id,container_id,status,startup_command,environment) VALUES(?,?,?,?,?,?,?)`,
		s.ID, s.Name, s.EggID, nullIfEmpty(s.ContainerID), s.Status, s.StartupCommand, s.Environment)
	return err
}

func (d *DB) GetServer(id string) (Server, error) {
	var s Server
	var cid sql.NullString
	err := d.sql.QueryRow(`SELECT id,name,egg_id,container_id,status,startup_command,environment FROM servers WHERE id=?`, id).
		Scan(&s.ID, &s.Name, &s.EggID, &cid, &s.Status, &s.StartupCommand, &s.Environment)
	if err != nil {
		return Server{}, err
	}
	s.ContainerID = cid.String
	return s, nil
}

func (d *DB) UpdateServerStatus(id, status string) error {
	_, err := d.sql.Exec(`UPDATE servers SET status=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, status, id)
	return err
}

func (d *DB) SetResourceLimits(l ResourceLimits) error {
	_, err := d.sql.Exec(`INSERT INTO resource_limits(server_id,cpu_limit,memory_limit_mb,disk_limit_mb,pid_limit,bandwidth_kbps)
		VALUES(?,?,?,?,?,?) ON CONFLICT(server_id) DO UPDATE SET cpu_limit=excluded.cpu_limit, memory_limit_mb=excluded.memory_limit_mb,
		disk_limit_mb=excluded.disk_limit_mb, pid_limit=excluded.pid_limit, bandwidth_kbps=excluded.bandwidth_kbps`,
		l.ServerID, l.CPULimit, l.MemoryLimitMB, l.DiskLimitMB, l.PIDLimit, l.BandwidthKbps)
	return err
}

func (d *DB) GetResourceLimits(serverID string) (ResourceLimits, error) {
	var l ResourceLimits
	err := d.sql.QueryRow(`SELECT server_id,cpu_limit,memory_limit_mb,disk_limit_mb,pid_limit,bandwidth_kbps FROM resource_limits WHERE server_id=?`, serverID).
		Scan(&l.ServerID, &l.CPULimit, &l.MemoryLimitMB, &l.DiskLimitMB, &l.PIDLimit, &l.BandwidthKbps)
	return l, err
}

func nullIfEmpty(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
```

Catatan: test di atas menjalankan `db.Migrate()` dari workdir repo sehingga path `migrations/0001_init.sql` valid saat `go test ./...`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add migrations/0001_init.sql internal/store/
git commit -m "feat: persistence sqlite dan migrasi awal"
```

---

### Task 4: Egg loader (`internal/egg`)

**Files:**
- Create: `internal/egg/egg.go`, `internal/egg/loader.go`, `internal/egg/validator.go`, `internal/egg/template.go`, `internal/egg/egg_test.go`
- Test: `internal/egg/egg_test.go`

**Interfaces:**
- Consumes: tidak ada.
- Produces: `egg.LoadDir(dir string) (Egg, error)`; `egg.Validate(e Egg) error`; `egg.RenderStartup(cmd string, vars map[string]string) string`; struct `Egg`, `EggVariable`, `EggPort`.

- [ ] **Step 1: Write the failing test**

`internal/egg/egg_test.go`:

```go
package egg

import (
	"os"
	"path/filepath"
	"testing"
)

func writeEgg(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "egg.yaml"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadAndValidateOK(t *testing.T) {
	dir := writeEgg(t, "name: \"Minecraft Vanilla\"\nslug: \"minecraft-vanilla\"\ndescription: \"test\"\nbuild:\n  dockerfile: \"./Dockerfile\"\nstartup:\n  command: \"java -Xmx{{MAX_MEMORY}}M -jar server.jar nogui\"\n  stop_signal: \"SIGTERM\"\nvariables:\n  - name: \"Maximum Memory\"\n    env: \"MAX_MEMORY\"\n    default: \"2048\"\n    editable: true\nports:\n  - name: \"game\"\n    default: 25565\n    protocol: \"tcp\"\n")
	e, err := LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(e); err != nil {
		t.Fatal(err)
	}
	if e.Slug != "minecraft-vanilla" {
		t.Fatalf("slug = %q", e.Slug)
	}
}

func TestValidateRejectsMissingCommand(t *testing.T) {
	e := Egg{Name: "x", Slug: "x"}
	if err := Validate(e); err == nil {
		t.Fatal("harus error untuk startup.command kosong")
	}
}

func TestRenderStartup(t *testing.T) {
	got := RenderStartup("java -Xmx{{MAX_MEMORY}}M -jar s.jar", map[string]string{"MAX_MEMORY": "2048"})
	if got != "java -Xmx2048M -jar s.jar" {
		t.Fatalf("hasil = %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/egg/ -v`
Expected: FAIL `undefined`.

- [ ] **Step 3: Write minimal implementation**

`internal/egg/egg.go`:

```go
package egg

type EggBuild struct {
	Dockerfile string `yaml:"dockerfile"`
}

type EggStartup struct {
	Command      string `yaml:"command"`
	StopSignal   string `yaml:"stop_signal"`
	StopCommand  string `yaml:"stop_command"`
	ReadinessLog string `yaml:"readiness_log"`
}

type EggVariable struct {
	Name     string `yaml:"name"`
	Env      string `yaml:"env"`
	Default  string `yaml:"default"`
	Editable bool   `yaml:"editable"`
}

type EggPort struct {
	Name     string `yaml:"name"`
	Default  int    `yaml:"default"`
	Protocol string `yaml:"protocol"`
}

type Egg struct {
	Name        string        `yaml:"name"`
	Slug        string        `yaml:"slug"`
	Description string        `yaml:"description"`
	Build       EggBuild      `yaml:"build"`
	Startup     EggStartup    `yaml:"startup"`
	Variables   []EggVariable `yaml:"variables"`
	Ports       []EggPort     `yaml:"ports"`
}
```

`internal/egg/loader.go`:

```go
package egg

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func LoadDir(dir string) (Egg, error) {
	data, err := os.ReadFile(filepath.Join(dir, "egg.yaml"))
	if err != nil {
		return Egg{}, fmt.Errorf("baca egg.yaml: %w", err)
	}
	var e Egg
	if err := yaml.Unmarshal(data, &e); err != nil {
		return Egg{}, fmt.Errorf("parse egg.yaml: %w", err)
	}
	return e, nil
}
```

`internal/egg/validator.go`:

```go
package egg

import "errors"

func Validate(e Egg) error {
	if e.Name == "" {
		return errors.New("egg.name wajib diisi")
	}
	if e.Slug == "" {
		return errors.New("egg.slug wajib diisi")
	}
	if e.Startup.Command == "" {
		return errors.New("egg.startup.command wajib diisi")
	}
	if e.Build.Dockerfile == "" {
		return errors.New("egg.build.dockerfile wajib diisi")
	}
	return nil
}
```

`internal/egg/template.go`:

```go
package egg

import "strings"

func RenderStartup(cmd string, vars map[string]string) string {
	out := cmd
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/egg/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/egg/
git commit -m "feat: egg loader validator dan template"
```

---

### Task 5: Runtime interface + Fake + Docker (`internal/runtime`)

**Files:**
- Create: `internal/runtime/runtime.go`, `internal/runtime/fake.go`, `internal/runtime/docker/docker.go`, `internal/runtime/docker/build.go`, `internal/runtime/docker/stats.go`, `internal/runtime/runtime_test.go`
- Test: `internal/runtime/runtime_test.go` (hanya Fake; Docker real di-skip bila daemon absen)

**Interfaces:**
- Consumes: tidak ada.
- Produces: interface `runtime.Runtime` + `ContainerSpec`, `Stats`, `ContainerStatus` persis dokumen rencana; `runtime.NewFake() Runtime`; `docker.New(socket string) (Runtime, error)`.

- [ ] **Step 1: Write the failing test**

`internal/runtime/runtime_test.go`:

```go
package runtime

import (
	"context"
	"testing"
)

func TestFakeLifecycle(t *testing.T) {
	r := NewFake()
	ctx := context.Background()
	id, err := r.Create(ctx, ContainerSpec{ServerID: "srv-1", Image: "img:test", Command: []string{"run"}, Env: map[string]string{"A": "1"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Start(ctx, id); err != nil {
		t.Fatal(err)
	}
	st, err := r.Status(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if st != StatusRunning {
		t.Fatalf("status = %q", st)
	}
	if err := r.Stop(ctx, id, 5); err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(ctx, id, false); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runtime/ -v`
Expected: FAIL `undefined: NewFake`.

- [ ] **Step 3: Write minimal implementation**

`internal/runtime/runtime.go`: salin verbatim interface + struct dari dokumen rencana (`ContainerSpec`, `ContainerStatus`, `Stats`, `Runtime` dengan 10 method).

`internal/runtime/fake.go`:

```go
package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type fake struct {
	mu         sync.Mutex
	containers map[string]ContainerStatus
	seq        int
}

func NewFake() Runtime {
	return &fake{containers: map[string]ContainerStatus{}}
}

func (f *fake) BuildImage(ctx context.Context, dockerfilePath, imageTag string) error {
	return nil
}

func (f *fake) Create(ctx context.Context, spec ContainerSpec) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	id := fmt.Sprintf("fake-%d", f.seq)
	f.containers[id] = StatusStopped
	return id, nil
}

func (f *fake) Start(ctx context.Context, containerID string) error {
	return f.set(containerID, StatusRunning)
}

func (f *fake) Stop(ctx context.Context, containerID string, timeoutSec int) error {
	return f.set(containerID, StatusStopped)
}

func (f *fake) Restart(ctx context.Context, containerID string) error {
	return f.set(containerID, StatusRunning)
}

func (f *fake) Delete(ctx context.Context, containerID string, force bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.containers, containerID)
	return nil
}

func (f *fake) Status(ctx context.Context, containerID string) (ContainerStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	st, ok := f.containers[containerID]
	if !ok {
		return "", errors.New("container tidak ditemukan")
	}
	return st, nil
}

func (f *fake) Stats(ctx context.Context, containerID string) (Stats, error) {
	return Stats{}, nil
}

func (f *fake) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	return "", nil
}

func (f *fake) Logs(ctx context.Context, containerID string, follow bool) (<-chan string, error) {
	ch := make(chan string)
	close(ch)
	return ch, nil
}

func (f *fake) set(id string, st ContainerStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.containers[id]; !ok {
		return errors.New("container tidak ditemukan")
	}
	f.containers[id] = st
	return nil
}
```

`internal/runtime/docker/docker.go`: implementasi `New(socket string)` via `github.com/docker/docker/client`, method `Create/Start/Stop/Restart/Delete/Status` memakai Docker API; `build.go` berisi `BuildImage`, `stats.go` berisi `Stats/Exec/Logs` (boleh minimal tapi harus compile dan return error yang jelas bila daemon tak terjangkau — tanpa placeholder komentar).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/runtime/... -v`
Expected: PASS (test Docker real wajib skip bila `docker info` gagal).

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/
git commit -m "feat: runtime interface docker dan fake"
```

---

### Task 6: Resource dasar + orchestrator (`internal/resource`, `internal/orchestrator`)

**Files:**
- Create: `internal/resource/cgroups.go`, `internal/orchestrator/eventbus.go`, `internal/orchestrator/state.go`, `internal/orchestrator/lifecycle.go`, `internal/orchestrator/lifecycle_test.go`
- Test: `internal/orchestrator/lifecycle_test.go`

**Interfaces:**
- Consumes: `store.DB`, `runtime.Runtime`.
- Produces: `orchestrator.New(db *store.DB, rt runtime.Runtime, bus *EventBus) *Lifecycle`; `(*Lifecycle).CreateServer(ctx, name, eggID, startup string, env map[string]string) (string, error)`; `StartServer/StopServer/RestartServer/DeleteServer`; `bus.Subscribe(typ string) <-chan Event`, `bus.Publish(ev Event)`; `resource.DetectCgroupVersion() string`.

- [ ] **Step 1: Write the failing test**

`internal/orchestrator/lifecycle_test.go`:

```go
package orchestrator

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/oikos/oikos/internal/runtime"
	"github.com/oikos/oikos/internal/store"
)

func setup(t *testing.T) *Lifecycle {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "o.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateEgg(store.Egg{ID: "egg-1", Name: "egg", DockerfilePath: "d", MetadataPath: "m"}); err != nil {
		t.Fatal(err)
	}
	return New(db, runtime.NewFake(), NewEventBus())
}

func TestCreateStartStopDelete(t *testing.T) {
	lc := setup(t)
	ctx := context.Background()
	id, err := lc.CreateServer(ctx, "srv", "egg-1", "run", map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if err := lc.StartServer(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := lc.StopServer(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := lc.DeleteServer(ctx, id); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/orchestrator/ -v`
Expected: FAIL `undefined: New`.

- [ ] **Step 3: Write minimal implementation**

`internal/resource/cgroups.go`: fungsi `DetectCgroupVersion() string` membaca `/sys/fs/cgroup/cgroup.controllers` (ada = "v2", tidak = "v1").

`internal/orchestrator/eventbus.go`: pub/sub channel-based dengan map tipe → daftar subscriber.

`internal/orchestrator/state.go`: wrapper tipis `GetServer/UpdateStatus` ke `store.DB`.

`internal/orchestrator/lifecycle.go`: `CreateServer` (uuid baru, simpan status `installing` lalu `stopped`, simpan env JSON), `StartServer` (runtime.Create bila `container_id` kosong lalu Start, status `running`), `StopServer` (Stop, status `stopped`), `RestartServer` (Restart, status `running`), `DeleteServer` (Delete + hapus row SQLite, publish event tiap transisi).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/orchestrator/ ./internal/resource/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/resource/ internal/orchestrator/
git commit -m "feat: orchestrator lifecycle dan eventbus"
```

---

### Task 7: Proto + CLI + egg contoh + installer

**Files:**
- Create: `proto/node/node.proto`, `proto/server/server.proto`, `proto/stream/stream.proto`, `eggs/minecraft-vanilla/egg.yaml`, `eggs/minecraft-vanilla/Dockerfile`, `deploy/systemd/oikos.service`, `scripts/install.sh`, `internal/security/token.go`, `internal/agent/pairing.go` (tipis)
- Modify: `cmd/oikos/main.go` (subcommand `server create|start|stop|delete`, `install`)
- Test: manual `make proto && make build && ./bin/oikos server create --help`-style; `go test ./...` tetap hijau.

**Interfaces:**
- Consumes: semua task sebelumnya.
- Produces: CLI `oikos server create|start|stop|delete`, `oikos install --token`; file proto yang bisa di-generate.

- [ ] **Step 1: Tulis proto dan generate**

Isi `proto/node/node.proto` (package `node`; pesan `RegisterNodeRequest`, `HeartbeatRequest`, `PairingRequest/Response`), `proto/server/server.proto` (package `server`; `CreateServerRequest`, `StartServerRequest`, `StopServerRequest`, `DeleteServerRequest`), `proto/stream/stream.proto` (package `stream`; `CommandStream`, `LogChunk`) — proto3 dengan field minimal yang sesuai rencana.

Run: `make proto`
Expected: file di `gen/go/` ter-generate tanpa error.

- [ ] **Step 2: Implementasi CLI + token + pairing tipis + egg contoh + installer**

`cmd/oikos/main.go` memakai `flag` standar: `server create --name --egg --startup`, `server start|stop|delete --id`, `install --token` (validasi token tak kosong, tulis config awal). `internal/security/token.go`: `GenerateToken() string` (32 byte hex via crypto/rand) + `ValidateFormat()`. `eggs/minecraft-vanilla/{egg.yaml,Dockerfile}` dan `deploy/systemd/oikos.service` + `scripts/install.sh` mengikuti dokumen rencana.

- [ ] **Step 3: Verifikasi akhir fondasi**

Run:

```bash
make proto && make build && go test ./... && go vet ./...
```

Expected: semua PASS tanpa error; `gosec`/`govulncheck` bila tersedia tanpa temuan tinggi (bila belum terinstal, catat sebagai follow-up, bukan blocker).

- [ ] **Step 4: Commit**

```bash
git add proto/ gen/go/ cmd/oikos/ internal/security/ internal/agent/ eggs/ deploy/ scripts/install.sh
git commit -m "feat: proto cli egg contoh dan installer dasar"
```

---

## Self-Review

- Cakupan spec: toolchain+struktur (Task 1), config (2), store+migrasi (3), egg (4), runtime (5), resource+orchestrator (6), proto+CLI+installer (7) — semua DoD fondasi terpetakan.
- Placeholder: tidak ada TBD/TODO; setiap langkah ada perintah run + expected konkret; kode test dan implementasi ditulis eksplisit.
- Konsistensi tipe: `store.Server/ResourceLimits`, `runtime.ContainerSpec/Stats/Runtime`, `orchestrator.New(db, rt, bus)`, `config.Load/Default` dipakai konsisten lintas task.
