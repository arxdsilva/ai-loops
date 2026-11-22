# Java: concurrency

Digest of `specs/languages/java/RULES.md` CONC. The race policy itself is WORKFLOW §6 and the pack (`race` in `pack.json`).

## Model
- Platform threads via `ExecutorService`; virtual threads (final since 21) for blocking IO; reactive only where backpressure streaming is needed (`JAVA-CONC-01`). Spring Boot: `spring.threads.virtual.enabled=true`; one task per virtual thread, never pool them.
- Pinning (`JAVA-CONC-02`): since JDK 24 `synchronized` no longer pins carriers, but native frames and some `Object.wait` still do; use `ReentrantLock` for long sections, a `Semaphore` to bound downstream calls, no big `ThreadLocal` caches. Diagnose with `-Djdk.tracePinnedThreads=full`.
- Avoid preview APIs on an LTS (`JAVA-CONC-03`): `StructuredTaskScope` is preview (JEP 533 in 27, JEP 543 final in 28, due 2027-03), not final in JDK 25. Use `ExecutorService` plus `CompletableFuture`.
- `ScopedValue` is final in 25 and replaces `ThreadLocal` for per-request context; MDC and `RequestContextHolder` still use `ThreadLocal`, so propagate explicitly in async code (`JAVA-CONC-04`).

## Shared state
- Immutability first, then `java.util.concurrent` (`ConcurrentHashMap.compute/merge`, `AtomicReference`, `LongAdder`, `BlockingQueue`); locks with a documented hierarchy; safe publication via `final` or `volatile`; double-checked locking only with `volatile` (`JAVA-CONC-05`).
- Spring beans are singletons: no mutable instance state (`JAVA-DOC-03`, `JAVA-FW-05`).

## The `-race` equivalent
There is no flag (`JAVA-CONC-06`). The gate's `test` check is the policy: JUnit parallel execution + random method order, twice with different seeds. Add:
- jcstress for lock-free or memory-model-sensitive code, Lincheck for concurrent data structures (runs under JUnit);
- SpotBugs concurrency detectors and Error Prone `GuardedBy` checks; `@GuardedBy`/`@ThreadSafe` annotations.
No detector is complete: the honest policy is the substitute plus review. Never add test retries or a fixed order to make a flake go away.

## Async and background work
- Never block an event loop (WebFlux, Vert.x) with JDBC or `Thread.sleep`; a timeout on every remote call; propagate cancellation (`orTimeout`, interrupt); name threads; handle `CompletableFuture` failures (`exceptionally`, `handle`); bounded queues (`JAVA-CONC-07`).
- Executors are beans (`ThreadPoolTaskExecutor` with graceful shutdown); `@Scheduled` needs ShedLock or Quartz/JobRunr for clusters; jobs are idempotent; never `new Thread()` or an unbounded `newCachedThreadPool()` (`JAVA-CONC-08`). The domain must not start threads (`impure_calls`).
- Shutdown: see `data-api.md` (`JAVA-OPS-04`). Restore the interrupt flag when catching `InterruptedException` (`JAVA-IDIOM-02`).
