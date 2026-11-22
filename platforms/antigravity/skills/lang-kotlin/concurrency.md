# Kotlin concurrency

Digest of RULES CONC. The gate's race policy for this pack is a substitute (SKILL.md "Race policy").

## Model (`KT-CONC-01`)
Coroutines on JVM threads, or Java 21 virtual threads for blocking code (`spring.threads.virtual.enabled=true`). Pick one per workload and measure.

## Scopes (`KT-CONC-02`)
- Never `GlobalScope`. Own a `CoroutineScope(SupervisorJob() + dispatcher)` per component and cancel it on shutdown.
- `coroutineScope {}` for all-or-nothing children; `supervisorScope {}` when siblings must survive a failure.

## Shared state (`KT-CONC-03`)
Prefer immutability and confinement: `limitedParallelism(1)`, actor or `Channel`, `StateFlow`. Otherwise `Mutex` (suspend-aware, non-reentrant) or `java.util.concurrent` atomics. `@Volatile` is visibility only. No suspension point inside `synchronized`.

## Detection (`KT-CONC-04`)
No built-in detector. Lincheck (`org.jetbrains.lincheck:lincheck:3.7`; `ModelCheckingOptions().check(...)`, `StressOptions()`, `Lincheck.runConcurrentTest`) for concurrent structures and critical sections; jcstress for JMM litmus tests; parallel JUnit and repeats for the rest. Lincheck checks linearizability, not every data race.

## Blocking and dispatchers (`KT-CONC-05`)
No JDBC, `Thread.sleep` or file IO on `Dispatchers.Default`: `withContext(Dispatchers.IO)` or `limitedParallelism(n)`. Inject the `CoroutineDispatcher` (default parameter). detekt: `SleepInsteadOfDelay`, `InjectDispatcher`, `GlobalCoroutineUsage`.

## Cancellation (`KT-CONC-06`)
Cooperative: long loops call `ensureActive()` or `yield()`. `withTimeout`/`withTimeoutOrNull` for deadlines; `withContext(NonCancellable)` for suspending cleanup in `finally`; always rethrow `CancellationException`.

## Flow (`KT-CONC-07`)
Cold `Flow` for streams, `StateFlow` for state, `SharedFlow(replay, extraBufferCapacity)` for events. Choose `buffer`/`conflate` explicitly; `flowOn` for upstream context, never `withContext` inside `flow {}`; `stateIn(scope, SharingStarted.WhileSubscribed(5000), init)`.

## Workers (`KT-CONC-08`)
A worker class owns its scope and exposes `close()` that `cancelAndJoin()`s, wired to JVM shutdown (`@PreDestroy`/`SmartLifecycle`, Ktor `ApplicationStopping`). Durable jobs: Quartz, JobRunr or db-scheduler.
