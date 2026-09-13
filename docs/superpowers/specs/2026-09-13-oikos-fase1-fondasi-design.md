# Oikos Fase 1 — Desain Fondasi (MVP Core, scope fondasi)

Tanggal: 2026-09-13
Status: Disetujui user (lanjut implementasi)
Sumber: `oikos-rencana-kerja-lengkap.md` Fase 1 (goals #1, #4, #6, #8, #9, #10, #14, #18)

## 1. Klasifikasi & keputusan scope

- Path brainstorming: **Architectural** (greenfield, repo kosong selain dokumen rencana).
- Scope eksekusi yang disetujui: **Fondasi dulu** — bukan full 12 groups sekaligus.
  - Termasuk: toolchain real (Go + protoc + Docker), `go.mod`, struktur folder,
    `Makefile`, CI lint+test, `config`, `store` (SQLite + migrasi 0001),
    `egg` (struct/loader/validator/template), `runtime` interface + Docker real
    + FakeRuntime untuk test, `orchestrator` lifecycle+state+eventbus,
    `resource` dasar, CLI `cmd/oikos` dasar, proto file + `gen-proto.sh`,
    egg contoh `minecraft-vanilla`, `install.sh` + systemd unit dasar.
  - Menyusul setelah fondasi hijau: pairing/mTLS penuh, agent heartbeat/stream
    penuh, cgroups v2 enforcement penuh, installer end-to-end.
- Env awal: Ubuntu 24.04, Go/Docker/protoc belum ada → diinstal sebagai langkah 0.

## 2. Pendekatan (yang dipilih vs alternatif)

- **A — Dipilih: toolchain real + bottom-up.**
  Install Go 1.23 + protoc + Docker dulu, lalu bangun berurutan:
  struktur → store/config/egg (pure logic) → runtime Docker real →
  orchestrator + CLI. Alasan: sesuai permintaan user (real, bukan stub),
  fondasi teruji lapis demi lapis. Risiko: install Docker lama / daemon
  tak jalan di sandbox → mitigasi: test integrasi Docker di-skip otomatis
  bila daemon absen, unit test jalan dengan FakeRuntime.
- **B — Ditolak: vertical slice tipis 12 groups sekaligus.**
  Cepat compile semua package tapi penuh stub; risiko interface berubah
  dan DoD kabur. Tidak dipilih.
- **C — Fallback bila daemon Docker tak bisa jalan:**
  Docker SDK tetap di-compile real, tapi test pakai FakeRuntime +
  build tag `integration` untuk test Docker. Ini sub-rencana dari A,
  bukan pendekatan terpisah.

## 3. Keputusan teknis kunci

- Module: `github.com/oikos/oikos`, Go 1.23.
- SQLite driver: `modernc.org/sqlite` (pure Go, tanpa cgo, portable —
  sejalan dengan tujuan ringan/portable).
- gRPC: file `proto/node/node.proto`, `proto/server/server.proto`,
  `proto/stream/stream.proto` ditulis tangan; generate via
  `protoc + protoc-gen-go + protoc-gen-go-grpc` ke `gen/go/`.
  Kontrak Go (struct/interface) tidak ditulis manual — hasil generate.
- Runtime: interface `Runtime` persis dari dokumen rencana
  (`BuildImage/Create/Start/Stop/Restart/Delete/Status/Stats/Exec/Logs`).
  Implementasi `docker/docker.go` pakai `github.com/docker/docker/client`.
  `FakeRuntime` in-memory untuk unit test orchestrator.
- cgroups: deteksi v1/v2 saat startup; fondasi ini sediakan deteksi +
  struktur `resource`, enforcement penuh menyusul (tak janji lebih).
- Keamanan: permission file ketat (0600) untuk key/cert sejak awal;
  token sekali pakai + mTLS penuh di tahap lanjutan fondasi.

## 4. Struktur proyek (fondasi)

```
go.mod, Makefile, .golangci.yml
cmd/oikos/main.go
internal/{config,store,egg,runtime/{docker},resource,orchestrator,agent,security,api}
proto/{node,server,stream}/*.proto
gen/go/... (hasil generate, di-commit)
migrations/0001_init.sql (skema persis dokumen rencana)
scripts/gen-proto.sh, scripts/install.sh
deploy/systemd/oikos.service
eggs/minecraft-vanilla/{egg.yaml,Dockerfile}
config.yaml (contoh)
```

## 5. Aliran data (fondasi)

```
CLI (oikos server create/start/...) → orchestrator.lifecycle
  → store (SQLite: servers/eggs/resource_limits)
  → runtime (Docker real / Fake di test)
  → eventbus (channel-based, dipakai nanti oleh tunnel/selfheal)
```

## 6. Error handling & testing

- Error typed + `%w` wrap; operasi runtime idempotent bila memungkinkan.
- Test: `go test ./...`; unit wajib untuk `egg/validator` +
  `orchestrator/lifecycle` (pakai FakeRuntime + SQLite temp file).
  Test integrasi Docker dibatasi build tag dan skip bila daemon absen.
- CI: `make lint` (golangci) + `make test`; `make build`, `make proto`, `make run`.

## 7. Definition of Done (fondasi ini)

- [ ] Toolchain terinstal (go, protoc+plugin, docker client; daemon bila memungkinkan).
- [ ] `make build`, `make test`, `make proto` hijau.
- [ ] Unit test `egg/validator` + `orchestrator/lifecycle` lulus.
- [ ] CLI dasar create/start/stop/delete berjalan melawan FakeRuntime
      (dan Docker real bila daemon ada).
- [ ] Migrasi 0001 teraplikasi bersih di DB kosong.
- [ ] Struktur + dokumen ini di-commit.

## 8. Self-review spec

- Placeholder: tidak ada TBD; versi Go dipatok 1.23; skema SQL ikut dokumen rencana.
- Konsistensi: tidak janji mTLS/cgroups/installer selesai di fondasi —
  eksplisit tahap lanjutan. Tidak ada kontradiksi dengan rencana induk.
- Scope: satu siklus fondasi, bukan full Fase 1 — sesuai persetujuan.

> User: silakan review file ini; koreksi menyusul tanpa blokir —
> eksekusi fondasi jalan setelah ini via writing-plans.
