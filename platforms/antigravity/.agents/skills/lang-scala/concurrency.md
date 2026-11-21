# Scala concurrency digest

Race rule: WORKFLOW §6 and the neutral wording in `specs/TESTING.md` when present. IDs from `specs/languages/scala/RULES.md`.

## Models (`SCALA-CONC-01`)
JVM threads (platform and virtual on JDK 21+), cats-effect fibers on a work-stealing `IORuntime`, ZIO fibers, Pekko actors. `Future` is eager and uncancellable: legacy only. Each model has its own cancellation and blocking rules; know which one the project uses (`SCALA-ARCH-04`).

## Shared state (`SCALA-CONC-02`)
Only through `Ref`, `AtomicCell`, `Semaphore`, `Queue`, `Deferred` (cats-effect), `Ref`, `Queue`, `Promise`, `STM`, `TRef` (ZIO), or messages (Pekko). Never mutate a `var` or `mutable.Map` from fibers. Never `get` then `set`: use `update` or `modify`.

## Why there is no detector (`SCALA-CONC-03`)
No JVM data-race detector exists; ThreadSanitizer does not apply. Static aids: `-Wnonunit-statement`, scalafix `DisableSyntax`. Dynamic aids: jcstress (lock-free code), Lincheck (linearizability), `TestControl` (a virtual-time mock runtime, not deterministic), ZIO `TestClock`.

## The substitute the gate enforces (`SCALA-CONC-04`)
1. The `test` check runs 3 times (`repeat: 3`): the whole argv, each time with `clean`.
2. Suites run in parallel. `parallelExecution` is true by default; with `Test / fork := true` the build needs `Test / testForkedParallel := true` (the spot run showed forked suites ran one after another otherwise).
3. Concurrent code has a concurrent test: `parTraverse` or `ZIO.foreachPar` with at least 100 iterations; `nonFlaky` for ZIO.
4. Time is virtual, never `sleep`.
The gate fails if the check cannot run; it never skips silently (D24). The policy is data in `pack.json` (`race.checks: ["test"]`).

## Async pitfalls (`SCALA-CONC-05`, `SCALA-IDIOM-10`)
- Never block a compute thread: wrap in `IO.blocking`, `ZIO.attemptBlocking`, or a dedicated pool. No `Await.result`.
- A timeout on every external call (`.timeout`); safe cancellation (`uncancelable`, `bracket`, `Resource`).
- Bound every queue; no unsupervised `.start` or `fork` (use `Supervisor`, `background`, `Scope`). Errors in forked fibers are silent, so join or log them.

## Streams (`SCALA-CONC-06`)
fs2, ZIO Streams or Pekko Streams with bounded buffers: `parEvalMap(n)`, `mapAsync(n)`; design for backpressure.

## Background work and shutdown (`SCALA-CONC-07`, `SCALA-OPS-04`)
Resource-managed worker fibers; `IOApp` or `ZIOAppDefault` cancels the main fiber on SIGTERM and runs finalizers. Drain the HTTP server, stop consumers before closing pools, release in reverse order of acquisition.

## Review checklist
Blocking calls in effects, unmanaged fibers, `Ref` read-modify-write, `var` captured by a fiber, sleeps in tests, `Test / fork := true` without `testForkedParallel`.
