# Task 4 Implementation Report

## Status

Implemented plugin registry lifecycle, EventBus bridging with cancellation, event timeouts, failure-based disabling, and namespace-scoped HTTP route proxying.

## Files

- `internal/plugin/registry.go`
  - Added `Registry` with `LoadEnabled`, `StartAll`, `StopAll`, `DispatchEvent`, and `HealthCheckAll`.
  - Loads enabled plugin rows from SQLite and constructs hosts from persisted plugin metadata.
  - Subscribes only to each plugin's declared events.
  - Uses a per-dispatch timeout and disables a plugin after three dispatch failures, persisting the disabled state when SQLite is available.
  - Cancels EventBus subscriptions and terminates bridge receive loops during `StopAll`.
- `internal/plugin/proxy.go`
  - Added `Proxy.Register` and `Proxy.Handler`.
  - Accepts only `/plugins/<id>/...` routes, rejects traversal/global routes, rejects duplicate routes, and returns 404 for unregistered paths.
- `internal/plugin/registry_test.go`
  - Added tests for allowed versus disabled event delivery, bridge cancellation, three-failure disable behavior, successful plugin route dispatch, and rejection of `/admin`.
- `internal/orchestrator/eventbus.go`
  - Preserved existing `Subscribe`, `SubscribeWithCancel`, `Publish`, and constructor behavior.
  - Added `SubscribeWithCancelDone` for consumers that need a cancellation signal to stop their receive loop.

## TDD Evidence

- Initial focused test run failed because `Registry`, `Proxy`, and their constructors were undefined.
- Implemented the minimum registry and proxy behavior required by the failing tests.
- Focused registry/proxy tests passed after implementation.

## Commit

- `feat: plugin event bridge dan route proxy`

## Verification

- `gofmt -w internal/plugin/registry.go internal/plugin/proxy.go internal/plugin/registry_test.go internal/orchestrator/eventbus.go`
  - PASS.
- `go test ./internal/plugin/ ./internal/orchestrator/ ./internal/api/ -v`
  - PASS.
- `go test ./...`
  - PASS.
- `git diff --check`
  - PASS with no whitespace errors.

## Concerns

- `LoadEnabled` reconstructs the persisted manifest from the plugin table; the current SQLite schema stores subscribed events but does not persist allowed routes or a separate manifest path, so route capability registration remains an explicit integration concern for callers.
- `HealthCheckAll` returns per-plugin errors rather than aggregating them because the brief specifies the method but not an aggregate error contract.

## Reviewer Fix Report

### Findings addressed

1. Persisted plugin manifests now include config path, SHA-256, subscribed events, and allowed routes. The plugin migration adds these columns with defaults, upgrades existing rows idempotently, disables legacy enabled rows that lack a safe checksum, and preserves existing data.
2. `LoadEnabled` reconstructs the complete validated manifest. Negotiated route handlers are registered with the registry proxy during `StartAll`, including after a restart.
3. `Proxy` protects route registration and request lookup with an RWMutex, making concurrent `Register` and `ServeHTTP` safe.
4. Registry bridge and dispatch wait groups ensure `StopAll` cancels subscriptions and waits for bridge goroutines and in-flight event handling before stopping hosts.
5. Event dispatch now passes a context deadline directly through the host adapter into the gRPC call. The registry no longer starts an uncancellable wrapper goroutine around `HandleEvent`.
6. Tests cover persisted manifest loading, idempotent legacy migration, automatic route registration, timeout context cancellation, persistent disable state, duplicate routes, traversal/global route rejection, and race-safe event assertions.

### Verification

- `go test ./internal/plugin/ ./internal/orchestrator/ ./internal/api/ -v`
  - PASS.
- `go test -race ./internal/plugin/`
  - PASS.
- `go test ./...`
  - PASS.
- `gofmt` and `git diff --check`
  - PASS.

### Fix commit

- Pending commit for this reviewer-fix set.
