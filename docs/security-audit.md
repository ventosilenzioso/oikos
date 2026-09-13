# Security Audit Fase 6

## Tooling

- `gosec`: v2.22.8
- `govulncheck`: v1.1.4
- Go toolchain verification target: Go 1.26.8 or newer in CI

Run:

```bash
bash scripts/security/run-gosec.sh
bash scripts/security/run-govulncheck.sh
```

Reports are written to `artifacts/security/`, which must not contain secrets.

## Baseline 2026-09-13

- Gosec high integer-narrowing findings in Docker stats and tunnel port
  conversion were fixed with bounded conversion helpers and tests.
- The complete gosec report still contains medium/low findings for validated
  dynamic paths and subprocess boundaries. CI runs a separate `-severity high`
  gate; the complete report remains an audit artifact for review.
- Remaining gosec findings are medium/low warnings for validated dynamic paths,
  subprocess boundaries, generated protobuf code, file permission policy, and
  HTTP hardening. They are documented for follow-up rather than hidden.
- Govulncheck on the old Go 1.26.0 toolchain reported standard-library issues
  fixed by Go 1.26.3-1.26.6. The toolchain target is upgraded to Go 1.26.8.
- Docker SDK findings `GO-2026-5746`, `GO-2026-5668`, and `GO-2026-5617` have no
  upstream fixed version at audit time. Oikos does not use Docker archive copy
  APIs; Docker socket access remains a privileged deployment boundary. These
  findings stay pending dependency/vendor review and are not suppressed.
- `golang.org/x/crypto/openpgp` is not imported by Oikos; the module-level
  finding is not reachable from the application packages.

## Policy

CI fails on scanner command failure and unresolved high findings. Medium/low
findings require disposition in this document or a code fix before release.
