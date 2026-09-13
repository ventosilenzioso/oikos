# Fase 6 Verification Report

Tanggal: 2026-09-13

## Automatic Verification

- `go test -count=1 ./...`: PASS.
- `go vet ./...`: PASS.
- `gofmt -l cmd internal gen`: clean.
- `make proto`: PASS.
- `make build`: PASS.
- Filesystem traversal/symlink attack matrix: PASS.
- Plugin/update race tests: PASS.
- Documentation consistency check: PASS.
- Gosec high-severity gate: PASS, `Issues: 0`.
- Govulncheck: no standard-library finding after Go 1.26.8 upgrade; Docker/Moby
  findings remain with no upstream fixed version and are documented in
  `docs/security-audit.md`.

## Sandbox Integration

- Docker runtime integration: PASS.
- cgroups v2 resource integration: PASS.
- Crash/recovery integration: PASS.
- Seccomp smoke: explicit SKIP because host runc rejects profile startup with
  `operation not permitted`; profile JSON and Docker option wiring are tested.
- AppArmor/SELinux: explicit fallback; target host reports unavailable.
- User namespace: capability/error behavior unit-tested; production enforcement
  requires target Docker daemon configuration.

## Cross-build

- `GOOS=linux go build ./...`: PASS.
- `GOOS=darwin go build ./...`: PASS.
- `GOOS=windows go build ./...`: PASS.
- Foreign-OS test binaries are not executed on Linux.

## Pending Manual Verification

- Head-to-head Oikos vs Wings benchmark.
- Production load test with 10, 50, and 100 active servers.
- Penetration test gRPC and SFTP.
- Clean VM installation by an independent operator.
- Real systemd graceful handoff and reboot recovery.
- Production signing key and signed `v1.0.0` release.
- AppArmor/SELinux enforcing profile on each target distro.
- User namespace remapping validation on target Docker daemon.

These items require external hosts, production credentials, or deployment
environment and are not represented as passing sandbox tests.
