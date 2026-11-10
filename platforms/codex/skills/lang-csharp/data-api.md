# Data, API, observability, operations (CS-DATA, CS-API, CS-OBS, CS-OPS)

## Data (EF Core)
- **Split (CS-DATA-01):** EF Core for aggregates and writes; Dapper or `FromSql` for read-heavy reports. `DbContext` and `IEntityTypeConfiguration<T>` stay in Infrastructure.
- **Migrations (CS-DATA-02):** `dotnet ef migrations add`, review the SQL, deploy with an idempotent script or a bundle. Never `Database.Migrate()` at start in multi-instance production; expand/contract for zero downtime.
- **Unit of work (CS-DATA-03):** one `SaveChangesAsync` per use case; explicit transactions only across several saves or an outbox write; wrap retrying strategies with `CreateExecutionStrategy()`; `[Timestamp]` concurrency tokens.
- **Queries (CS-DATA-04):** `Include`/projection, `AsSplitQuery()`, no lazy-loading proxies, `AsNoTracking()` for reads; never `ToListAsync()` then filter in memory.
- **Lifetime (CS-DATA-05):** scoped `DbContext` (or pooled); never in a singleton or across threads; `IDbContextFactory<T>` in workers; async APIs with the request token.
- **Outbox (CS-DATA-06, CS-API-05):** publish events through an outbox and idempotent consumers keyed by message id.

## API
- **Style (CS-API-01):** minimal APIs for new services (`MapGroup`, endpoint filters, `TypedResults`), controllers where MVC features are needed. Handlers are thin and call an Application use case.
- **Contract (CS-API-02):** built-in OpenAPI (`AddOpenApi`/`MapOpenApi`), `Asp.Versioning.Http`, gRPC for service-to-service.
- **Errors (CS-API-03):** RFC 9457 problem details: `AddProblemDetails`, `UseExceptionHandler`, `IExceptionHandler` mapping domain errors; no stack traces in production.
- **Resilience (CS-API-04):** `AddStandardResilienceHandler()` on every typed client; retry only idempotent calls; `AddRateLimiter` on your own API.
- **HttpClient (CS-API-06):** `IHttpClientFactory` or a long-lived client with `PooledConnectionLifetime`; pass the token and a timeout.

## Observability
- Logging (CS-OBS-01): `ILogger<T>` with message templates (CA2254), `[LoggerMessage]` on hot paths, no secrets or personal data.
- Metrics and traces (CS-OBS-02/03): `System.Diagnostics.Metrics` and `ActivitySource`, exported through OpenTelemetry (OTLP).
- Health (CS-OBS-04): `/healthz/live` with no dependencies, `/healthz/ready` with tagged dependency checks; no detail publicly.
- Correlation (CS-OBS-05): trace id in problem details and log scopes; redact sensitive fields.

## Operations
- **Config (CS-OPS-01):** `AddOptions<T>().BindConfiguration(...).ValidateDataAnnotations().ValidateOnStart()`; layering `appsettings.json` < environment file < user-secrets < env vars (`Section__Key`) < CLI.
- **Packaging (CS-OPS-02/03/07):** `dotnet publish -t:PublishContainer` or a multi-stage Dockerfile (`sdk` build, `aspnet` runtime, non-root); NativeAOT only for trim-safe small services (`CreateSlimBuilder`; EF Core and MVC are not AOT-ready); `UseArtifactsOutput` for one output folder.
- **Shutdown (CS-OPS-04):** `ShutdownTimeout` below the orchestrator grace period; readiness false, then drain.
- **Run caches and artifacts (CS-OPS-05/06):** SKILL.md section 3.
