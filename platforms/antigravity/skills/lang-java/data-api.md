# Java: data, API, observability, operations, docs

Digest of `specs/languages/java/RULES.md` DATA, API, OBS, OPS, DOC.

## DATA
- Pick access deliberately (`JAVA-DATA-01`): JPA/Hibernate for aggregate CRUD; jOOQ, `JdbcClient` or Spring Data JDBC for reporting and bulk; never lazy JPA entities as the domain model across layers.
- Schema with Flyway (`V<n>__desc.sql`, immutable once merged) or Liquibase; `spring.jpa.hibernate.ddl-auto=validate` or `none`, never `update` outside tests (`JAVA-DATA-02`).
- `@Transactional` on application use-case methods only; short, no remote calls inside; proxy-based, so self-invocation and `private`/`final` methods bypass it; only unchecked exceptions roll back by default (`JAVA-DATA-03`).
- N+1 (`JAVA-DATA-04`): associations `LAZY` (`@ManyToOne` defaults to EAGER), fetch with `JOIN FETCH`, `@EntityGraph`, DTO projections or `default_batch_fetch_size`; assert query counts in tests.
- `spring.jpa.open-in-view=false` (`JAVA-DATA-05`).
- Entity `equals`/`hashCode`: business key, or id with a constant `hashCode` and null-id-unequal `equals`; no lazy associations or generated ids in `hashCode` (`JAVA-DATA-06`).
- HikariCP with explicit `maximumPoolSize`, `connectionTimeout`, `maxLifetime` below the DB timeout, JDBC timeouts (`JAVA-DATA-07`).

## API
- Spring MVC is the default (works with virtual threads); WebFlux only for reactive backpressure; controllers are thin inbound adapters (`JAVA-API-01`).
- OpenAPI-first or springdoc; commit the spec and diff it in CI; version without breaking clients (`JAVA-API-02`).
- RFC 9457 problem details via `ProblemDetail` and a `@RestControllerAdvice`; no stack traces or SQL in responses (`JAVA-API-03`).
- Timeouts on every outbound call; retry idempotent operations with backoff and jitter; Resilience4j; `Idempotency-Key` on non-idempotent POSTs (`JAVA-API-04`).
- Messaging: transactional outbox, idempotent consumers, dead-letter topics (`JAVA-API-05`). One shared HTTP client with pool and timeouts (`RestClient`, `WebClient`, `java.net.http.HttpClient`) (`JAVA-API-06`).

## OBS
- SLF4J + Logback or Log4j2, structured JSON, parameterised messages, MDC for `traceId`/`requestId`, no secrets or PII, no `System.out` (`JAVA-OBS-01`).
- Micrometer metrics with low-cardinality tags (`JAVA-OBS-02`); OpenTelemetry tracing with W3C `traceparent` copied into logs (`JAVA-OBS-03`).
- Health probes: liveness must not depend on the database; readiness includes dependencies (`JAVA-OBS-04`). GC logging, JFR, JVM binders, alerts (`JAVA-OBS-05`).

## OPS
- Typed `@ConfigurationProperties` records, 12-factor, env overrides (`JAVA-OPS-01`).
- Layered jar, buildpacks or Jib or a multi-stage image with `eclipse-temurin:25-jre`, non-root (`JAVA-OPS-02`). Native image only for CLIs and scale-to-zero, knowing the trade-offs (`JAVA-OPS-03`).
- Graceful shutdown (`server.shutdown=graceful`, SIGTERM, readiness down first, JVM as PID 1) (`JAVA-OPS-04`).
- Per-run caches: scope `target/`, `build/`, `java.io.tmpdir`, the JaCoCo exec (both `destFile` and `dataFile`) and Gradle `--project-cache-dir`; share `~/.m2` and `GRADLE_USER_HOME` (`JAVA-OPS-05`). The pack's `cache_env`, `shared_env` and `cache_argv` encode this.
- JVM flags in containers: `-XX:MaxRAMPercentage=75`, `-XX:+ExitOnOutOfMemoryError` (`JAVA-OPS-07`).

## DOC
- Javadoc on every public API of a library, built with `-Xdoclint:all` (`JAVA-DOC-01`); README with the one-command build and ADRs in `docs/adr/` (`JAVA-DOC-02`).
- Review focus (`JAVA-DOC-03`): transaction boundaries, N+1, null and `Optional`, exceptions, singleton thread safety, injection, `equals`/`hashCode`, resource closing, Jackson exposure of entities. Module docs via Spring Modulith `Documenter` (`JAVA-DOC-04`).
