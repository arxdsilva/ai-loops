# Concurrency (CS-CONC)

Race policy and the three substitute checks: SKILL.md section 4 and CS-CONC-04 (WORKFLOW §6). Universal concurrency rules: `specs/ARCHITECTURE.md`.

## Async model
- `async`/`await` on the thread pool is the default (Tasks, `Channel<T>`, `IAsyncEnumerable<T>`); `Task.Run` only for CPU-bound work off a request thread, never to wrap I/O (CS-CONC-01).
- Never block on async: no `.Result`, `.Wait()`, `.GetAwaiter().GetResult()`, `Task.WaitAll`; no `async void` except event handlers; always await or return the Task (CS4014, CA2012). One `await` per `ValueTask`, used only on measured hot paths (CS-CONC-05, CS-CONC-09).
- `ConfigureAwait(false)` only in library code that may run under a `SynchronizationContext`; omit it in ASP.NET Core application code (CS-CONC-06).

## Shared state (CS-CONC-02)
Immutable types first; `Interlocked`; `lock` on a private `object` (`System.Threading.Lock` on .NET 9+); `SemaphoreSlim.WaitAsync` around async critical sections, never `lock` around `await`; `ConcurrentDictionary` (the `GetOrAdd` factory may run twice); `Channel<T>` for producer/consumer. Never lock on `this`, a `Type` or a string.

## Cancellation (CS-CONC-07)
`CancellationToken` is the last parameter of every async method and flows through the chain (`HttpContext.RequestAborted`, `ApplicationStopping`). Timeouts via `CancelAfter` or linked sources. Do not swallow `OperationCanceledException` (CA2016).

## Background work (CS-CONC-08, CS-OPS-04)
`BackgroundService`/`IHostedService`: honour `stoppingToken`, catch inside the loop (an unhandled exception stops the host), `HostOptions.ShutdownTimeout` below the orchestrator grace period. Durable jobs in a queue (Hangfire, Quartz, Wolverine). No fire-and-forget `Task.Run` in handlers.

## Proving it (CS-CONC-03)
No detector exists for managed code. Write stress tests: many iterations, `Parallel.For`, `Task.WhenAll`, tagged `Category=Concurrency`. Keep xUnit's parallel collections on. Microsoft Coyote is optional for systematic interleaving. Review checklist: shared mutable fields, `lock` and `await` together, captured `DbContext`, unbounded `Task.Run`.
