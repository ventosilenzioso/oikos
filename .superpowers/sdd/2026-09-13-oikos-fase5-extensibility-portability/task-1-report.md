# Task 1 Implementation Report

## Status

Implemented Task 1 in the Fase 5 worktree.

## Files

- `proto/plugin/plugin.proto`
  - Added `PluginService` with `Init`, `HandleEvent`, `HealthCheck`, and `Close` RPCs.
  - Added init, capability, event, and health response messages.
- `gen/go/plugin/plugin.pb.go`
  - Generated protobuf message bindings.
- `gen/go/plugin/plugin_grpc.pb.go`
  - Generated `PluginServiceClient`, `PluginServiceServer`, registration, and RPC bindings.
- `scripts/gen-proto.sh`
  - Added `proto/plugin/plugin.proto` to the generation inputs.
- `migrations/0005_plugins.sql`
  - Added idempotent `plugins` table and enabled-plugin index.
- `internal/store/models.go`
  - Added the `Plugin` model with the required fields.
- `internal/store/sqlite.go`
  - Added idempotent `MigratePlugins` and wired it into the existing `Migrate` flow.
  - Added `SavePlugin`, `GetPlugin`, `ListPlugins`, and `SetPluginEnabled`.
  - Added SQLite JSON validation for `SubscribedEvents`.
- `internal/store/plugin_test.go`
  - Added registry migration/save/get/enable-toggle roundtrip coverage.

## Commit

- `6e15d41 feat: plugin proto dan registry`

## Verification

- `go test ./internal/store/ -run TestPluginRegistryRoundtrip -v`
  - PASS: `ok github.com/oikos/oikos/internal/store 0.037s`
- `go test ./internal/store/ -v`
  - PASS: all store tests, including `TestPluginRegistryRoundtrip`.
- `make proto`
  - PASS: generated plugin protobuf and gRPC bindings.
- `go test ./...`
  - PASS: all repository packages and tests.
- `gofmt -w internal/store/models.go internal/store/sqlite.go internal/store/plugin_test.go`
  - PASS.
- `git diff --check`
  - PASS with no whitespace errors.

## Concerns

- The task brief only supplied a roundtrip test. Additional direct tests for invalid JSON, list ordering, upsert behavior, and missing plugin IDs were not required and were not added.
- Generated code depends on the repository's existing protobuf and gRPC toolchain versions; it was generated successfully by `make proto`.
