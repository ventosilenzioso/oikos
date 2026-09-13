# Oikos

**A lightweight, security-first node daemon for running game and application servers with Docker.**

Oikos connects a Linux node to a control Panel, manages server containers,
enforces resource boundaries, and exposes an API-first operational surface.
It is designed for operators who want the control-plane experience of a server
hosting platform without coupling the node to one runtime or requiring shell
access for every lifecycle operation.

## Why Oikos

Oikos was created to address recurring node-level hosting problems:

- Keep the node daemon small and inexpensive while servers remain managed by
  Docker independently.
- Make first installation simple with one-time pairing and zero-config defaults.
- Keep Node-to-Panel communication protected by mTLS after pairing.
- Make server lifecycle, files, backups, resources, and diagnostics available
  through APIs instead of manual host operations.
- Keep container runtime policy behind an interface so Docker is not the only
  future option.
- Provide bounded recovery and clear health signals rather than silent failure.

Oikos is not a drop-in replacement for every Pterodactyl/Wings deployment. The
design prioritizes a smaller, explicit node surface and an extensible runtime
boundary. Benchmark claims against Wings remain pending until both systems are
measured on equivalent dedicated VMs.

## Features

- **Zero-config node pairing:** install with a one-time token, generate node
  credentials, and connect through mTLS.
- **Docker runtime abstraction:** create, start, stop, restart, delete, exec,
  logs, stats, and image build operations behind a runtime interface.
- **Resource management:** CPU, memory, and PID limits through Docker/cgroup v2
  support with platform capability reporting.
- **Self-healing:** bounded restart policies, crash detection, crash-loop stop,
  and tunnel reconnect handling.
- **Observability:** structured JSON logs, Prometheus metrics, `/healthz`, event
  history, and `oikos doctor` diagnostics.
- **Optional NAT networking:** frp-based tunnel management for nodes behind
  NAT/CGNAT. `frps` is deployed separately; NAT tunneling is optional.
- **Filesystem and data:** jailed file API, streaming upload/download, SFTP,
  hot `tar.gz` backup, checksum validation, and safe staged restore.
- **Plugin system:** isolated plugin subprocesses over gRPC Unix sockets,
  capability allowlists, event forwarding, and namespaced routes.
- **Graceful update foundation:** signed/checksummed versioned binary slots,
  rollback state, candidate health checks, and handoff logic. Real systemd
  handoff remains manual verification.

## Benchmark Status

| Scenario | Result |
|---|---:|
| Oikos idle, 0 active servers | 20.64 MB RSS average, 0.02% CPU average |
| Oikos idle target | Passed: `<30 MB`, `<1% CPU` |
| Oikos with 10 active servers | Pending manual verification |
| Oikos with 50 active servers | Pending manual verification |
| Oikos with 100 active servers | Pending manual verification |
| Oikos vs Wings head-to-head | Pending manual verification |
| Production load and penetration testing | Pending manual verification |

The idle result was measured on Linux amd64 with cgroup v2, a local mTLS mock
Panel, 12 samples at 5-second intervals after warm-up, and no active server
containers. See [`docs/benchmarks/idle-resource-usage.md`](docs/benchmarks/idle-resource-usage.md)
for raw values and limitations.

## Installation

### Prerequisites

- Linux amd64 node.
- Go is required only to build from source; release installs use the binary.
- Docker Engine with access to its Unix socket.
- A reachable Oikos Panel and one-time pairing token.
- `frpc` and a separate `frps` deployment only when NAT tunneling is needed.

### Install from a release

```bash
curl -fsSL https://install.oikos.io | sh
oikos install --token <one-time-token> --panel panel.example.com:9091
systemctl enable --now oikos
oikos doctor --config /etc/oikos/config.yaml
```

The installer validates the Oikos binary and, when configured, downloads a
pinned `frpc` binary with SHA-256 verification. Production hardening details are
in [`docs/installation.md`](docs/installation.md).

### Build from source

```bash
go build -o bin/oikos ./cmd/oikos
sudo install -m 0755 bin/oikos /usr/local/bin/oikos
```

Run `oikos doctor` after configuring the node. It reports Docker, cgroup, mTLS,
filesystem, plugin, and tunnel status explicitly.

## Basic Usage

Create a server from an installed egg:

```bash
oikos server create \
  --name minecraft-1 \
  --egg minecraft-vanilla \
  --startup 'java -Xms1024M -Xmx2048M -jar server.jar nogui'
```

Manage its lifecycle with the returned server ID:

```bash
oikos server start --id <server-id>
oikos server stop --id <server-id>
oikos server delete --id <server-id>
```

The local gRPC API exposes the same lifecycle operations. Server data is kept
under the configured filesystem root, and restore requires the server to be
stopped.

## Project Structure

```text
cmd/          CLI and operational commands
internal/     config, agent, runtime, orchestration, files, plugins, updates
proto/        gRPC contracts
gen/          generated protobuf Go code
migrations/   numbered SQLite migrations
eggs/         server egg definitions and Dockerfiles
deploy/       systemd, frp, and Docker deployment examples
docs/         operational and security documentation
```

See [`docs/architecture.md`](docs/architecture.md) for platform boundaries and
runtime architecture.

## Documentation

- [Installation](docs/installation.md)
- [Operations](docs/operations.md)
- [Architecture](docs/architecture.md)
- [Security model and vulnerability review](docs/security.md)
- [Security audit tooling](docs/security-audit.md)
- [Filesystem and backup](docs/filesystem.md)
- [Networking and frp](docs/networking.md)
- [Plugins](docs/plugins.md)
- [Graceful update](docs/update.md)
- [Disaster recovery](docs/disaster-recovery.md)
- [Egg specification](docs/egg-spec.md)
- [Release checklist](docs/release-checklist.md)

## Contributing

1. Create a focused branch for the change.
2. Keep production code and operational documentation consistent.
3. Add or update tests in a development checkout before cleanup/release export.
4. Run `go test ./...`, `go vet ./...`, `make proto`, and `make build`.
5. Run security scanners and document any dependency risk.
6. Explain manual verification requirements instead of inventing results.

## License

MIT License

Copyright (c) 2026 kevin.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
