# Oikos Fase 6 Production Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Mengeraskan keamanan runtime Oikos dan menyediakan validasi production yang reproducible tanpa menambah kapabilitas produk baru.

**Architecture:** Task dikerjakan berurutan dari static analysis dan filesystem security menuju Docker sandbox options, capability fallback, benchmark/chaos harness, dokumentasi, dan release checklist. Setiap task independen, memiliki test/command verification, dan di-commit sebelum task berikutnya.

**Tech Stack:** Go 1.26, `gosec`, `govulncheck`, Docker SDK v28, cgroup v2, seccomp JSON, AppArmor/SELinux probes, shell/Python benchmark harness, Markdown/YAML.

**Spec:** `docs/superpowers/specs/2026-09-13-oikos-fase6-production-hardening-design.md`

## Global Constraints

- Fase ini murni hardening dan validasi fitur Fase 1-5; tidak menambah API produk baru.
- Tool static analysis dan dependency audit harus fail pada finding high unresolved.
- `ResolvePath` tetap single security boundary dan deny-by-default.
- Seccomp/AppArmor/SELinux/user namespace unsupported harus dilaporkan jelas, tidak diam-diam dianggap aktif.
- Benchmark Wings, load puluhan/ratusan server, penetration test nyata, signing key production, dan clean VM/systemd diberi status `pending manual verification` bila environment tidak tersedia.
- Setiap task di-commit setelah verification hijau.
- Jangan menggunakan secret production dalam test atau artifact.

---

## File Map

- Modify `.github/workflows/ci.yml`, `go.mod`, `go.sum`; create `tools.go` only if needed.
- Create `scripts/security/run-gosec.sh`, `scripts/security/run-govulncheck.sh`, `docs/security-audit.md`.
- Modify/create `internal/filesystem/*_test.go` for attack matrix.
- Create `internal/security/sandbox.go`, `sandbox_linux.go`, `sandbox_unsupported.go`, and tests.
- Modify `internal/runtime/runtime.go`, `internal/runtime/docker/docker.go` for sandbox options.
- Create `deploy/docker/seccomp-default.json` and AppArmor/SELinux capability docs.
- Create `internal/security/lsm.go` and tests.
- Create `internal/security/userns.go` and tests.
- Create `docs/benchmarks/README.md`, comparison/load reports, and `scripts/benchmarks/` harnesses.
- Create `scripts/chaos/` harness and `docs/chaos-testing.md`.
- Create/update production docs and `CHANGELOG.md`, release checklist.

---

### Task 1: Static analysis and dependency audit

**Files:**
- Create: `scripts/security/run-gosec.sh`, `scripts/security/run-govulncheck.sh`, `docs/security-audit.md`
- Modify: `.github/workflows/ci.yml`

**Interfaces:**
- Produces: reproducible `gosec`/`govulncheck` commands and CI jobs with explicit exit behavior.

- [ ] **Step 1: Check tool availability and write reproducible scripts**

Run: `command -v gosec || true; command -v govulncheck || true`.

Scripts must install pinned tool versions into `$(go env GOPATH)/bin` when absent, run scanners with `set -euo pipefail`, write reports under `artifacts/security/`, and return scanner exit status. Scanner output must not include environment dumps.

- [ ] **Step 2: Run scanners and capture baseline findings**

Run:

```bash
bash scripts/security/run-gosec.sh
bash scripts/security/run-govulncheck.sh
```

Record exact tool versions, finding IDs, severity, file/line, and disposition in `docs/security-audit.md`. Do not suppress a finding without a concrete reason.

- [ ] **Step 3: Fix high findings**

For each high finding, add a regression test before changing implementation, fix the smallest root cause, and rerun the affected scanner. If a finding is a verified false positive, add its ID and technical justification to the report and scanner config.

- [ ] **Step 4: Wire CI**

Add CI steps after test/vet:

```yaml
- run: bash scripts/security/run-gosec.sh
- run: bash scripts/security/run-govulncheck.sh
```

Upload `artifacts/security/` as CI artifacts without secrets.

- [ ] **Step 5: Verify and commit**

Run: `bash scripts/security/run-gosec.sh && bash scripts/security/run-govulncheck.sh && go test ./... && git diff --check`.

Commit: `git add .github/workflows/ci.yml scripts/security docs/security-audit.md && git commit -m "chore: add security static analysis"`.

---

### Task 2: Filesystem attack matrix

**Files:**
- Create: `internal/filesystem/security_audit_test.go`
- Modify: `internal/backup/manager_test.go` only for missing attack cases.

**Interfaces:** existing `filesystem.Manager` and backup restore helpers.

- [ ] **Step 1: Write attack tests**

Add table-driven tests for `../../etc/passwd`, absolute paths, backslash, sibling-prefix collision (`server-a`/`server-ab`), symlink file, symlink parent, rename destination escape, copy destination escape, and symlink/traversal tar entries. Each must assert `errors.Is(err, ErrPathOutsideRoot)` and assert no outside file changed.

- [ ] **Step 2: Run tests before implementation changes**

Run: `go test ./internal/filesystem/ ./internal/backup/ -run 'Security|Traversal|Symlink|Escape' -v`.

Expected: existing cases pass; any newly exposed weakness fails with the exact payload.

- [ ] **Step 3: Fix only confirmed weaknesses**

Keep `ResolvePath` as single boundary. Use `Lstat` component walk and `filepath.Rel` boundary check. Do not add unrelated filesystem features. Add a cancellation test for streaming write that verifies temporary files are removed.

- [ ] **Step 4: Verify and commit**

Run: `go test ./internal/filesystem/ ./internal/backup/ -v && go test -race ./internal/filesystem/ ./internal/backup/`.

Commit: `git add internal/filesystem internal/backup && git commit -m "test: harden filesystem attack boundary"`.

---

### Task 3: Seccomp default profile and Docker options

**Files:**
- Create: `deploy/docker/seccomp-default.json`, `internal/security/sandbox.go`, `internal/security/sandbox_linux.go`, `internal/security/sandbox_unsupported.go`, `internal/security/sandbox_test.go`
- Modify: `internal/runtime/runtime.go`, `internal/runtime/docker/docker.go`

**Interfaces:**
- Produces: `security.SandboxOptions{SeccompProfile, AppArmorProfile, SELinuxLabel string; UserNamespace bool; Permissive bool}`; `security.LoadDefaultSandboxOptions()`, `ValidateSandboxOptions()`.

- [ ] **Step 1: Write failing tests**

```go
func TestDefaultSeccompProfileIsValidJSON(t *testing.T) {
    data, err := os.ReadFile("../../deploy/docker/seccomp-default.json"); if err != nil { t.Fatal(err) }
    var profile map[string]any; if err := json.Unmarshal(data,&profile); err != nil { t.Fatal(err) }
    if profile["defaultAction"] != "SCMP_ACT_ERRNO" { t.Fatal(profile) }
}

func TestUnsupportedSandboxReturnsExplicitError(t *testing.T) {
    if _, err := LoadDefaultSandboxOptionsForPlatform("windows"); !errors.Is(err, ErrNotSupported) { t.Fatal(err) }
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/security/ -run Sandbox -v`.
Expected: FAIL because profile/options do not exist.

- [ ] **Step 3: Implement profile and Docker integration**

Profile must be valid JSON with explicit architecture and allowlist based on the existing egg/runtime needs. Docker create passes `SecurityOpt: []string{"seccomp=" + profilePath}` only when profile exists and `Permissive=false`. Missing profile returns `ErrSandboxUnavailable` in enforcing mode and warning-compatible result in explicit permissive mode. Do not invent syscall requirements without documenting them.

- [ ] **Step 4: Add Docker smoke integration**

Create a build-tagged test that runs `busybox:stable` with the profile, executes `echo`, checks success, and removes the container. Skip only if Docker unavailable.

- [ ] **Step 5: Verify and commit**

Run: `go test ./internal/security/ ./internal/runtime/... -v && go test -tags integration ./internal/runtime/docker/ -run Sandbox -v`.

Commit: `git add deploy/docker internal/security internal/runtime && git commit -m "feat: default seccomp sandbox"`.

---

### Task 4: AppArmor/SELinux detection and fallback

**Files:**
- Create: `internal/security/lsm.go`, `internal/security/lsm_test.go`, `deploy/docker/oikos-apparmor.profile`
- Modify: `cmd/oikos/doctor.go`, `docs/security-audit.md`

**Interfaces:** `DetectLSM() LSMStatus`, `LSMStatus{AppArmorAvailable, SELinuxAvailable bool; AppArmorProfile, SELinuxMode string}`, `ValidateLSMProfile(status, requested) error`.

- [ ] **Step 1: Write tests**

Test fixture-based detection for AppArmor marker present/absent and SELinux enforcing/permissive/disabled. Assert unsupported systems return explicit status rather than enabled=true.

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/security/ -run LSM -v`; expect FAIL.

- [ ] **Step 3: Implement detection and fallback**

Use injected filesystem reader/command runner for tests. Production checks securityfs and `getenforce` only when available. Doctor prints `[ok]`, `[warn]`, or `[FAIL]` with actionable remediation. Never load AppArmor/SELinux profile if the corresponding subsystem/profile is unavailable.

- [ ] **Step 4: Verify and commit**

Run: `go test ./internal/security/ -run LSM -v && go test ./cmd/oikos/ -run Doctor -v`.

Commit: `git add internal/security cmd/oikos deploy/docker docs/security-audit.md && git commit -m "feat: lsm capability detection fallback"`.

---

### Task 5: User namespace remapping

**Files:**
- Create: `internal/security/userns.go`, `internal/security/userns_test.go`
- Modify: `internal/runtime/docker/docker.go`, `cmd/oikos/doctor.go`, `docs/security-audit.md`

**Interfaces:** `UserNamespaceStatus{Supported,Enabled bool; Mode string}`, `DetectUserNamespace()`, `ApplyUserNamespace(spec, status, permissive) error`.

- [ ] **Step 1: Write tests**

Test supported/enabled config produces Docker `UsernsMode`, unsupported enforcing mode returns `ErrNotSupported`, and permissive mode returns warning status without silently claiming remapping.

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/security/ -run UserNamespace -v`; expect FAIL.

- [ ] **Step 3: Implement capability-gated Docker mapping**

Detect Docker daemon/userns-remap capability using injected probe. Apply only when enabled and supported. Keep default behavior unchanged unless config explicitly enables enforcement. Doctor reports exact mode.

- [ ] **Step 4: Verify and commit**

Run: `go test ./internal/security/ ./internal/runtime/docker/ -v && go test ./cmd/oikos/ -run Doctor -v`.

Commit: `git add internal/security internal/runtime cmd/oikos docs/security-audit.md && git commit -m "feat: user namespace capability guard"`.

---

### Task 6: Benchmark/load-test/chaos harness

**Files:**
- Create: `docs/benchmarks/README.md`, `docs/benchmarks/oikos-vs-wings-comparison.md`, `docs/benchmarks/load-test-100-servers.md`, `scripts/benchmarks/measure-daemon.sh`, `scripts/benchmarks/load-servers.sh`, `scripts/chaos/run-chaos.sh`, `docs/chaos-testing.md`

**Interfaces:** shell scripts accept explicit config/output paths and never embed production credentials.

- [ ] **Step 1: Write harness smoke tests**

Test scripts with `--help`, invalid argument, and temporary fake commands. Assert raw output directory and metadata file are created only in output path.

- [ ] **Step 2: Implement idle/load harness**

`measure-daemon.sh` records binary commit, Go/Docker/kernel versions, PID, RSS/CPU samples, and outputs JSON/Markdown. `load-servers.sh` accepts count `0|10|50|100`, starts fake/runtime-compatible workloads only when explicitly enabled, and exits nonzero on missing prerequisites.

- [ ] **Step 3: Implement chaos harness**

`run-chaos.sh` runs isolated scenarios for Docker unavailable, unwritable data dir, Panel unreachable, and SQLite lock. Each scenario records command, exit code, stdout/stderr, and expected status. No destructive host-level actions without explicit `OIKOS_CHAOS_ENABLE=1`.

- [ ] **Step 4: Write reports honestly**

Reports include methodology and tables with `pending manual verification` for Wings comparison, 10/50/100 production server results, and penetration tests until real environments provide numbers. Never use `TBD` as an unqualified claim; label the status and required environment.

- [ ] **Step 5: Verify and commit**

Run: `bash scripts/benchmarks/measure-daemon.sh --help; bash scripts/benchmarks/load-servers.sh --help; bash scripts/chaos/run-chaos.sh --help; shellcheck scripts/benchmarks/*.sh scripts/chaos/*.sh` when shellcheck is available.

Commit: `git add docs/benchmarks scripts/benchmarks scripts/chaos docs/chaos-testing.md && git commit -m "chore: add production validation harnesses"`.

---

### Task 7: Production documentation and release checklist

**Files:**
- Create/modify: `docs/architecture.md`, `docs/installation.md`, `docs/operations.md`, `docs/disaster-recovery.md`, `docs/security.md`, `docs/egg-spec.md`, `CHANGELOG.md`, `docs/release-checklist.md`

- [ ] **Step 1: Write documentation consistency check**

Add a script/test that verifies required docs exist and contains exact references to `/metrics`, `/healthz`, `oikos doctor`, `oikos update`, filesystem roots, plugin capability, sandbox fallback, and pending manual verification markers where evidence is unavailable.

- [ ] **Step 2: Write production docs**

Document actual commands/config from Fase 1-5: installation, Docker/frps optionality, mTLS, filesystem/backup/restore, plugin host, update slots/rollback, observability, security limitations, disaster recovery, and egg schema. Do not document unsupported features as active.

- [ ] **Step 3: Add release metadata**

Use Keep a Changelog sections `Added`, `Changed`, `Fixed`, `Security`; document semantic versioning, reproducible build/checksum/signing workflow, support policy, rollback, and explicit manual verification list.

- [ ] **Step 4: Verify and commit**

Run: documentation consistency script, `go test ./...`, and `go vet ./...`.

Commit: `git add docs CHANGELOG.md && git commit -m "docs: production hardening and release checklist"`.

---

### Task 8: Final Fase 6 verification

**Files:** `docs/fase6-verification.md`

- [ ] **Step 1: Run automatic verification**

```bash
make proto && make build
go test -count=1 ./...
go vet ./...
test -z "$(gofmt -l cmd internal gen)"
bash scripts/security/run-gosec.sh
bash scripts/security/run-govulncheck.sh
```

- [ ] **Step 2: Run integration verification available in sandbox**

Run seccomp Docker smoke, cgroup/resource tests, filesystem security tests,
plugin/update integration, and chaos harness safe scenarios. Record exact PASS,
FAIL, or SKIP reason.

- [ ] **Step 3: Cross-build**

Run `GOOS=linux go build ./...`, `GOOS=darwin go build ./...`, and
`GOOS=windows go build ./...`. Do not run foreign test binaries on Linux.

- [ ] **Step 4: Record manual verification**

Explicitly list as `pending manual verification`: Wings head-to-head, production
load 10/50/100, penetration test gRPC/SFTP, clean VM install, systemd chaos,
production signing key, and AppArmor/SELinux enforcement on target distro if not
available in this host.

- [ ] **Step 5: Commit verification report**

```bash
git commit -m "chore: verify fase 6 production hardening"
```

## Self-Review

- Coverage: static audit (Task 1), filesystem attack audit (2), seccomp (3), LSM fallback (4), user namespace (5), benchmark/chaos (6), docs/release (7), final verification/manual list (8).
- No production-only benchmark or signature result is fabricated.
- Security options are capability-gated and observable.
- Existing Fase 1-5 API contracts remain unchanged unless an additive security option is required.
