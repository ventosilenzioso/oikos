# Oikos Fase 2: Networking Design

Tanggal: 2026-09-13  
Status: Disetujui dalam sesi brainstorming, menunggu review file

## Tujuan

Fase 2 menambahkan networking berbasis frp sehingga node di belakang NAT/CGNAT
dapat mengekspos port server ke publik tanpa port forwarding manual. Fase ini
juga menyiapkan private network multi-node melalui relay frps, reconnect dasar,
dan API/proto untuk assignment tunnel.

## Scope MVP

- `frps` berjalan sebagai deployment terpisah dari daemon Oikos.
- Oikos node menjalankan satu `frpc` sebagai child process untuk seluruh proxy
  aktif pada node tersebut.
- Agent meminta remote port melalui RPC mTLS ke Panel; Panel mengalokasikan
  port dari range yang dikonfigurasi.
- Mapping disimpan di SQLite dan konfigurasi `frpc.toml` dibuat otomatis.
- Reload dilakukan dengan graceful restart child process; daemon Oikos tidak
  ikut restart ketika frpc gagal.
- Private network MVP memakai relay lewat frps. Kontraknya dipisah dari
  implementasi frp agar WireGuard dapat ditambahkan tanpa mengubah orchestrator.
- Installer mengunduh binary `frpc` versi yang dipatok dan memvalidasi SHA-256.
  Unit test memakai binary/controller palsu dan tidak bergantung internet.

## Non-goals

- Self-healing policy lengkap, retry limit dan alerting kompleks (Fase 3).
- Load balancing lintas node.
- Enkripsi payload aplikasi di atas tunnel.
- WireGuard mesh native pada fase ini.

## Komponen

### Tunnel manager

Paket `internal/tunnel` memiliki interface engine-agnostic:

```go
type PortMapping struct {
    ServerID  string
    LocalPort int
    Protocol  string // tcp atau udp
}

type TunnelStatus string

const (
    TunnelPending      TunnelStatus = "pending"
    TunnelConnected    TunnelStatus = "connected"
    TunnelDisconnected TunnelStatus = "disconnected"
    TunnelError        TunnelStatus = "error"
)

type Assignment struct {
    TunnelID   string
    RemotePort int
    Status     TunnelStatus
}

type Manager interface {
    RegisterPort(context.Context, PortMapping) (Assignment, error)
    ReleasePort(context.Context, string, int) error
    Status(context.Context, string) ([]TunnelStatus, error)
    Reload(context.Context) error
}
```

Implementasi `FrpManager` bergantung pada tiga boundary terpisah:

- `PortAllocator`: client RPC Panel untuk assignment remote port.
- `TunnelStore`: persistence mapping dan status.
- `ProcessController`: start, stop, signal, dan observe child `frpc`.

Pemisahan ini memastikan generator config, lifecycle, dan reconnect bisa diuji
tanpa executable frp nyata.

### Konfigurasi

Konfigurasi runtime ditambah:

```yaml
runtime:
  frp_binary: "/usr/local/bin/frpc"
  frp_config: "/var/lib/oikos/frpc.toml"
  frp_server_addr: "frps.example.com"
  frp_server_port: 7000
  frp_token: ""
  frp_remote_port_min: 30000
  frp_remote_port_max: 40000
```

`frp_token` tidak boleh ditulis ke log. File config frpc permission `0600` dan
ditulis atomik menggunakan temporary file pada direktori yang sama lalu rename.
Installer tidak mengganti binary existing bila checksum gagal.

### Panel RPC

`proto/tunnel/tunnel.proto` menyediakan RPC berikut pada koneksi mTLS agent:

- `RegisterTunnel`: menerima `server_id`, local port, protocol; mengembalikan
  `tunnel_id`, `remote_port`, status, dan endpoint frps.
- `ReleaseTunnel`: melepas satu mapping dengan idempotensi.
- `GetTunnelStatus`: query mapping untuk server/node.
- `AssignNetworkGroup`: memberi node assignment group relay dan private IP.
- `ListNetworkGroupMembers`: discovery member group dan private IP.

Assignment harus idempoten berdasarkan `(server_id, local_port, protocol)`.
Panel harus menolak remote port yang sudah digunakan dan mengalokasikan hanya
dalam range yang disepakati.

### SQLite

Migrasi `0002_networking.sql` menambah:

```sql
CREATE TABLE tunnels (
    id              TEXT PRIMARY KEY,
    server_id       TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    local_port      INTEGER NOT NULL,
    remote_port     INTEGER,
    protocol        TEXT NOT NULL DEFAULT 'tcp',
    status          TEXT NOT NULL DEFAULT 'pending',
    last_connected  DATETIME,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_id, local_port, protocol),
    UNIQUE(remote_port)
);

CREATE TABLE network_groups (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE network_group_members (
    group_id        TEXT NOT NULL REFERENCES network_groups(id) ON DELETE CASCADE,
    node_id         TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    private_ip      TEXT NOT NULL,
    PRIMARY KEY (group_id, node_id),
    UNIQUE(group_id, private_ip)
);
```

`remote_port` boleh null saat status `pending`; SQLite uniqueness terhadap null
tetap memungkinkan beberapa mapping pending, sedangkan assigned port wajib
unik.

## Alur operasi

```text
orchestrator.CreateServer
  -> tunnel.RegisterPort
  -> Panel RegisterTunnel melalui mTLS
  -> simpan assignment ke SQLite
  -> generate frpc.toml secara atomik
  -> start/reload satu child frpc
  -> publish tunnel.connected atau tunnel.disconnected

orchestrator.DeleteServer
  -> tunnel.ReleasePort untuk setiap mapping server
  -> hapus mapping SQLite
  -> generate ulang config
  -> reload frpc
```

Jika assignment Panel gagal, mapping baru tidak direload sebagai proxy aktif.
Jika frpc gagal start/reload, assignment tetap tersimpan dengan status `error`
dan event disconnected dipublish. Child yang mati diamati manager dan dicoba
lagi dengan backoff `1s, 2s, 4s ... 30s`; retry policy rinci tetap Fase 3.

`ReleasePort` aman dipanggil berulang kali. Semua goroutine manager berhenti
saat context dibatalkan.

## Private network relay

Fase ini memakai frps sebagai relay. Panel memberi `NetworkGroupAssignment`
berisi group ID, node member, private IP relay, dan metadata endpoint yang
diperlukan. `NetworkManager` tidak mengekspos detail konfigurasi frp kepada
orchestrator; implementasi WireGuard dapat menggantikan relay di fase berikutnya.

Trade-off: relay menyederhanakan provisioning dan bekerja di balik NAT, tetapi
menjadikan frps dependency bandwidth dan single point of failure. HA frps
didokumentasikan sebagai operasi lanjutan, bukan blocker MVP.

## Provisioning frpc

Installer mengunduh binary versi frp yang dipatok untuk Linux amd64, memvalidasi
SHA-256 sebelum instalasi, menyimpan binary sebagai executable non-writable oleh
user daemon, dan gagal tanpa menghapus binary lama jika checksum salah. URL,
versi, dan checksum berasal dari release manifest atau environment installer.
Test lokal dapat mengatur URL `file://` dan checksum fixture.

## Testing

- Unit: validasi protocol, local port, remote port range, config TOML, escaping
  nilai, atomic write, status transition, dan idempotensi store.
- Component: fake Panel RPC dan fake process controller memastikan register,
  release, reload, event, serta reconnect berinteraksi benar.
- Integration opsional: frps/frpc nyata pada Docker network terisolasi untuk
  expose port, delete/release, SIGKILL frpc, dan reconnect.
- Test tanpa frp nyata wajib tetap berjalan pada host development biasa.

## Observability dan error handling

Log status minimal memakai level dan field `server_id`, `tunnel_id`, status,
dan attempt; token tidak pernah dicetak. Event `tunnel.connected` dan
`tunnel.disconnected` dikirim melalui event bus Fase 1. Metrics Prometheus,
event history, dan alerting eksternal tetap tanggung jawab Fase 3.

## Definition of Done Fase 2

- Node NAT dapat mengekspos port melalui frps tanpa konfigurasi manual user.
- Delete server melepaskan mapping dan port tidak lagi dapat diakses.
- frpc yang dihentikan paksa tersambung ulang dalam waktu kurang dari 30 detik.
- Dua node pada network group dapat memakai relay private network.
- Status tunnel dapat di-query melalui RPC Panel.
- Reload config tidak memutus tunnel existing yang masih valid.
- Unit dan component test lulus; integration test nyata lulus bila frps/frpc
  tersedia.

## Self-review

- Placeholder dan keputusan kosong tidak ada; deployment frps, port assignment,
  relay network, provisioning, schema, interface, failure policy, dan testing
  dipatok eksplisit.
- Scope tetap satu fase: tunnel publik + relay private network; self-healing
  matang dan observability penuh tidak masuk.
- `Manager` mengembalikan `Assignment`, sehingga implementasi berikutnya wajib
  memakai signature yang konsisten di orchestrator, fake, dan frp manager.
- `UNIQUE(remote_port)` sesuai kebutuhan port conflict; mapping pending tetap
  boleh karena null tidak bentrok di SQLite.
