# Frameworks, performance, docs (CS-FW, CS-PERF, CS-DOC)

Follow the framework the project uses; project rules win and each override is logged (CS-FW-02).

## Frameworks
- **Stack (CS-FW-01):** ASP.NET Core (Web API, minimal APIs, MVC, SignalR, gRPC), EF Core (with Dapper beside it), the generic host and Worker Service, .NET Aspire for local orchestration, Azure Functions for serverless.
- **Fit (CS-FW-02):** `Program.cs` is the composition root; `appsettings` is configuration; `WebApplicationFactory` is the integration harness; `DbContext` already is a unit of work and repository. The default `Controllers/`, `Models/` folders belong only in the inbound adapter project.
- **Host model (CS-FW-04):** `WebApplication.CreateBuilder` for HTTP, `Host.CreateApplicationBuilder` for workers, `CreateSlimBuilder` for AOT; share telemetry, health and resilience in a `ServiceDefaults` project.
- **Footguns (CS-FW-03):** scoped service in a singleton (`ValidateScopes` catches it); `DbContext` across threads; middleware order (auth after routing, before endpoints); `IOptions` read once but expected to reload (use `IOptionsMonitor`); synchronous `Stream` reads in Kestrel (disallowed by default); minimal API binding ambiguity; licence changes (MediatR v13+ is dual RPL-1.5/commercial, FluentAssertions v8+ commercial, Moq telemetry) that affect CI.

## Performance (CS-PERF)
- Measure first (CS-PERF-01): BenchmarkDotNet in `benchmarks/`, `Release`, `[MemoryDiagnoser]`; never benchmark in Debug.
- Profile (CS-PERF-02): `dotnet-trace`, `dotnet-counters`, `dotnet-dump`/`dotnet-gcdump`.
- Runtime (CS-PERF-03): Server vs Workstation GC, container limits, ReadyToRun, `ArrayPool<T>`, `Span<T>` only on measured hot paths.
- Hot-path traps (CS-PERF-04): string concatenation in loops, boxing, `Regex` per call (`[GeneratedRegex]`), interpolated logging, LINQ in tight loops, exceptions as control flow, sync I/O.
- Caching (CS-PERF-05): `OutputCache`, `HybridCache` or `IMemoryCache` with expiry and size limits; no unbounded keys.

## Docs and review (CS-DOC)
- XML docs on public API with `GenerateDocumentationFile` (CS-DOC-01); README, ADRs, CHANGELOG, CONTRIBUTING naming the exact `dotnet` commands (CS-DOC-02).
- Review first for (CS-DOC-03): sync-over-async, missing `CancellationToken`, `DbContext` lifetime and N+1, `!` suppressions, `HttpClient` creation, swallowed exceptions, secrets in config, public API growth, untested branches.
- Libraries: `EnablePackageValidation` with a baseline version (CS-DOC-04).
