# Swift: concurrency digest

Policy: WORKFLOW §6; pack `race.mode = flag`. Rules in RULES.md CONC.

## Model
async/await on the cooperative pool, structured concurrency (`async let`, `TaskGroup`), actors; SwiftNIO event loops underneath. `EventLoopFuture` is legacy (SWIFT-CONC-01).

## Data-race freedom
- Compile every target in Swift 6 mode. Shared mutable state is an `actor`, a `Synchronization.Mutex`, or `Sendable` values. `@unchecked Sendable` and `nonisolated(unsafe)` need a lock and a written reason (SWIFT-CONC-02).
- Run `swift test --sanitize=thread` in the gate: it catches what the compiler cannot (`@unchecked Sendable`, C interop, `Mutex` misuse). Expect 5-15x slowdown; scope it to unit and integration tests (SWIFT-CONC-03).
- Migrate per target: `swift package migrate`, `StrictConcurrency` before `.v6`. Never downgrade to `.v5` without a logged override (SWIFT-CONC-04).
- `@MainActor` is for UI. Never set `defaultIsolation(MainActor.self)` in a server target (SWIFT-CONC-05).

## Cancellation and blocking
- Cancellation is cooperative: `try Task.checkCancellation()`, structured tasks so a cancelled request cancels children, `withTaskCancellationHandler` for I/O; no `Task.detached` for request work (SWIFT-CONC-06).
- Do not block the pool: no `sleep()`, `DispatchSemaphore.wait`, sync DB or `URLSession` data calls in `async`; use `Task.sleep(for:)`, async clients, `NIOThreadPool` for CPU work (SWIFT-CONC-07).

## Bounds and lifecycle
- Sliding-window task groups, `AsyncStream` with `bufferingPolicy: .bufferingNewest(n)`, timeouts on every outbound call (SWIFT-CONC-08).
- Background work and shutdown through swift-service-lifecycle: one `ServiceGroup` with server, DB client, OTel and workers; handle SIGTERM and SIGINT (SWIFT-CONC-09, SWIFT-OPS-04).
- Swift Testing runs in parallel: mark shared-resource suites `@Suite(.serialized)`; never depend on test order (SWIFT-CONC-10).

## Review focus
Isolation correctness, every `@unchecked Sendable`/`nonisolated(unsafe)`, blocking calls in async, Task lifetime, `[weak self]` (SWIFT-DOC-03).
