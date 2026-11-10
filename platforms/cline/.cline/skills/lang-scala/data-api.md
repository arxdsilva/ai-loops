# Scala data, API, observability and ops digest

IDs from `specs/languages/scala/RULES.md`. The project's stack wins (`frameworks.md`).

## DATA
- Access (`SCALA-DATA-01`): SQL-first. doobie (JDBC, checked SQL) or skunk (native Postgres, non-blocking); Slick, Quill, zio-jdbc where the project uses them. No heavy ORMs; Hibernate or JPA from Scala is an anti-pattern.
- Migrations (`SCALA-DATA-02`): Flyway, versioned `V<n>__name.sql`, forward-only, expand-contract; never auto-migrate on every replica without a lock.
- Transactions (`SCALA-DATA-03`): at the use-case boundary (doobie `.transact(xa)` on a whole `ConnectionIO`; ZIO `transaction`). Adapters do not open one per call. Retry on serialization failure.
- N+1 (`SCALA-DATA-04`): one query with `IN` or `unnest`, joins in Slick. doobie and skunk have no lazy loading.
- Pools (`SCALA-DATA-05`): HikariCP (`HikariTransactor`) or skunk `Session.pooled`; set `connectionTimeout` and `maxLifetime`; bound the connect and blocking pools.

## API
- Stack (`SCALA-API-01`): http4s plus Tapir (Typelevel), ZIO HTTP, Play, or Pekko HTTP. Routes live in inbound adapters only.
- Contract (`SCALA-API-02`): Tapir endpoints as one source for server, client and OpenAPI, or OpenAPI-first generation; gRPC via ScalaPB, fs2-grpc, zio-grpc; version in path or media type.
- Errors (`SCALA-API-03`): map domain errors in the adapter to RFC 9457 `application/problem+json`; never leak stack traces; 400 for decode failures.
- Resilience (`SCALA-API-04`): timeouts everywhere, retries with backoff (cats-retry, `Schedule.exponential`), circuit breakers, idempotency keys for POST.
- Messaging (`SCALA-API-05`): fs2-kafka, zio-kafka or Pekko Connectors; commit offsets after processing, idempotent handlers, outbox in the same transaction.
- Clients (`SCALA-API-06`): one shared client `Resource` per process with a bounded pool; propagate deadlines and trace headers.

## OBS
- Logs (`SCALA-OBS-01`): log4cats or `zio-logging` over SLF4J and Logback with a JSON encoder; trace and request IDs in context; no `println`, no secrets or PII.
- Metrics (`SCALA-OBS-02`): Micrometer or Prometheus client; `snake_case`, base units, bounded label cardinality.
- Tracing (`SCALA-OBS-03`): otel4s, the OpenTelemetry Java agent, or zio-opentelemetry; fiber context needs the library, not just the agent.
- Health (`SCALA-OBS-04`): `/healthz` (no dependencies) and `/readyz` (DB ping with a timeout).

## OPS
- Config (`SCALA-OPS-01`): HOCON with PureConfig, Ciris or zio-config into typed case classes at startup; `${?ENV}` overrides; fail fast on missing values.
- Packaging (`SCALA-OPS-02`, `-03`): `sbt-native-packager` (`Docker/publishLocal`) or `sbt-assembly`; multi-stage image on a Temurin JRE, non-root, `-XX:MaxRAMPercentage`, `-XX:+ExitOnOutOfMemoryError`. GraalVM native image only when startup or RAM dominates and reflection configs exist.
- Lifecycle (`SCALA-OPS-04`): `IOApp` or `ZIOAppDefault`; drain, then stop consumers, then close pools.
- Run caches (`SCALA-OPS-05`): `COURSIER_CACHE` and the sbt boot directory stay shared; only `java.io.tmpdir` is run-scoped. `target/` stays in the tree.
- Cleanup list (`SCALA-OPS-06`): the `artifacts` in `pack.json`: `target/`, `project/target/`, `.bsp/`, `.bloop/`, `.metals/`, `.scala-build/`, `out/`, `*.class`, `hs_err_pid*.log`, `*.hprof`, `*.jfr`, `scoverage-data/`, `scoverage-report/`, `coverage-report/`.

## DOC
Scaladoc on public traits and ADTs (`sbt doc`; `sbt-mdoc` for checked docs); README, ADRs under `docs/adr/`, a changelog from conventional commits (`SCALA-DOC-01..04`). Public API evolution: `@deprecated("msg", "since")`, MiMa filters with a justification, `private[acme]`.
