# Kotlin data, API, observability, ops

Digest of RULES DATA, API, OBS, OPS. Language-neutral practice: `specs/ARCHITECTURE.md`, `skills/consistency`, `skills/jobs-and-state-machines`.

## Persistence (`KT-DATA-*`)
- Pick one per service, in the persistence adapter only: jOOQ (SQL-heavy), Exposed (Kotlin-first small), Spring Data JPA (entity graph), `JdbcClient`/R2DBC (thin) (`KT-DATA-01`).
- Flyway or Liquibase, forward-only; Hibernate `ddl-auto` is `validate` or `none` in prod; expand then contract (`KT-DATA-02`).
- Transactions at the use-case boundary through a `TransactionRunner` port; coroutines need `newSuspendedTransaction` or Spring's coroutine support; never across `delay` or a remote call (`KT-DATA-03`).
- JPA: no `data class` entities, `kotlin-jpa` and all-open, `open-in-view=false`, `JOIN FETCH` against N+1 (`KT-DATA-04`).
- HikariCP with explicit `maximumPoolSize`, `connectionTimeout`, `maxLifetime`; size for the blocking dispatcher (`KT-DATA-05`).

## API (`KT-API-*`)
- Thin controllers and routes: parse, call a use case, map the result (`KT-API-01`).
- OpenAPI-first (`openapi-generator`) or code-first; grpc-kotlin; additive-only versions (`KT-API-02`).
- Map sealed domain errors to RFC 9457 problem+json with exhaustive `when` (`KT-API-03`).
- Timeouts on every client; retry idempotent calls with backoff and jitter; circuit breakers; `Idempotency-Key` for POST retries (`KT-API-04`).
- Kafka: transactional outbox, idempotent consumers, offsets after processing, DLQ (`KT-API-05`).
- One shared HTTP client per upstream, closed on shutdown, cancellation and deadline propagated (`KT-API-06`).

## Observability (`KT-OBS-*`)
SLF4J with `kotlin-logging`, JSON output, correlation id in MDC and `MDCContext()` across coroutines (`KT-OBS-01`). Micrometer with low-cardinality tags (`KT-OBS-02`). OpenTelemetry agent or starter, context across coroutines (`KT-OBS-03`). Liveness must not check dependencies; readiness does (`KT-OBS-04`).

## Ops (`KT-OPS-*`)
- Typed config that fails fast (`@ConfigurationProperties` on a data class, `hoplite`); no config in the domain (`KT-OPS-01`).
- `bootJar`, `buildFatJar` or Jib, JRE-only non-root image (`KT-OPS-02`). Native image only when measured (`KT-OPS-03`).
- Graceful shutdown: Spring `server.shutdown=graceful`; handle SIGTERM, drain, cancel scopes, close pools (`KT-OPS-04`).
- Run caches (`KT-OPS-05`): share `GRADLE_USER_HOME`; the gate scopes `--project-cache-dir`. Leftovers cleaned: `build/`, `.gradle/`, `.kotlin/`, `*.hprof`, `*.jfr`, `hs_err_pid*.log` (`KT-OPS-06`).

## Docs and review (`KT-DOC-*`)
KDoc and Dokka (`dokkaGeneratePublicationHtml`); README with `./gradlew build`, JDK and config; ADRs; `@Deprecated(ReplaceWith)` and `apiCheck` for libraries.

## Performance (`KT-PERF-*`)
JMH via `kotlinx-benchmark`; async-profiler or JFR; `-XX:MaxRAMPercentage=75`; hoist `Regex`, use `buildString`, avoid `runBlocking` in request paths.
