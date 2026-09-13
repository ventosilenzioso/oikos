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

## Latest Reviewer Follow-up

- Reserved slot names `current`, `previous`, and `state.json` are rejected as release versions.
- `Apply` rejects a release whose version equals the authoritative current or previous slot before candidate removal, preserving active and rollback binaries and projections.
- Projection failure after manifest replacement now restores the prior authoritative manifest and repairs projections directly from that prior state before returning the error.
- Added injected projection-failure coverage and active/previous/reserved-name regression coverage.
- `state.json` remains authoritative; symlinks are compatibility projections and are repaired during projection-failure recovery.

## Latest Verification

- `go test ./internal/update/ -v` PASS
- `go test ./...` PASS
- `go vet ./...` PASS
- `gofmt -w internal/update/*.go` applied
- `git diff --check` PASS

## Concerns

- The `HealthRunner` contract is intentionally narrow: it receives the candidate path before switching and the `current` symlink path after switching. Container lifecycle management remains outside this package, so updates never stop containers here.
- Signature semantics are delegated to the injected verifier; this package defines the interface but does not prescribe a cryptographic algorithm or key store.

## Reviewer Follow-up

- Added strict release-version validation: only a single safe slot-name component is accepted; empty, dot, dot-dot, absolute, separator, and traversal names are rejected before any candidate removal or symlink projection.
- Added destructive-path regression coverage proving unsafe versions cannot delete an existing sentinel path.
- Added `state.json` as the authoritative current/previous pair. It is replaced through a temporary file and rename, while `current` and `previous` symlinks are compatibility projections repaired from the manifest. Apply and rollback load the manifest, so a missing projection recovers deterministically.
- Made release verification mandatory: a non-nil `ReleaseVerifier` and non-empty signature are required.
- SHA-256 input now requires exactly 64 lowercase hexadecimal characters. Uppercase and surrounding whitespace are rejected without normalization.
- Added verifier failure, backup failure, candidate health failure, and state consistency tests.

## Follow-up Verification

- `go test ./internal/update/ -v` PASS
- `go test ./...` PASS
- `go vet ./...` PASS
- `gofmt -w internal/update/*.go` applied
- `git diff --check` PASS
