# Oikos Final Release Readiness

Status: **production repository prepared; external release verification pending**  
Review date: 2026-09-13

## Repository Cleanup

The production repository excludes development-only artifacts:

- Test files and integration test fixtures.
- Mock/example plugin source.
- Benchmark, chaos, scanner, and documentation helper scripts.
- Historical planning/specification workspace.
- The original full work-plan document.
- Local binaries, scanner artifacts, worktrees, and opencode configuration.

The exclusion is represented in `.gitignore` for local files. Files that were
removed from Git remain available in repository history but are not part of the
production checkout.

## Completed Across Fases 1-6

### Fase 1: MVP Core

- Docker runtime abstraction and lifecycle operations.
- SQLite persistence and numbered migrations.
- Egg loader, validation, and startup variable rendering.
- CPU, memory, and PID resource limits.
- Pairing token, CSR, certificate storage, and mTLS agent connection.
- Heartbeat, command stream, local gRPC API, and installer foundation.

### Fase 2: Networking

- frp tunnel manager interface and TOML generation.
- Panel port allocator and tunnel persistence.
- Child-process lifecycle and reconnect policy.
- Relay network-group metadata.
- frpc checksum installer and separate frps deployment documentation.

### Fase 3: Reliability and Observability

- Structured JSON logger with secret redaction.
- Prometheus metrics and localhost HTTP endpoints.
- Health checks for runtime dependencies.
- Event history and retention pruning.
- Bounded container/tunnel self-healing.
- Crash observer and observability gRPC adapter.

### Fase 4: Filesystem and Data

- Per-server filesystem roots and centralized path resolution.
- Traversal and symlink rejection.
- Streaming file API and gRPC upload/download.
- Jailed SFTP with public-key credentials.
- Streaming tar.gz backup and SHA-256 validation.
- Staged restore with running-server protection.

### Fase 5: Extensibility and Portability

- Plugin subprocess host over Unix socket gRPC.
- Capability and route namespace validation.
- Plugin crash isolation and event forwarding.
- Versioned update slots, checksum/signature checks, and rollback state.
- Portable platform interfaces and non-Linux fallbacks.
- `oikos doctor` dependency diagnostics.

### Fase 6: Production Hardening

- Gosec high-severity gate integrated into CI with zero high findings at review.
- Govulncheck policy that accepts only reviewed reachable IDs and fails on new IDs.
- Filesystem security attack matrix.
- Seccomp profile and explicit LSM/user-namespace fallback reporting.
- Benchmark and chaos-test methodology.
- Production installation, operations, recovery, security, egg, and release docs.
- Keep a Changelog and release checklist.

## Verified Evidence

- Production source builds with `go build ./...`.
- `go vet ./...` passes on the production checkout.
- Source comments in production `.go`, `.proto`, YAML, and Dockerfile files use
  English comments without logic or identifier changes.
- Final idle baseline: RSS average **24.19 MB**, CPU average **0.05%**, RSS max
  **24.55 MB**, CPU max **0.40%**, measured over 12 samples at 5-second
  intervals with no active server containers.
- Gosec high gate: **0 findings**.
- Govulncheck policy test: **PASS**; reviewed reachable findings are the three
  documented Docker/Moby IDs.
- Docker runtime, cgroup, crash recovery, plugin, update, filesystem, backup,
  and SFTP tests passed before development tests were removed from the
  production checkout.

## Pending Manual Verification

These items require external infrastructure, production credentials, or a real
operator. They are not represented as passed sandbox evidence:

- Fase 1: clean VM install and production Panel pairing.
- Fase 2: real frps/frpc NAT exposure, public port release, and relay between
  real nodes.
- Fase 3: long-running production workload and external Prometheus scrape.
- Fase 4: 1GB+ upload/download profiling, hot backup under write pressure, and
  SFTP verification with FileZilla/WinSCP or OpenSSH client on a real host.
- Fase 5: real systemd daemon handoff/reboot recovery, signed production key,
  and runtime execution on native Darwin/Windows hosts.
- Fase 6: Oikos versus Wings head-to-head benchmark on equivalent VMs.
- Fase 6: load test with 10, 50, and 100 active servers.
- Fase 6: penetration test against gRPC and SFTP.
- Fase 6: AppArmor/SELinux enforcing profile validation on every target distro.
- Fase 6: user namespace remapping validation on the target Docker daemon.
- Fase 6: clean VM install by an independent operator.
- Fase 6: production `v1.0.0` release signing, checksums, tag, and rollback drill.
- Govulncheck Docker/Moby dependency advisories with no upstream fixed version:
  `GO-2026-6253`, `GO-2026-4887`, `GO-2026-4883`, plus module-only findings
  documented in `docs/security.md`. Review again on **2026-10-13**.

## Release Decision

The repository is cleaned and the automatically verifiable implementation is
documented. A production release should proceed only after the pending manual
verification list is reviewed by the release owner and the required external
evidence is attached to the release record.
