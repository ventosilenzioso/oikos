# Task 5 Report

## Status

Implemented versioned update slots and release verification in `internal/update`.

## Changes

- Added `Release` with version, binary URL, SHA-256, signature, GOOS, and GOARCH fields.
- Added `ReleaseVerifier` and `HealthRunner` interfaces.
- Added `UpdateManager` with `Check`, `Apply`, and `Rollback`.
- Downloads releases into temporary files and compares the exact lowercase SHA-256 digest.
- Invokes the signature verifier interface after checksum verification.
- Installs candidate binaries with executable permissions.
- Switches `current` and `previous` through temporary symlinks and rename operations.
- Preserves the existing known-good slot on checksum, signature, backup, candidate-health, or switch errors.
- Restores the known-good slot after a post-switch health failure.
- Never deletes the previous slot automatically. Existing versioned slots remain available for rollback.
- Supports a backup callback before installation for config/database/manifest backup.
- Does not stop or restart containers.

## Tests

- Checksum mismatch leaves `current` unchanged.
- Valid candidates switch `current`, record `previous`, and rollback restores the prior slot.
- Backup callback is invoked once before a successful update.
- Candidate health failure preserves the known-good slot.
- Post-switch health failure restores the known-good slot and retains the candidate slot.
- Signature verification is invoked and the installed candidate has mode `0755`.
- Tests use temporary directories and local HTTP test servers.

## Verification

- `go test ./internal/update/ -v` PASS
- `go test ./...` PASS
- `gofmt -w internal/update/*.go` applied
- `git diff --check` PASS

## Concerns

- The `HealthRunner` contract is intentionally narrow: it receives the candidate path before switching and the `current` symlink path after switching. Container lifecycle management remains outside this package, so updates never stop containers here.
- Signature semantics are delegated to the injected verifier; this package defines the interface but does not prescribe a cryptographic algorithm or key store.
