# Changelog

All notable changes to Oikos are documented here.

## [Unreleased]

### Added

- Production hardening checks, security reports, benchmark harness, and chaos harness.
- Seccomp/LSM/user namespace capability boundaries.
- Production operations, recovery, security, and egg documentation.

### Changed

- Docker stats conversions now use bounded integer conversions.
- CI runs static security and vulnerability analysis.

### Fixed

- Filesystem path traversal and symlink attack coverage expanded.
- Failed update candidate cleanup and rollback state recovery hardened.

### Security

- Gosec high-severity gate reports zero issues on the audited tree.
- Docker/Moby vulnerabilities without upstream fixes remain documented residual risk.

Production v1.0.0 release, binary signing, and clean VM validation remain pending manual verification.
