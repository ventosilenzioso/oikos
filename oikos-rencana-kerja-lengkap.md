# Oikos — Rencana Kerja Lengkap (Fase 1–6)

> Dokumen ini merangkum seluruh rencana kerja Oikos dari MVP hingga siap production, mencakup goals, task breakdown, detail teknis, definition of done, dan risiko & mitigasi untuk setiap fase.

## Daftar Fase

| Fase | Nama | Tujuan yang Dicakup |
|---|---|---|
| 1 | MVP Core | #1, #4, #6, #8, #9, #10, #14, #18 |
| 2 | Networking | #3, #11 |
| 3 | Reliability & Observability | #5, #17 |
| 4 | Filesystem & Data | #7, #12 |
| 5 | Ekstensibilitas & Portabilitas | #13, #16, #15, #2 |
| 6 | Production Hardening | Konsolidasi & validasi akhir seluruh tujuan |

---

# Fase 1 — MVP Core

> Fondasi Oikos: daemon bisa dipasang, terhubung ke Panel, dan menjalankan lifecycle server dasar menggunakan Docker.

**Tujuan dari daftar awal yang dicakup:** #1 (ringan), #4 (runtime abstraction), #6 (resource dasar), #8 (lightweight), #9 (security-first), #10 (zero-config node), #14 (API-first), #18 (egg .dockerfile)

**Status:** Belum dimulai
**Dependency fase sebelumnya:** Tidak ada (fase awal)

---

## Daftar Isi

1. [Goals](#goals)
2. [Non-Goals](#non-goals)
3. [Task Breakdown](#task-breakdown)
4. [Detail Teknis](#detail-teknis)
5. [Definition of Done](#definition-of-done)
6. [Risiko & Mitigasi](#risiko--mitigasi)

---

## Goals

- Node bisa dipasang dengan satu perintah dan langsung terhubung ke Panel tanpa konfigurasi manual (zero-config).
- Komunikasi Node ↔ Panel aman sejak awal (token sekali pakai → mTLS).
- Server (container) bisa dibuat, dijalankan, dihentikan, dan dihapus lewat perintah dari Panel.
- Format egg baru (Dockerfile + `egg.yaml`) bisa dipakai untuk build image server.
- Semua fungsi di atas bisa dikontrol lewat API (gRPC) — tidak ada operasi yang hanya bisa lewat akses manual ke server.
- Resource dasar (CPU, memori, PID) bisa dibatasi per server melalui cgroups v2.
- Daemon punya jejak resource (RAM/CPU) yang kecil saat idle.

## Non-Goals

Hal-hal berikut **sengaja tidak** dikerjakan di fase ini, akan masuk fase berikutnya:
- Tunnel frp / NAT traversal (Fase 2).
- Self-healing otomatis untuk crash (Fase 3).
- Observability penuh — metrics/log terstruktur lengkap (Fase 3).
- File manager, SFTP, backup (Fase 4).
- Plugin system, graceful update, cross-platform selain Linux (Fase 5).
- Implementasi Podman — interface disiapkan generic, tapi hanya Docker yang diimplementasikan.

---

## Task Breakdown

### 1. Setup Proyek
- [ ] Inisialisasi `go.mod`, struktur folder sesuai `oikos-project-structure.md`.
- [ ] Setup `.golangci.yml` dan CI dasar (lint + test).
- [ ] Setup `Makefile` (`make build`, `make proto`, `make test`, `make run`).

### 2. Definisi gRPC (`proto/`)
- [ ] `proto/node/node.proto` — pesan `RegisterNodeRequest`, `HeartbeatRequest`, `PairingRequest/Response`.
- [ ] `proto/server/server.proto` — pesan `CreateServerRequest`, `StartServerRequest`, `StopServerRequest`, `DeleteServerRequest`.
- [ ] `proto/stream/stream.proto` — pesan `CommandStream` (server→client push command), `LogChunk`.
- [ ] `scripts/gen-proto.sh` untuk generate kode ke `gen/go/`.

### 3. Persistence Layer (`internal/store/`)
- [ ] Setup koneksi SQLite (`sqlite.go`) menggunakan driver `mattn/go-sqlite3` atau `modernc.org/sqlite` (cgo-free, lebih portable).
- [ ] Migrasi awal `0001_init.sql` (tabel `nodes`, `servers`, `eggs`, `resource_limits`).
- [ ] Struct model di `models.go`.

### 4. Konfigurasi (`internal/config/`)
- [ ] Struct `Config` (alamat panel, path data dir, port lokal, dsb).
- [ ] Loader dari `config.yaml` dengan default fallback (`defaults.go`).

### 5. Security (`internal/security/`)
- [ ] `token.go` — generate & validasi token pairing sekali pakai (di sisi Panel), verifikasi di sisi node.
- [ ] `mtls.go` — generate CSR di node, terima cert signed dari Panel, simpan ke disk dengan permission ketat (0600).

### 6. Agent (`internal/agent/`)
- [ ] `pairing.go` — alur: baca token dari input/CLI → kirim ke Panel → terima cert → simpan.
- [ ] `client.go` — bentuk koneksi gRPC dengan mTLS, retry/backoff saat reconnect.
- [ ] `heartbeat.go` — kirim heartbeat periodik (interval default 15 detik, exponential backoff jika gagal).
- [ ] `stream.go` — buka stream, terima command dari Panel, teruskan ke `orchestrator`.

### 7. Runtime Abstraction (`internal/runtime/`)
- [ ] `runtime.go` — definisikan interface `Runtime`.
- [ ] `docker/docker.go` — implementasi berbasis Docker SDK (`github.com/docker/docker/client`).
- [ ] `docker/build.go` — build image dari Dockerfile egg.
- [ ] `docker/stats.go` — ambil stats dasar (CPU%, mem usage) dari Docker API.

### 8. Egg Loader (`internal/egg/`)
- [ ] `egg.go` — struct `Egg` merepresentasikan `egg.yaml`.
- [ ] `loader.go` — baca folder egg lokal.
- [ ] `validator.go` — validasi field wajib.
- [ ] `template.go` — substitusi `{{VAR}}` di startup command.

### 9. Resource Dasar (`internal/resource/`)
- [ ] `cgroups.go` — buat cgroup v2 per server, set limit CPU (`cpu.max`), memori (`memory.max`), PID (`pids.max`).

### 10. Orchestrator (`internal/orchestrator/`)
- [ ] `lifecycle.go` — `CreateServer`, `StartServer`, `StopServer`, `RestartServer`, `DeleteServer`.
- [ ] `state.go` — simpan/ambil state server dari SQLite.
- [ ] `eventbus.go` — pub/sub sederhana (channel-based) untuk dipakai modul lain nanti.

### 11. API Lokal (`internal/api/`)
- [ ] gRPC server lokal (opsional untuk testing tanpa Panel) di `internal/api/grpc/server.go`.
- [ ] CLI dasar di `cmd/oikos/` (`oikos server create`, `oikos server start`, dst) untuk testing manual sebelum Panel siap.

### 12. Zero-Config Installer
- [ ] `scripts/install.sh` — deteksi OS, download binary, buat systemd unit, jalankan `oikos install`.
- [ ] `oikos install` — prompt token, jalankan pairing, tulis `config.yaml` awal.
- [ ] `deploy/systemd/oikos.service`.

---

## Detail Teknis

### Skema SQLite Awal

```sql
-- 0001_init.sql

CREATE TABLE nodes (
    id              TEXT PRIMARY KEY,       -- UUID node, di-generate saat pairing
    name            TEXT NOT NULL,
    panel_url       TEXT NOT NULL,
    cert_path       TEXT NOT NULL,
    key_path        TEXT NOT NULL,
    paired_at       DATETIME NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE eggs (
    id              TEXT PRIMARY KEY,       -- slug, mis. "minecraft-vanilla"
    name            TEXT NOT NULL,
    dockerfile_path TEXT NOT NULL,
    metadata_path   TEXT NOT NULL,          -- path ke egg.yaml
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE servers (
    id              TEXT PRIMARY KEY,       -- UUID server
    name            TEXT NOT NULL,
    egg_id          TEXT NOT NULL REFERENCES eggs(id),
    container_id    TEXT,                   -- ID container dari runtime, null jika belum dibuat
    status          TEXT NOT NULL DEFAULT 'installing',
                    -- installing | stopped | starting | running | stopping | crashed | deleting
    startup_command TEXT NOT NULL,
    environment     TEXT NOT NULL DEFAULT '{}',  -- JSON key-value
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE resource_limits (
    server_id       TEXT PRIMARY KEY REFERENCES servers(id) ON DELETE CASCADE,
    cpu_limit       INTEGER,                -- dalam milicores, mis. 2000 = 2 core
    memory_limit_mb INTEGER,
    disk_limit_mb   INTEGER,
    pid_limit       INTEGER,
    bandwidth_kbps  INTEGER                 -- disiapkan untuk fase lanjut
);

CREATE INDEX idx_servers_status ON servers(status);
```

### Interface Runtime

```go
// internal/runtime/runtime.go
package runtime

import "context"

type ContainerSpec struct {
    ServerID     string
    Image        string
    Command      []string
    Env          map[string]string
    CPULimit     int64  // milicores
    MemoryLimit  int64  // bytes
    PIDLimit     int64
    WorkingDir   string
    MountSource  string // path folder data server di host
}

type ContainerStatus string

const (
    StatusRunning ContainerStatus = "running"
    StatusStopped ContainerStatus = "stopped"
    StatusCrashed ContainerStatus = "crashed"
)

type Stats struct {
    CPUPercent    float64
    MemoryUsedMB  int64
    MemoryLimitMB int64
    NetRxBytes    int64
    NetTxBytes    int64
}

// Runtime adalah kontrak yang harus dipenuhi setiap implementasi container engine
// (Docker, Podman, dll). Semua operasi bersifat idempotent bila memungkinkan.
type Runtime interface {
    BuildImage(ctx context.Context, dockerfilePath, imageTag string) error
    Create(ctx context.Context, spec ContainerSpec) (containerID string, err error)
    Start(ctx context.Context, containerID string) error
    Stop(ctx context.Context, containerID string, timeoutSec int) error
    Restart(ctx context.Context, containerID string) error
    Delete(ctx context.Context, containerID string, force bool) error
    Status(ctx context.Context, containerID string) (ContainerStatus, error)
    Stats(ctx context.Context, containerID string) (Stats, error)
    Exec(ctx context.Context, containerID string, cmd []string) (output string, err error)
    Logs(ctx context.Context, containerID string, follow bool) (<-chan string, error)
}
```

### Format `egg.yaml`

```yaml
# eggs/minecraft-vanilla/egg.yaml
name: "Minecraft Vanilla"
slug: "minecraft-vanilla"
description: "Server Minecraft vanilla resmi dari Mojang"

build:
  dockerfile: "./Dockerfile"

startup:
  command: "java -Xms{{MIN_MEMORY}}M -Xmx{{MAX_MEMORY}}M -jar server.jar nogui"
  stop_signal: "SIGTERM"
  stop_command: "stop"
  readiness_log: "Done ("   # pattern log yang menandakan server siap

variables:
  - name: "Minimum Memory"
    env: "MIN_MEMORY"
    default: "1024"
    editable: true
  - name: "Maximum Memory"
    env: "MAX_MEMORY"
    default: "2048"
    editable: true

ports:
  - name: "game"
    default: 25565
    protocol: "tcp"
```

### Contoh Dockerfile Egg

```dockerfile
# eggs/minecraft-vanilla/Dockerfile
FROM eclipse-temurin:21-jre-jammy

RUN useradd -m -u 1000 oikos
WORKDIR /home/oikos/server
USER oikos

COPY --chown=oikos:oikos server.jar ./server.jar

# CMD tidak didefinisikan di sini — startup command diambil dari egg.yaml
# agar tetap fleksibel untuk variable substitution ({{MIN_MEMORY}}, dst)
```

### Alur Pairing Token → mTLS (Sequence)

```
Node                              Panel
 |--- oikos install -------------->|
 |    (input: pairing token)       |
 |                                  |
 |--- PairingRequest{token} ------>|
 |                                  |--- validasi token (belum expired, belum dipakai)
 |                                  |--- generate keypair + sign cert utk node
 |<--- PairingResponse{cert, key}--|
 |                                  |--- tandai token sebagai "used"
 |
 |--- simpan cert & key ke disk
 |    (permission 0600)
 |
 |--- buka gRPC stream baru dgn mTLS ------->|
 |<--- stream command siap ------------------|
```

### Contoh `config.yaml`

```yaml
# config.yaml
node:
  data_dir: "/var/lib/oikos"
  cert_path: "/etc/oikos/certs/node.crt"
  key_path: "/etc/oikos/certs/node.key"

panel:
  address: "panel.example.com:9091"

runtime:
  engine: "docker"          # future: "podman"
  docker_socket: "unix:///var/run/docker.sock"

api:
  local_grpc_port: 9190     # untuk debugging/CLI sebelum panel terhubung

log:
  level: "info"
```

---

## Definition of Done

- [ ] `curl -sSL install.oikos.io | sh` (simulasi lokal) berhasil memasang daemon di VM bersih.
- [ ] `oikos install --token=<token>` berhasil pairing dan menghasilkan node berstatus "online" di Panel (atau mock Panel untuk testing).
- [ ] Server dapat dibuat dari egg `minecraft-vanilla` melalui perintah gRPC, image ter-build otomatis dari Dockerfile.
- [ ] Server dapat di-start, stop, restart, delete lewat command dari Panel/CLI, status tersinkron di SQLite.
- [ ] Batas CPU/memori/PID pada `resource_limits` benar-benar diterapkan (diverifikasi dengan stress test container).
- [ ] Idle daemon menggunakan < 30MB RAM dan < 1% CPU pada mesin uji standar (angka acuan awal, disesuaikan setelah benchmark nyata).
- [ ] Semua komunikasi node↔panel setelah pairing menggunakan mTLS, token pairing tidak bisa dipakai dua kali.
- [ ] Unit test untuk `orchestrator/lifecycle.go` dan `egg/validator.go` lulus dengan coverage memadai.

---

## Risiko & Mitigasi

| Risiko | Dampak | Mitigasi |
|---|---|---|
| Docker SDK Go cukup berat (banyak dependency) | Bisa memperbesar binary/footprint | Evaluasi pakai Docker HTTP API langsung tanpa SDK penuh jika perlu |
| cgroups v2 belum tentu tersedia di semua distro/VPS | Resource limit gagal di-set | Deteksi versi cgroup saat startup, fallback ke cgroups v1 atau warning jelas |
| mTLS cert generation salah konfigurasi | Node gagal connect, sulit didiagnosis | Buat command `oikos diagnose` sejak awal untuk cek validitas cert & koneksi |
| Skema SQLite berubah drastis di fase berikutnya | Migrasi menyakitkan | Terapkan sistem migrasi bernomor (`0001_`, `0002_`, dst) sejak awal, jangan edit migrasi lama |


---

# Fase 2 — Networking (NAT Tunnel & Multi-Node)

> Membuat node di belakang NAT/CGNAT bisa diakses publik lewat frp, serta memungkinkan node saling berkomunikasi dalam private network.

**Tujuan dari daftar awal yang dicakup:** #3 (VPS NAT + auto tunnel frp), #11 (multi-node networking)

**Status:** Belum dimulai
**Dependency fase sebelumnya:** Fase 1 (agent, orchestrator, security/mTLS harus sudah berjalan)

---

## Daftar Isi

1. [Goals](#goals)
2. [Non-Goals](#non-goals)
3. [Task Breakdown](#task-breakdown)
4. [Detail Teknis](#detail-teknis)
5. [Definition of Done](#definition-of-done)
6. [Risiko & Mitigasi](#risiko--mitigasi)

---

## Goals

- Node di belakang NAT/CGNAT bisa membuka port game/service ke publik tanpa port forwarding manual, menggunakan frp.
- Tunnel dikonfigurasi otomatis oleh Oikos berdasarkan port yang didefinisikan egg — user/admin tidak perlu sentuh config frp secara manual.
- Beberapa node bisa membentuk private network agar bisa saling berkomunikasi (mis. untuk server yang butuh koneksi antar-node, seperti database terpisah atau proxy).
- Tunnel yang terputus terdeteksi dan dicoba disambungkan kembali otomatis (dasar untuk self-healing penuh di Fase 3).

## Non-Goals

- Self-healing policy lengkap dengan backoff kompleks dan alerting (masuk Fase 3, di sini baru reconnect dasar).
- Load balancing lintas node (di luar cakupan Oikos, ranah Panel/orkestrasi lebih tinggi).
- Enkripsi payload traffic game/service itu sendiri — tunnel hanya menyediakan jalur, bukan mengenkripsi ulang traffic aplikasi.

---

## Task Breakdown

### 1. Riset & Keputusan Desain frp
- [ ] Tentukan mode frp yang dipakai: `frps` di-hosting di mana (built-in ke Panel, atau server terpisah)?
- [ ] Tentukan skema autentikasi frpc↔frps (token statis per-node vs dynamic token dari Panel).
- [ ] Tentukan port range yang dialokasikan Panel untuk setiap node (agar tidak bentrok antar-server/node).

### 2. Tunnel Manager (`internal/tunnel/`)
- [ ] `manager.go` — start/stop/reload proses `frpc` sebagai child process per node (satu proses frpc menangani banyak port mapping).
- [ ] `config.go` — generate file `frpc.toml`/`frpc.ini` secara dinamis berdasarkan port yang dibutuhkan tiap server aktif.
- [ ] `selfheal.go` — watcher dasar: jika proses `frpc` mati/exit, restart dengan backoff sederhana (detail policy penuh di Fase 3).

### 3. Integrasi dengan Orchestrator
- [ ] Saat `CreateServer` dijalankan dan egg mendefinisikan port, orchestrator memanggil `tunnel.RegisterPort()`.
- [ ] Saat `DeleteServer`, panggil `tunnel.ReleasePort()` agar port dibebaskan dari config frpc.
- [ ] Emit event `tunnel.connected` / `tunnel.disconnected` ke event bus.

### 4. Multi-Node Private Network
- [ ] Tentukan pendekatan: overlay network sederhana (mis. WireGuard mesh) vs memanfaatkan frp untuk P2P antar-node.
- [ ] `internal/tunnel/mesh.go` (baru) — kelola koneksi antar-node dalam satu "network group" yang didefinisikan Panel.
- [ ] Mekanisme discovery: Panel memberi tahu node lain mana saja yang satu grup, node saling membuka tunnel P2P atau lewat relay.

### 5. API & Proto
- [ ] `proto/tunnel/tunnel.proto` — pesan `RegisterTunnelRequest`, `TunnelStatus`, `NetworkGroupAssignment`.
- [ ] Endpoint gRPC untuk Panel query status tunnel semua node.

### 6. Observability Dasar untuk Tunnel
- [ ] Log status koneksi tunnel (connected/disconnected/reconnecting) — detail penuh masuk Fase 3, tapi log dasar harus ada dari awal fase ini untuk debugging.

---

## Detail Teknis

### Skema Tambahan SQLite

```sql
-- 0002_networking.sql

CREATE TABLE tunnels (
    id              TEXT PRIMARY KEY,
    server_id       TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    local_port      INTEGER NOT NULL,
    remote_port     INTEGER,              -- di-assign oleh frps/Panel
    protocol        TEXT NOT NULL DEFAULT 'tcp',
    status          TEXT NOT NULL DEFAULT 'pending',
                    -- pending | connected | disconnected | error
    last_connected  DATETIME,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE network_groups (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE network_group_members (
    group_id        TEXT NOT NULL REFERENCES network_groups(id) ON DELETE CASCADE,
    node_id         TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    private_ip      TEXT NOT NULL,        -- IP dalam overlay network, mis. 10.88.0.x
    PRIMARY KEY (group_id, node_id)
);
```

### Interface Tunnel Manager

```go
// internal/tunnel/manager.go
package tunnel

import "context"

type PortMapping struct {
    ServerID   string
    LocalPort  int
    Protocol   string // "tcp" | "udp"
}

type TunnelStatus string

const (
    TunnelPending      TunnelStatus = "pending"
    TunnelConnected    TunnelStatus = "connected"
    TunnelDisconnected TunnelStatus = "disconnected"
    TunnelError        TunnelStatus = "error"
)

type Manager interface {
    RegisterPort(ctx context.Context, mapping PortMapping) (remotePort int, err error)
    ReleasePort(ctx context.Context, serverID string, localPort int) error
    Status(ctx context.Context, serverID string) ([]TunnelStatus, error)
    Reload(ctx context.Context) error // regenerate config & restart frpc dengan graceful reload
}
```

### Contoh Generated `frpc.toml`

```toml
# generated by internal/tunnel/config.go — JANGAN edit manual
serverAddr = "frps.oikos-panel.example.com"
serverPort = 7000
auth.method = "token"
auth.token = "{{NODE_FRP_TOKEN}}"   # diberikan Panel saat pairing/registrasi tunnel

[[proxies]]
name = "server-a1b2c3-game"
type = "tcp"
localIP = "127.0.0.1"
localPort = 25565
remotePort = 30001   # di-assign oleh Panel/frps, disimpan di tabel tunnels

[[proxies]]
name = "server-a1b2c3-query"
type = "udp"
localIP = "127.0.0.1"
localPort = 25566
remotePort = 30002
```

### Alur Registrasi Tunnel Saat Server Dibuat

```
orchestrator.CreateServer(spec)
   │
   ▼
egg.yaml punya definisi ports: [{name: game, default: 25565, protocol: tcp}]
   │
   ▼
tunnel.RegisterPort({ServerID, LocalPort: 25565, Protocol: tcp})
   │
   ▼
Manager kirim request ke Panel: "butuh remote port utk server X"
   │
   ▼
Panel assign remote_port (mis. 30001), simpan mapping
   │
   ▼
tunnel/config.go regenerate frpc.toml dengan proxy baru
   │
   ▼
tunnel/manager.go reload frpc (SIGHUP atau restart graceful)
   │
   ▼
Event "tunnel.connected" di-emit ke event bus
```

### Pertimbangan Desain: Kenapa frpc Sebagai Child Process, Bukan Library

Meskipun frp punya opsi dipakai sebagai Go library langsung (embed di binary), pendekatan **child process terpisah** dipilih karena:
- Reload konfigurasi tidak perlu merestart daemon utama Oikos.
- Crash pada frpc tidak menjatuhkan seluruh daemon Oikos (isolasi failure).
- Update versi frp bisa dilakukan independen dari update Oikos.

Trade-off: sedikit overhead proses tambahan, tapi ini sejalan dengan prinsip **pemisahan primitives vs policy** yang sudah dipakai di orchestrator.

### Multi-Node Private Network — Opsi Desain

| Opsi | Kelebihan | Kekurangan |
|---|---|---|
| WireGuard mesh | Cepat, kernel-level, matang | Butuh dependency WireGuard di setiap node |
| Overlay via frp (P2P/xtcp) | Reuse infrastruktur tunnel yang sudah ada | Latency lebih tinggi dibanding WireGuard native |
| Relay lewat Panel/frps saja | Paling sederhana diimplementasikan | Semua traffic antar-node lewat Panel = bottleneck & biaya bandwidth Panel |

> Rekomendasi awal: mulai dengan **relay lewat frps** untuk kesederhanaan MVP fase ini, dengan interface yang dirancang agar bisa diganti ke WireGuard mesh nanti tanpa mengubah kontrak `Manager` di atas.

---

## Definition of Done

- [ ] VPS dengan NAT (disimulasikan dengan Docker network terisolasi atau firewall block inbound) berhasil expose port game server ke publik lewat frp, tanpa konfigurasi manual dari user.
- [ ] Saat server dihapus, port tunnel otomatis dilepas dan tidak lagi bisa diakses dari luar.
- [ ] Tunnel yang mati (proses frpc di-kill paksa) otomatis reconnect dalam waktu wajar (< 30 detik) tanpa intervensi manual.
- [ ] Dua node dalam `network_group` yang sama bisa saling ping/berkomunikasi lewat private IP.
- [ ] Status tunnel semua node bisa di-query lewat gRPC API dari Panel.
- [ ] Reload config frpc tidak menyebabkan downtime pada tunnel yang sudah connected sebelumnya.

---

## Risiko & Mitigasi

| Risiko | Dampak | Mitigasi |
|---|---|---|
| frps jadi single point of failure untuk semua node | Semua tunnel putus bila frps down | Dokumentasikan opsi frps redundant/HA sebagai catatan operasional, bukan blocker MVP fase ini |
| Port conflict antar server dalam satu node | Server gagal start/tunnel gagal register | Validasi port availability sebelum assign, port lokal auto-increment bila bentrok |
| Overhead relay penuh lewat Panel untuk multi-node network | Bandwidth Panel membengkak | Desain interface `Manager` agar mudah diganti ke WireGuard mesh tanpa breaking change di orchestrator |
| Auth token frpc bocor | Orang lain bisa expose port lewat frps milik kita | Token per-node unik, rotasi berkala, batasi frps hanya terima registrasi dari node yang sudah ter-mTLS |


---

# Fase 3 — Reliability & Observability

> Membuat Oikos bisa dipercaya berjalan tanpa pengawasan terus-menerus: self-healing otomatis dan visibilitas penuh terhadap kondisi sistem.

**Tujuan dari daftar awal yang dicakup:** #5 (self-healing), #17 (observability: log, metrics, health check, event)

**Status:** Belum dimulai
**Dependency fase sebelumnya:** Fase 1 (orchestrator, event bus), Fase 2 (tunnel manager, untuk self-healing tunnel)

---

## Daftar Isi

1. [Goals](#goals)
2. [Non-Goals](#non-goals)
3. [Task Breakdown](#task-breakdown)
4. [Detail Teknis](#detail-teknis)
5. [Definition of Done](#definition-of-done)
6. [Risiko & Mitigasi](#risiko--mitigasi)

---

## Goals

- Container yang crash terdeteksi dan dipulihkan otomatis sesuai kebijakan yang bisa dikonfigurasi (retry limit, backoff).
- Tunnel yang terputus (dari Fase 2) mendapat kebijakan recovery yang lebih matang: backoff eksponensial, batas retry, alerting saat gagal terus-menerus.
- Semua komponen daemon menghasilkan log terstruktur (bukan sekadar `fmt.Println`), memudahkan debugging dan agregasi log terpusat.
- Metrics dalam format Prometheus (protokol metrics standar) diekspos untuk dikonsumsi oleh Prometheus Panel atau monitoring eksternal.
- Health check endpoint tersedia untuk memastikan daemon (dan dependency-nya seperti Docker socket) dalam keadaan sehat.
- Event penting (server crash, tunnel putus, resource limit terlampaui) tercatat dan bisa di-query riwayatnya.

## Non-Goals

- Dashboard visual untuk metrics/log — itu tanggung jawab Prometheus Panel (di luar cakupan daemon Oikos).
- Distributed tracing lintas node — di luar skala kebutuhan MVP, dipertimbangkan hanya jika multi-node jadi sangat kompleks di masa depan.
- Alerting eksternal (kirim ke Slack/Discord/email) — itu ranah Panel, daemon cukup mengekspos data yang dibutuhkan.

---

## Task Breakdown

### 1. Structured Logging (`internal/observability/logger.go`)
- [ ] Pilih library logging (`zerolog` direkomendasikan untuk performa & footprint kecil).
- [ ] Standarisasi field log wajib: `timestamp`, `level`, `component`, `server_id` (jika relevan), `message`.
- [ ] Ganti semua `fmt.Println`/`log.Println` di modul Fase 1 & 2 dengan logger terstruktur ini.
- [ ] Dukungan output ganda: stdout (untuk systemd journal) + file rotasi lokal opsional.

### 2. Metrics (`internal/observability/metrics.go`)
- [ ] Integrasikan `prometheus/client_golang`.
- [ ] Definisikan metrics inti:
  - `oikos_servers_total{status="running|stopped|crashed"}`
  - `oikos_server_cpu_usage_percent{server_id}`
  - `oikos_server_memory_usage_bytes{server_id}`
  - `oikos_tunnel_status{server_id, status}`
  - `oikos_daemon_uptime_seconds`
  - `oikos_selfheal_restarts_total{server_id}`
- [ ] Endpoint HTTP `/metrics` (di `internal/api/http/`).

### 3. Health Check (`internal/observability/health.go`)
- [ ] Endpoint `/healthz` — cek koneksi ke Docker socket, koneksi ke Panel (gRPC), akses baca/tulis SQLite.
- [ ] Bedakan `healthy`, `degraded` (mis. Panel unreachable tapi container tetap jalan), `unhealthy`.

### 4. Self-Healing Policy (`internal/orchestrator/selfheal.go` — perluasan dari Fase 1)
- [ ] Definisikan `RestartPolicy` per server (default: max 5 retry, backoff eksponensial mulai 5 detik).
- [ ] Watcher berlangganan event `server.crashed` dari event bus, evaluasi policy, panggil `lifecycle.RestartServer`.
- [ ] Setelah melewati max retry, ubah status server jadi `crash_looping` dan emit event `server.recovery_failed` (bukan terus mencoba tanpa henti).
- [ ] Reset counter retry setelah server stabil berjalan dalam durasi tertentu (mis. 5 menit tanpa crash).

### 5. Self-Healing Tunnel (perluasan `internal/tunnel/selfheal.go`)
- [ ] Terapkan `RestartPolicy` yang sama seperti container ke proses frpc.
- [ ] Bedakan jenis kegagalan: proses frpc crash vs koneksi ke frps terputus (network issue) — keduanya butuh penanganan sedikit berbeda.

### 6. Event History (`internal/store/` — tabel baru)
- [ ] Tabel `events` untuk menyimpan riwayat event penting (bukan hanya broadcast realtime, tapi juga queryable).
- [ ] Retensi/pruning otomatis (mis. simpan 30 hari terakhir) agar SQLite tidak membengkak.

### 7. Proto untuk Observability
- [ ] `proto/observability/observability.proto` — endpoint gRPC `StreamEvents`, `GetServerMetrics`, `GetHealthStatus` agar Panel bisa query tanpa scrape HTTP langsung (opsional, bisa juga cukup lewat `/metrics` HTTP standar Prometheus).

---

## Detail Teknis

### Skema Tambahan SQLite

```sql
-- 0003_observability.sql

CREATE TABLE events (
    id              TEXT PRIMARY KEY,
    type            TEXT NOT NULL,          -- server.crashed, tunnel.disconnected, dsb.
    server_id       TEXT REFERENCES servers(id) ON DELETE CASCADE,
    severity        TEXT NOT NULL DEFAULT 'info', -- info | warning | error | critical
    message         TEXT NOT NULL,
    metadata        TEXT NOT NULL DEFAULT '{}',   -- JSON bebas, mis. {"retry_count": 3}
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_events_created_at ON events(created_at);
CREATE INDEX idx_events_server_id ON events(server_id);

ALTER TABLE servers ADD COLUMN restart_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE servers ADD COLUMN last_crash_at DATETIME;
```

### Interface Restart Policy

```go
// internal/orchestrator/selfheal.go
package orchestrator

import "time"

type RestartPolicy struct {
    MaxRetries       int
    BackoffBase      time.Duration // mis. 5 * time.Second
    BackoffMax       time.Duration // mis. 5 * time.Minute
    StableResetAfter time.Duration // durasi sehat sebelum counter di-reset, mis. 5 * time.Minute
}

var DefaultRestartPolicy = RestartPolicy{
    MaxRetries:       5,
    BackoffBase:      5 * time.Second,
    BackoffMax:       5 * time.Minute,
    StableResetAfter: 5 * time.Minute,
}

// nextBackoff menghitung durasi tunggu sebelum retry berikutnya (exponential backoff).
func (p RestartPolicy) nextBackoff(attempt int) time.Duration {
    d := p.BackoffBase * time.Duration(1<<attempt)
    if d > p.BackoffMax {
        return p.BackoffMax
    }
    return d
}
```

```go
// Watcher yang bereaksi terhadap event server.crashed
type SelfHealer struct {
    bus       *EventBus
    lifecycle *Lifecycle
    policy    RestartPolicy
    logger    Logger
}

func (h *SelfHealer) Start(ctx context.Context) {
    events := h.bus.Subscribe("server.crashed")
    for {
        select {
        case <-ctx.Done():
            return
        case ev := <-events:
            h.handleCrash(ctx, ev.ServerID)
        }
    }
}

func (h *SelfHealer) handleCrash(ctx context.Context, serverID string) {
    server := h.lifecycle.GetServer(serverID)

    if server.RestartCount >= h.policy.MaxRetries {
        h.lifecycle.MarkCrashLooping(serverID)
        h.bus.Publish(Event{Type: "server.recovery_failed", ServerID: serverID})
        return
    }

    wait := h.policy.nextBackoff(server.RestartCount)
    h.logger.Warn("server crashed, scheduling restart",
        "server_id", serverID, "attempt", server.RestartCount+1, "wait", wait)

    time.AfterFunc(wait, func() {
        if err := h.lifecycle.RestartServer(ctx, serverID); err != nil {
            h.logger.Error("restart failed", "server_id", serverID, "error", err)
            return
        }
        h.lifecycle.IncrementRestartCount(serverID)
        h.bus.Publish(Event{Type: "server.recovered", ServerID: serverID})
    })
}
```

### Contoh Output Metrics (`/metrics`)

```
# HELP oikos_servers_total Jumlah server berdasarkan status
# TYPE oikos_servers_total gauge
oikos_servers_total{status="running"} 12
oikos_servers_total{status="stopped"} 3
oikos_servers_total{status="crashed"} 1

# HELP oikos_server_cpu_usage_percent Penggunaan CPU per server
# TYPE oikos_server_cpu_usage_percent gauge
oikos_server_cpu_usage_percent{server_id="a1b2c3"} 42.5

# HELP oikos_selfheal_restarts_total Total restart otomatis oleh self-healer
# TYPE oikos_selfheal_restarts_total counter
oikos_selfheal_restarts_total{server_id="a1b2c3"} 2

# HELP oikos_daemon_uptime_seconds Lama daemon berjalan
# TYPE oikos_daemon_uptime_seconds counter
oikos_daemon_uptime_seconds 183940
```

### Contoh Response Health Check

```json
{
  "status": "degraded",
  "checks": {
    "docker_socket": "healthy",
    "panel_connection": "unhealthy",
    "sqlite": "healthy"
  },
  "timestamp": "2026-09-04T10:15:00Z"
}
```

### Format Log Terstruktur (contoh output zerolog, JSON)

```json
{"level":"warn","component":"orchestrator.selfheal","server_id":"a1b2c3","attempt":2,"wait_ms":10000,"message":"server crashed, scheduling restart","time":"2026-09-04T10:15:03Z"}
{"level":"info","component":"tunnel.manager","server_id":"a1b2c3","status":"connected","message":"tunnel reconnected","time":"2026-09-04T10:15:14Z"}
```

---

## Definition of Done

- [ ] Container yang di-kill paksa otomatis restart sesuai `RestartPolicy`, dengan backoff yang benar-benar eksponensial (diverifikasi lewat log timestamp).
- [ ] Setelah melebihi `MaxRetries`, server berstatus `crash_looping` dan **berhenti** mencoba restart otomatis (tidak retry tanpa batas).
- [ ] Endpoint `/metrics` menghasilkan output valid yang bisa di-scrape Prometheus asli (divalidasi dengan `promtool check metrics`).
- [ ] Endpoint `/healthz` mengembalikan status yang akurat saat salah satu dependency (mis. Docker socket) sengaja dimatikan untuk testing.
- [ ] Semua log dari modul Fase 1 & 2 sudah bermigrasi ke logger terstruktur, tidak ada lagi `fmt.Println` untuk log operasional.
- [ ] Riwayat event bisa di-query dari tabel `events` dan otomatis terpangkas sesuai kebijakan retensi.
- [ ] Self-healing tunnel (dari Fase 2) sekarang memakai `RestartPolicy` yang sama dengan container, bukan lagi logic ad-hoc.

---

## Risiko & Mitigasi

| Risiko | Dampak | Mitigasi |
|---|---|---|
| Restart loop yang agresif menghabiskan resource sebelum `MaxRetries` tercapai | Node jadi lambat/tidak stabil untuk server lain | Backoff dimulai cukup tinggi (5 detik) dan naik cepat secara eksponensial |
| Event table tumbuh tanpa batas | SQLite membengkak, query jadi lambat | Job pruning berjalan berkala (mis. tiap 24 jam), hapus event lebih dari N hari |
| Metrics endpoint terbuka tanpa autentikasi | Informasi internal bocor ke jaringan lokal | Bind `/metrics` hanya ke localhost secara default, atau lindungi dengan token bila diekspos |
| False positive "crashed" untuk container yang sengaja berhenti (exit code 0 karena stop manual) | Self-healer mencoba restart server yang memang sedang di-stop | Bedakan exit code & sumber stop (manual command vs unexpected exit) sebelum publish event `server.crashed` |


---

# Fase 4 — Filesystem & Data

> Memberi user akses ke file server secara aman (API + SFTP) dan kemampuan backup/restore tanpa mengganggu proses yang sedang berjalan.

**Tujuan dari daftar awal yang dicakup:** #7 (filesystem modern: file manager/API/SFTP dengan permission konsisten), #12 (backup & restore tanpa mengganggu proses utama)

**Status:** Belum dimulai
**Dependency fase sebelumnya:** Fase 1 (orchestrator, security/permission dasar), Fase 3 (observability untuk memantau proses backup)

---

## Daftar Isi

1. [Goals](#goals)
2. [Non-Goals](#non-goals)
3. [Task Breakdown](#task-breakdown)
4. [Detail Teknis](#detail-teknis)
5. [Definition of Done](#definition-of-done)
6. [Risiko & Mitigasi](#risiko--mitigasi)

---

## Goals

- User bisa mengelola file server (baca, tulis, upload, download, rename, hapus) lewat API tanpa akses shell langsung ke host.
- SFTP tersedia sebagai alternatif akses file yang familiar bagi user teknis, tetap terisolasi per server.
- Permission antar-server konsisten dan terisolasi — server A tidak bisa membaca/menulis file milik server B dalam kondisi apa pun.
- Backup bisa dibuat kapan saja tanpa perlu menghentikan container yang sedang berjalan (backup "hot").
- Restore dari backup mengembalikan data server ke kondisi yang di-backup dengan aman, termasuk validasi integritas arsip.

## Non-Goals

- Backup terjadwal otomatis dengan retensi kompleks — cukup sediakan primitives (create/restore), penjadwalan adalah tanggung jawab Panel yang memanggil API ini secara berkala.
- Penyimpanan backup ke cloud storage (S3, dst) sebagai fitur bawaan daemon — di fase ini fokus ke local storage, dukungan storage eksternal bisa jadi plugin di Fase 5.
- Version control penuh gaya Git untuk file server — di luar cakupan.

---

## Task Breakdown

### 1. Filesystem Manager (`internal/filesystem/manager.go`)
- [ ] Operasi dasar: `ListDir`, `ReadFile`, `WriteFile`, `Rename`, `Delete`, `CreateDir`, `Move`, `Copy`.
- [ ] Semua operasi dibatasi ke dalam root direktori data server masing-masing (chroot-like path resolution, cegah path traversal `../../`).
- [ ] Dukungan streaming untuk file besar (upload/download tidak boleh memuat seluruh file ke memori).

### 2. Permission Isolation (`internal/filesystem/permission.go`)
- [ ] Setiap server punya UID/GID host yang berbeda (mapping user namespace bila memungkinkan) agar isolasi berlaku di level filesystem OS, bukan hanya logika aplikasi.
- [ ] Validasi setiap request path selalu di-resolve absolut dan dicek berada di dalam boundary direktori server sebelum operasi dieksekusi.

### 3. SFTP Server (`internal/filesystem/sftp.go`)
- [ ] Implementasi SFTP server (mis. pakai `pkg/sftp` + `golang.org/x/crypto/ssh`) yang berjalan sebagai satu proses/listener di daemon.
- [ ] Autentikasi SFTP terhubung ke sistem kredensial yang sama dengan Panel (bukan sistem user Linux asli) — token/keypair per-server atau per-user.
- [ ] Setiap sesi SFTP di-jail ke root direktori server yang bersangkutan, memakai `filesystem.Manager` yang sama sebagai backend (bukan duplikasi logic).

### 4. API File Manager (`internal/api/` + `proto/filesystem/`)
- [ ] `proto/filesystem/filesystem.proto` — RPC untuk list/read/write/delete/rename, plus streaming RPC untuk upload/download.
- [ ] Validasi ukuran file & kuota disk (terhubung ke `internal/resource/disk.go` dari Fase 1) sebelum menerima upload.

### 5. Backup Manager (`internal/backup/`)
- [ ] `archive.go` — buat arsip (tar.gz atau zstd) dari direktori data server sambil container tetap berjalan (baca file secara langsung dari disk, bukan lewat container).
- [ ] Strategi konsistensi: gunakan filesystem snapshot bila tersedia (mis. overlay/btrfs/zfs), atau fallback ke copy langsung dengan catatan bahwa file yang sedang ditulis intensif berisiko sedikit tidak konsisten.
- [ ] `manager.go` — orkestrasi proses backup: mulai, lapor progres, simpan metadata backup (ukuran, checksum, waktu).
- [ ] `restore.go` — ekstraksi arsip ke direktori server, dengan opsi restore ke server yang sama (setelah stop) atau ke server baru.
- [ ] Checksum (SHA-256) disimpan saat backup dibuat, divalidasi ulang saat restore untuk deteksi korupsi.

### 6. Integrasi Lifecycle
- [ ] `BackupServer` sebaiknya tidak mengharuskan `StopServer` — namun beri opsi eksplisit "stop before backup" untuk kasus yang butuh konsistensi penuh (mis. database).
- [ ] `RestoreServer` **mengharuskan** server dalam status stopped sebelum proses restore dimulai, untuk mencegah file ditulis dua arah sekaligus.

---

## Detail Teknis

### Skema Tambahan SQLite

```sql
-- 0004_filesystem_backup.sql

CREATE TABLE backups (
    id              TEXT PRIMARY KEY,
    server_id       TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    file_path       TEXT NOT NULL,          -- path arsip di disk node
    size_bytes      INTEGER NOT NULL,
    checksum_sha256 TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'creating',
                    -- creating | completed | failed | restoring
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at    DATETIME
);

CREATE TABLE sftp_credentials (
    id              TEXT PRIMARY KEY,
    server_id       TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    username        TEXT NOT NULL,
    public_key      TEXT,                    -- jika autentikasi berbasis keypair
    password_hash   TEXT,                    -- jika autentikasi berbasis password (opsional)
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_backups_server_id ON backups(server_id);
```

### Interface Filesystem Manager

```go
// internal/filesystem/manager.go
package filesystem

import (
    "context"
    "io"
    "time"
)

type FileInfo struct {
    Name       string
    Path       string
    IsDir      bool
    SizeBytes  int64
    ModifiedAt time.Time
    Mode       string // representasi permission, mis. "rwxr-xr-x"
}

// Manager membatasi semua operasi ke dalam root direktori satu server.
// Implementasi WAJIB menolak path yang mencoba keluar dari root (path traversal).
type Manager interface {
    List(ctx context.Context, serverID, relPath string) ([]FileInfo, error)
    Read(ctx context.Context, serverID, relPath string) (io.ReadCloser, error)
    Write(ctx context.Context, serverID, relPath string, content io.Reader) error
    Delete(ctx context.Context, serverID, relPath string, recursive bool) error
    Rename(ctx context.Context, serverID, oldPath, newPath string) error
    Copy(ctx context.Context, serverID, srcPath, dstPath string) error
    CreateDir(ctx context.Context, serverID, relPath string) error
    ResolvePath(serverID, relPath string) (absPath string, err error) // dipakai internal, cek boundary
}
```

### Contoh Resolusi Path Aman (Anti Path-Traversal)

```go
// internal/filesystem/manager.go

func (m *manager) ResolvePath(serverID, relPath string) (string, error) {
    root := m.serverRoot(serverID) // mis. /var/lib/oikos/servers/<serverID>/data

    cleaned := filepath.Clean("/" + relPath) // paksa treat sebagai absolute dari root
    abs := filepath.Join(root, cleaned)

    // Pastikan hasil akhir tetap di dalam root, cegah "../../../etc/passwd"
    if !strings.HasPrefix(abs, root+string(os.PathSeparator)) && abs != root {
        return "", ErrPathOutsideRoot
    }
    return abs, nil
}
```

### Interface Backup Manager

```go
// internal/backup/manager.go
package backup

import "context"

type BackupOptions struct {
    ServerID      string
    StopBeforeRun bool // true jika ingin backup konsisten penuh dgn stop container
}

type BackupResult struct {
    BackupID    string
    SizeBytes   int64
    ChecksumSHA string
}

type RestoreOptions struct {
    ServerID string
    BackupID string
    // TargetServerID kosong berarti restore ke server yang sama
    TargetServerID string
}

type Manager interface {
    Create(ctx context.Context, opts BackupOptions) (BackupResult, error)
    Restore(ctx context.Context, opts RestoreOptions) error
    List(ctx context.Context, serverID string) ([]BackupResult, error)
    Delete(ctx context.Context, backupID string) error
}
```

### Alur Backup "Hot" (Tanpa Stop Container)

```
Panel/API panggil backup.Create({ServerID, StopBeforeRun: false})
   │
   ▼
backup/manager.go tandai status "creating" di tabel backups
   │
   ▼
archive.go baca langsung direktori data server dari host filesystem
   (tidak lewat container — container tetap berjalan normal)
   │
   ▼
Kompres jadi .tar.zst secara streaming, hitung checksum SHA-256 on-the-fly
   │
   ▼
Simpan arsip ke direktori backup node, update tabel backups: status "completed"
   │
   ▼
Event "backup.completed" di-emit ke event bus (Fase 3)
```

> **Catatan konsistensi:** untuk aplikasi yang sangat sensitif terhadap file yang sedang ditulis (mis. database dengan file lock aktif), user/Panel disarankan memakai `StopBeforeRun: true` demi keamanan data, karena backup hot pada dasarnya adalah *best-effort consistency*, bukan snapshot atomik kecuali filesystem host mendukung snapshot native (Btrfs/ZFS/LVM).

### Alur Restore

```
Panel/API panggil backup.Restore({ServerID, BackupID})
   │
   ▼
Cek status server HARUS "stopped" — tolak jika masih running
   │
   ▼
Validasi checksum arsip backup (cegah restore dari file korup)
   │
   ▼
Kosongkan/backup-sementara direktori data server saat ini (safety net)
   │
   ▼
Ekstrak arsip ke direktori data server
   │
   ▼
Event "restore.completed" di-emit, status server kembali "stopped" (siap di-start manual)
```

---

## Definition of Done

- [ ] File manager API bisa list/read/write/delete/rename file dalam server tanpa bisa mengakses file di luar root server tersebut (diverifikasi dengan uji path traversal aktif).
- [ ] Upload/download file besar (mis. 1GB+) tidak menyebabkan lonjakan memori signifikan pada daemon (diverifikasi via profiling).
- [ ] SFTP bisa dipakai client SFTP standar (FileZilla/WinSCP/`sftp` CLI) untuk akses file satu server, dan tidak bisa menembus ke direktori server lain.
- [ ] Backup berhasil dibuat saat container sedang berjalan aktif (hot), arsip valid dan bisa diekstrak ulang.
- [ ] Restore menolak dijalankan jika server berstatus running, dan berhasil mengembalikan data secara utuh (checksum cocok) saat server stopped.
- [ ] Kuota disk (dari Fase 1 `resource/disk.go`) benar-benar mencegah upload yang melebihi batas.

---

## Risiko & Mitigasi

| Risiko | Dampak | Mitigasi |
|---|---|---|
| Path traversal luput dari validasi | Server bisa baca/tulis file host di luar sandbox — risiko keamanan serius | Sentralisasi semua resolusi path lewat satu fungsi `ResolvePath`, uji otomatis dengan payload traversal umum |
| Backup hot pada file yang sedang ditulis intensif menghasilkan arsip tidak konsisten | Data corrupt saat restore | Dokumentasikan batasan jelas ke user, sediakan opsi `StopBeforeRun`, pertimbangkan dukungan snapshot filesystem di masa depan |
| SFTP server jadi vektor serangan tambahan (permukaan baru) | Kerentanan pada implementasi SSH/SFTP | Gunakan library matang (`pkg/sftp`, `golang.org/x/crypto/ssh`), batasi cipher/algoritma ke yang aman, audit berkala |
| Disk penuh karena banyak backup menumpuk | Node kehabisan storage, server lain ikut terdampak | Sediakan API `Delete` backup lama, beri warning/limit jumlah backup per server di level Panel |


---

# Fase 5 — Ekstensibilitas & Portabilitas

> Membuat Oikos bisa berkembang tanpa mengubah core, dan bisa di-update tanpa mengganggu server yang sedang berjalan.

**Tujuan dari daftar awal yang dicakup:** #13 (plugin system), #16 (graceful update), #15 (cross-platform), #2 (mudah dipakai — hasil kumulatif, divalidasi ulang di fase ini)

**Status:** Belum dimulai
**Dependency fase sebelumnya:** Fase 1–4 (butuh event bus, orchestrator, filesystem, tunnel, backup semuanya stabil sebagai "permukaan" yang bisa di-extend plugin)

---

## Daftar Isi

1. [Goals](#goals)
2. [Non-Goals](#non-goals)
3. [Task Breakdown](#task-breakdown)
4. [Detail Teknis](#detail-teknis)
5. [Definition of Done](#definition-of-done)
6. [Risiko & Mitigasi](#risiko--mitigasi)

---

## Goals

- Fitur baru bisa ditambahkan ke Oikos melalui plugin, tanpa mengubah/mengkompilasi ulang core daemon.
- Plugin bisa bereaksi terhadap event (lewat event bus dari Fase 3) dan menambahkan endpoint API sendiri.
- Update daemon ke versi baru tidak menghentikan atau merusak server yang sedang berjalan.
- Konfigurasi dan state (SQLite) tetap utuh setelah proses update, termasuk bila update gagal di tengah jalan (rollback aman).
- Struktur internal tidak terlalu terikat ke asumsi khusus Linux, membuka jalan dukungan platform lain di masa depan tanpa rewrite besar (meski Linux tetap target utama).

## Non-Goals

- Marketplace/plugin registry publik dengan sistem rating, dsb — itu ranah Panel/ekosistem, bukan daemon.
- Dukungan penuh Windows/macOS sebagai node produksi di fase ini — cukup pastikan struktur kode tidak "mengunci" ke Linux secara membabi buta (mis. hardcode syscall Linux tanpa abstraksi).
- Live migration server antar-node saat update — di luar cakupan, karena itu masalah orkestrasi multi-node yang jauh lebih kompleks.

---

## Task Breakdown

### 1. Plugin System (`internal/plugin/`)
- [ ] Tentukan mekanisme loading plugin: Go plugin (`.so`, punya banyak keterbatasan versi) vs proses terpisah dengan gRPC (lebih portable, direkomendasikan berdasarkan pengalaman ekosistem serupa seperti HashiCorp go-plugin).
- [ ] `api.go` — definisikan kontrak plugin: event apa saja yang bisa di-subscribe, API apa yang bisa didaftarkan.
- [ ] `host.go` — proses lifecycle plugin: load saat startup, health check plugin, unload saat shutdown/reload.
- [ ] `registry.go` — daftar plugin terpasang, versi, status aktif/nonaktif, disimpan di `config/plugins.yaml` atau tabel SQLite.

### 2. Kontrak API untuk Plugin
- [ ] Plugin bisa subscribe ke event bus (mis. plugin "Discord notifier" subscribe ke `server.crashed`).
- [ ] Plugin bisa mendaftarkan custom gRPC/HTTP endpoint tambahan yang di-mount di bawah namespace tersendiri (mis. `/plugins/<plugin-name>/...`).
- [ ] Plugin punya akses terbatas dan eksplisit — tidak otomatis full access ke seluruh internal state (least privilege).

### 3. Graceful Update (`cmd/oikos/` + proses baru)
- [ ] Mekanisme deteksi versi baru (manual trigger dulu: `oikos update`, bukan auto-update otomatis tanpa izin admin).
- [ ] Strategi "binary swap tanpa downtime container": daemon baru start, ambil alih koneksi ke Panel, daemon lama shutdown setelah memastikan container-container tidak bergantung langsung pada proses daemon (karena Docker container tetap jalan independen dari proses Oikos berkat containerd).
- [ ] State SQLite dan config tidak boleh disentuh/dimigrasi otomatis tanpa backup — buat backup otomatis sebelum migrasi schema baru dijalankan.
- [ ] Rollback plan: jika daemon baru gagal start (health check gagal), otomatis kembalikan binary lama.

### 4. Cross-Platform Readiness (`internal/runtime/`, `internal/resource/`)
- [ ] Audit kode yang bergantung langsung ke fitur spesifik Linux (cgroups, syscall tertentu) dan pastikan terisolasi di balik interface (`resource.LimitEnforcer`, dst) — bukan tersebar di banyak file.
- [ ] Tandai jelas modul mana yang "Linux-only" vs "portable", dokumentasikan di `docs/architecture.md`.
- [ ] Uji build (`GOOS=darwin`, `GOOS=windows`) tetap sukses dikompilasi meski fitur tertentu di-stub/nonaktif di platform tsb — tujuannya portabilitas kode, bukan node produksi penuh di platform lain.

### 5. Validasi Ulang "Mudah Dipakai" (#2)
- [ ] Review ulang seluruh command CLI (`oikos install`, `oikos update`, dst) — pastikan tetap simpel meski fitur bertambah banyak sejak Fase 1.
- [ ] Sediakan `oikos doctor` — command diagnostik menyeluruh (cek Docker, cgroups, koneksi Panel, plugin bermasalah, dst) dalam satu perintah.

---

## Detail Teknis

### Skema Tambahan SQLite

```sql
-- 0005_plugins.sql

CREATE TABLE plugins (
    id              TEXT PRIMARY KEY,       -- slug plugin
    name            TEXT NOT NULL,
    version         TEXT NOT NULL,
    binary_path     TEXT NOT NULL,          -- path ke binary plugin (proses terpisah)
    enabled         BOOLEAN NOT NULL DEFAULT 1,
    subscribed_events TEXT NOT NULL DEFAULT '[]', -- JSON array, mis. ["server.crashed"]
    installed_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### Arsitektur Plugin: Proses Terpisah via gRPC (Direkomendasikan)

Mengikuti pola `hashicorp/go-plugin`: plugin adalah **binary terpisah** yang dijalankan sebagai subprocess, berkomunikasi dengan daemon utama lewat gRPC melalui local socket. Ini dipilih dibanding Go native plugin (`.so`) karena:

| Aspek | Go Plugin (`.so`) | Proses Terpisah + gRPC |
|---|---|---|
| Kompatibilitas versi Go | Sangat ketat (harus versi identik) | Bebas, plugin bisa beda versi Go |
| Crash isolation | Plugin crash = daemon ikut crash | Plugin crash tidak menjatuhkan daemon |
| Cross-platform | Tidak didukung di Windows | Berjalan di semua platform |
| Kemudahan development pihak ketiga | Sulit, harus compile bareng | Mudah, bisa bahasa apa pun asal bicara gRPC |

> Keputusan ini konsisten dengan prinsip yang sudah dipakai di Fase 2 untuk frpc — proses terpisah demi isolasi failure.

### Interface Plugin Contract

```go
// internal/plugin/api.go
package plugin

import "context"

type Event struct {
    Type     string
    ServerID string
    Metadata map[string]string
}

// PluginServer adalah kontrak gRPC yang WAJIB diimplementasikan setiap plugin.
// Didefinisikan penuh di proto/plugin/plugin.proto, ringkasan Go interface-nya:
type PluginServer interface {
    // Dipanggil sekali saat plugin di-load, plugin mengembalikan event apa saja
    // yang ingin di-subscribe dan endpoint tambahan apa yang ingin didaftarkan.
    Init(ctx context.Context, req InitRequest) (InitResponse, error)

    // Dipanggil setiap kali event yang di-subscribe terjadi.
    HandleEvent(ctx context.Context, ev Event) error

    // Dipanggil daemon secara berkala untuk memastikan plugin masih sehat.
    HealthCheck(ctx context.Context) (Healthy bool, err error)
}

type InitRequest struct {
    OikosVersion string
    ConfigPath   string // path config khusus plugin ini, mis. config/plugins/discord-notifier.yaml
}

type InitResponse struct {
    SubscribedEvents []string
    RegisteredRoutes []string // mis. ["/plugins/discord-notifier/webhook"]
}
```

### Contoh `plugins.yaml`

```yaml
# config/plugins.yaml
plugins:
  - id: "discord-notifier"
    binary: "/var/lib/oikos/plugins/discord-notifier"
    enabled: true
    config: "/etc/oikos/plugins/discord-notifier.yaml"
  - id: "s3-backup-storage"
    binary: "/var/lib/oikos/plugins/s3-backup-storage"
    enabled: false
    config: "/etc/oikos/plugins/s3-backup-storage.yaml"
```

### Alur Graceful Update

```
Admin jalankan: oikos update
   │
   ▼
1. Backup config.yaml + database SQLite ke folder timestamped
   │
   ▼
2. Download/verifikasi binary versi baru (checksum & signature)
   │
   ▼
3. Jalankan binary baru sebagai proses terpisah dgn flag --health-check-only
   (cek: bisa baca config, bisa buka SQLite, bisa connect Docker socket)
   │
   ▼
   Gagal? ──> Abort, laporkan error, binary lama tetap jalan (tidak ada downtime)
   │
   Berhasil
   ▼
4. Jalankan migrasi schema SQLite baru (jika ada) di dalam transaksi
   │
   ▼
5. Signal proses lama utk berhenti menerima command baru (graceful drain)
   │
   ▼
6. Proses baru ambil alih: buka koneksi gRPC ke Panel, resume monitoring container
   (container yang sudah berjalan di Docker TIDAK di-restart — mereka independen dari proses Oikos)
   │
   ▼
7. Proses lama exit sepenuhnya setelah proses baru konfirmasi "ready"
   │
   ▼
8. Event "daemon.updated" di-emit, tercatat di tabel events (Fase 3)
```

> **Kunci graceful update:** container Docker berjalan sebagai proses independen dikelola oleh Docker daemon/containerd, **bukan** child process dari Oikos. Ini berarti mematikan/mengganti proses Oikos tidak otomatis mematikan container — properti inilah yang dimanfaatkan agar update daemon tidak mengganggu server yang sedang berjalan.

### Audit Cross-Platform — Contoh Isolasi

```go
// internal/resource/limiter.go — interface yang platform-agnostic
package resource

type LimitEnforcer interface {
    ApplyCPULimit(containerID string, milicores int64) error
    ApplyMemoryLimit(containerID string, bytes int64) error
    ApplyPIDLimit(containerID string, max int64) error
}

// internal/resource/cgroups_linux.go — implementasi khusus Linux
//go:build linux

package resource

type cgroupsEnforcer struct{}

func NewLimitEnforcer() LimitEnforcer { return &cgroupsEnforcer{} }
// ... implementasi cgroups v2 ...
```

```go
// internal/resource/limiter_unsupported.go — fallback platform lain
//go:build !linux

package resource

type noopEnforcer struct{}

func NewLimitEnforcer() LimitEnforcer { return &noopEnforcer{} }

func (n *noopEnforcer) ApplyCPULimit(_ string, _ int64) error {
    return ErrNotSupportedOnPlatform
}
// dst — daemon tetap bisa jalan/compile, fitur limit dinonaktifkan dgn jelas
```

---

## Definition of Done

- [ ] Plugin contoh (mis. "log-to-file" sederhana) berhasil di-load, subscribe ke event `server.crashed`, dan bereaksi tanpa mengubah kode core.
- [ ] Plugin yang sengaja dibuat crash tidak menyebabkan daemon utama ikut down (crash isolation terbukti via test).
- [ ] `oikos update` berhasil mengganti binary daemon dari versi lama ke baru tanpa merestart container yang sedang berjalan (diverifikasi: uptime container tidak reset).
- [ ] Update yang gagal health-check di tengah jalan otomatis rollback ke binary lama tanpa kehilangan data config/state.
- [ ] `go build` untuk `GOOS=linux`, `GOOS=darwin`, `GOOS=windows` semuanya sukses tanpa error compile (meski fitur resource-limit di-stub pada platform selain Linux).
- [ ] `oikos doctor` menghasilkan laporan diagnostik yang jelas dan actionable saat salah satu dependency bermasalah.

---

## Risiko & Mitigasi

| Risiko | Dampak | Mitigasi |
|---|---|---|
| Plugin pihak ketiga berkualitas rendah membebani resource daemon | Melanggar tujuan #1 (ringan) dan #8 (lightweight) | Terapkan resource limit juga pada proses plugin, sediakan health check & auto-disable plugin yang bermasalah |
| Graceful update gagal di tengah migrasi schema SQLite | Data korup atau daemon tidak bisa start sama sekali | Migrasi schema di dalam transaksi database, backup wajib sebelum migrasi, dan uji rollback secara rutin |
| Interface plugin terlalu terbuka/powerful | Risiko keamanan — plugin jahat bisa akses berlebihan | Terapkan least privilege sejak desain awal kontrak `PluginServer`, plugin hanya dapat apa yang secara eksplisit diberikan lewat `InitResponse` |
| Klaim cross-platform menciptakan ekspektasi salah bahwa Windows/macOS didukung penuh | User kecewa saat fitur resource-limit tidak jalan di non-Linux | Dokumentasikan dengan jelas bahwa Linux adalah target produksi utama, platform lain hanya untuk keperluan development/testing kode |


---

# Fase 6 — Production Hardening

> Fase terakhir sebelum Oikos dianggap siap produksi: mengeraskan keamanan, memvalidasi performa di skala nyata, dan menyiapkan seluruh dokumentasi & tooling operasional.

**Tujuan dari daftar awal yang dicakup:** Konsolidasi & validasi akhir seluruh 18 tujuan, dengan fokus khusus pada #9 (security-first, tingkat lanjut) dan #1/#8 (ringan & lightweight, dibuktikan lewat benchmark nyata)

**Status:** Belum dimulai
**Dependency fase sebelumnya:** Fase 1–5 (seluruh fitur inti sudah ada, fase ini adalah pengerasan & validasi, bukan fitur baru)

---

## Daftar Isi

1. [Goals](#goals)
2. [Non-Goals](#non-goals)
3. [Task Breakdown](#task-breakdown)
4. [Detail Teknis](#detail-teknis)
5. [Definition of Done](#definition-of-done)
6. [Risiko & Mitigasi](#risiko--mitigasi)
7. [Checklist Rilis Production](#checklist-rilis-production)

---

## Goals

- Sandboxing container diperkuat melampaui isolasi default Docker (seccomp, AppArmor/SELinux profile, user namespace remapping).
- Daemon divalidasi lewat benchmark nyata untuk membuktikan klaim "ringan" dan "lightweight" (#1, #8) dengan angka konkret, bukan asumsi.
- Audit keamanan menyeluruh: permission isolation, mTLS, token handling, plugin sandboxing, filesystem boundary — semua diuji dengan skenario serangan nyata (path traversal, privilege escalation, dst).
- Dokumentasi lengkap untuk instalasi, operasional harian, troubleshooting, dan disaster recovery.
- Uji beban (load test) dengan jumlah server/node yang merepresentasikan skenario produksi realistis.
- Proses rilis, versioning, dan changelog terstandarisasi.

## Non-Goals

- Fitur baru — fase ini murni pengerasan, optimasi, dan validasi dari fitur yang sudah ada di Fase 1–5.
- Sertifikasi keamanan formal (SOC2, ISO 27001, dst) — di luar cakupan proyek open-source/personal pada tahap ini, meski praktik yang diterapkan mendukung ke arah sana.

---

## Task Breakdown

### 1. Sandboxing Lanjutan (`internal/security/sandbox.go` — perluasan dari Fase 1)
- [ ] Terapkan seccomp profile default yang membatasi syscall berbahaya untuk semua container (bukan hanya mengandalkan default Docker).
- [ ] Evaluasi AppArmor (Ubuntu/Debian) dan/atau SELinux (RHEL-based) profile tambahan, dengan fallback jelas jika tidak tersedia di sistem.
- [ ] Terapkan user namespace remapping agar root di dalam container tidak sama dengan root di host.
- [ ] Audit `internal/filesystem/permission.go` ulang dengan skenario serangan spesifik (symlink attack, race condition TOCTOU).

### 2. Audit Keamanan Menyeluruh
- [ ] Checklist audit manual: token lifecycle (Fase 1), mTLS cert rotation & revocation, path traversal (Fase 4), plugin privilege (Fase 5).
- [ ] Jalankan static analysis security scanner (`gosec` atau setara) di CI, selesaikan semua temuan level tinggi.
- [ ] Uji penetrasi dasar (self-conducted atau melibatkan pihak lain) terhadap API gRPC dan SFTP.
- [ ] Review dependency pihak ketiga (Docker SDK, sftp lib, dll) untuk known CVE — integrasikan `govulncheck` di CI.

### 3. Benchmark & Validasi Performa
- [ ] Benchmark idle resource usage (RAM/CPU) daemon di berbagai jumlah server aktif (0, 10, 50, 100 server).
- [ ] Bandingkan head-to-head dengan Wings pada beban kerja yang sama (jumlah server, jenis egg) untuk memvalidasi klaim "lebih ringan" secara kuantitatif.
- [ ] Load test: berapa banyak server yang bisa dikelola satu node sebelum degradasi performa terjadi.
- [ ] Profiling (`pprof`) untuk temukan bottleneck CPU/memory yang tidak terlihat dari benchmark permukaan.

### 4. Observability Produksi
- [ ] Pastikan semua metrics dari Fase 3 punya dashboard referensi (Grafana JSON model) yang bisa langsung dipakai Prometheus Panel atau tools lain.
- [ ] Definisikan SLO/SLI dasar (mis. "99% command dari Panel direspons dalam 500ms").
- [ ] Alerting rule referensi (meski eksekusi alerting ada di Panel, daemon harus expose metrics yang cukup untuk itu).

### 5. Dokumentasi Produksi
- [ ] `docs/architecture.md` — finalisasi diagram & penjelasan menyeluruh (update dari seluruh fase 1-5).
- [ ] `docs/installation.md` — panduan instalasi produksi (bukan hanya `install.sh` sederhana, tapi juga hardening checklist).
- [ ] `docs/operations.md` — panduan operasional harian: cara update, cara backup config, cara membaca log/metrics.
- [ ] `docs/disaster-recovery.md` — langkah pemulihan bila node rusak total, database SQLite korup, dst.
- [ ] `docs/security.md` — model ancaman (threat model), batasan keamanan yang diketahui, cara melaporkan vulnerability.
- [ ] `docs/egg-spec.md` — finalisasi spesifikasi `egg.yaml` versi stabil (versioning skema egg itu sendiri).

### 6. Release Engineering
- [ ] Semantic versioning untuk Oikos (`v1.0.0` sebagai target rilis awal produksi).
- [ ] `CHANGELOG.md` terstruktur (Keep a Changelog format).
- [ ] Build pipeline reproducible (checksum, signing binary rilis).
- [ ] Strategi dukungan versi (berapa lama versi lama didukung setelah rilis baru).

### 7. Chaos & Failure Testing
- [ ] Simulasi kegagalan: Docker daemon mati mendadak, disk penuh, koneksi ke Panel putus lama, SQLite file terkunci.
- [ ] Pastikan setiap skenario di atas menghasilkan perilaku predictable (bukan crash tanpa log, bukan silent failure) — validasi ulang self-healing (Fase 3) di kondisi ekstrem.

---

## Detail Teknis

### Contoh Seccomp Profile Dasar

```json
{
  "defaultAction": "SCMP_ACT_ERRNO",
  "architectures": ["SCMP_ARCH_X86_64"],
  "syscalls": [
    {
      "names": [
        "read", "write", "open", "close", "stat", "fstat",
        "mmap", "mprotect", "munmap", "brk", "rt_sigaction",
        "access", "execve", "socket", "connect", "accept",
        "clone", "fork", "wait4", "exit", "exit_group"
      ],
      "action": "SCMP_ACT_ALLOW"
    }
  ]
}
```
> Ini contoh minimal ilustratif — profil produksi nyata harus disusun berdasarkan analisis syscall yang benar-benar dibutuhkan tiap jenis egg (mis. game server vs aplikasi web punya kebutuhan syscall berbeda), idealnya di-generate dari trace (`strace`) berbagai egg populer.

### Struktur Benchmark Report

```
docs/benchmarks/
├── README.md                    # metodologi pengujian
├── idle-resource-usage.md       # RAM/CPU idle vs jumlah server
├── oikos-vs-wings-comparison.md # perbandingan head-to-head
└── load-test-100-servers.md     # hasil load test skala besar
```

Contoh tabel hasil benchmark (format yang diharapkan diisi setelah pengujian nyata):

| Jumlah Server Aktif | RAM Oikos (MB) | RAM Wings (MB) | CPU Idle Oikos (%) | CPU Idle Wings (%) |
|---|---|---|---|---|
| 0 | TBD | TBD | TBD | TBD |
| 10 | TBD | TBD | TBD | TBD |
| 50 | TBD | TBD | TBD | TBD |
| 100 | TBD | TBD | TBD | TBD |

> Nilai "TBD" wajib diisi dengan data hasil pengujian aktual sebelum fase ini dianggap selesai — klaim "lebih ringan" (#1) tidak boleh berhenti di asumsi desain, harus dibuktikan.

### Threat Model Ringkas (`docs/security.md` — kerangka)

| Aset | Ancaman | Mitigasi yang Relevan |
|---|---|---|
| Cert mTLS node | Pencurian cert → node palsu bisa terhubung ke Panel | Cert disimpan permission 0600, rotasi berkala, revocation list di Panel |
| Data server (file) | Path traversal antar-server | `filesystem.ResolvePath` (Fase 4) + audit ulang di fase ini |
| Container | Container escape ke host | Seccomp, user namespace, AppArmor/SELinux (task 1 fase ini) |
| Token pairing | Token dicuri sebelum dipakai | Token expire pendek (mis. 15 menit), sekali pakai (Fase 1) |
| Plugin | Plugin jahat/berbahaya | Least privilege contract (Fase 5), plugin resource limit |
| SQLite | Akses langsung ke file DB oleh proses lain | Permission file ketat, opsional enkripsi at-rest untuk data sensitif |

### Contoh SLO Dasar

```yaml
# docs/observability/slo.yaml (referensi, dikonsumsi Panel/monitoring)
slo:
  - name: "command_response_time"
    description: "Command dari Panel direspons node dalam waktu wajar"
    target: "p99 < 500ms"
  - name: "self_heal_recovery_time"
    description: "Server crash pulih otomatis dalam waktu wajar"
    target: "p95 < 30s (untuk MaxRetries pertama)"
  - name: "daemon_availability"
    description: "Daemon tetap responsif terhadap heartbeat"
    target: "99.5% uptime per 30 hari"
```

---

## Definition of Done

- [ ] Seccomp/AppArmor profile aktif secara default pada container yang dibuat Oikos, dengan fallback graceful jika platform tidak mendukung.
- [ ] `gosec` dan `govulncheck` berjalan di CI tanpa temuan level tinggi yang belum diselesaikan.
- [ ] Laporan benchmark lengkap tersedia di `docs/benchmarks/`, mengisi seluruh data "TBD" dengan angka nyata, termasuk perbandingan terhadap Wings.
- [ ] Seluruh dokumen di `docs/` (architecture, installation, operations, disaster-recovery, security, egg-spec) selesai dan konsisten dengan implementasi aktual.
- [ ] Chaos testing (Docker mati, disk penuh, Panel unreachable lama, SQLite terkunci) semuanya menghasilkan perilaku yang terekam jelas di log/event, tidak ada silent failure.
- [ ] Rilis `v1.0.0` diterbitkan dengan changelog lengkap, binary tersigning, dan checksum terverifikasi.

---

## Risiko & Mitigasi

| Risiko | Dampak | Mitigasi |
|---|---|---|
| Benchmark menunjukkan Oikos ternyata tidak lebih ringan dari Wings di beberapa skenario | Klaim utama proyek (#1) tidak terbukti | Terima hasil apa adanya, identifikasi bottleneck spesifik lewat profiling, iterasi ulang sebelum rilis final — jangan memoles angka |
| Hardening keamanan (seccomp/AppArmor) menyebabkan sebagian egg lama gagal jalan (syscall diblokir) | Kompatibilitas mundur egg terganggu | Sediakan mode "permissive" opsional dengan warning jelas, uji regresi terhadap seluruh egg contoh di `eggs/` |
| Dokumentasi tertinggal dari implementasi aktual karena dikerjakan di akhir | Dokumentasi production-grade jadi tidak akurat | Idealnya dokumentasi di-draft inkremental sejak Fase 1, fase ini hanya finalisasi & konsolidasi, bukan menulis dari nol |
| Proses rilis manual rawan human error (lupa sign, lupa update changelog) | Rilis cacat ke production | Otomasi lewat CI/CD pipeline release, checklist rilis wajib diikuti (lihat di bawah) |

---

## Checklist Rilis Production

- [ ] Semua Definition of Done dari Fase 1–6 terpenuhi.
- [ ] `CHANGELOG.md` diperbarui sesuai format Keep a Changelog.
- [ ] Versi di-tag mengikuti semantic versioning (`v1.0.0`).
- [ ] Binary rilis dibangun lewat CI (bukan build manual di mesin developer), disertai checksum SHA-256.
- [ ] Dokumentasi instalasi diuji ulang dari nol di VM bersih oleh seseorang yang bukan penulis dokumen tersebut.
- [ ] Threat model (`docs/security.md`) direview minimal oleh satu orang lain selain penulis kode.
- [ ] Rollback plan terdokumentasi jika `v1.0.0` ternyata bermasalah di produksi.
