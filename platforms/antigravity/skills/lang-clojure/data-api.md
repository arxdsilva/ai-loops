# Clojure data, API, observability, ops

Digest of CLJ-DATA, CLJ-API, CLJ-OBS, CLJ-OPS. Rules: `specs/languages/clojure/RULES.md`.

## Persistence (CLJ-DATA-01..05)
- next.jdbc (`execute!`, `execute-one!`, `plan`) plus HoneySQL; no ORM. Map rows to domain maps in the outbound adapter.
- Migrations: Migratus or Ragtime, versioned `.sql`, run by one job, not every replica.
- Transactions are opened in the application layer: `jdbc/with-transaction [tx ds]`; pass `tx` to repository functions.
- N+1 is repeated queries inside `map`: batch with `[:in :id ids]` and group in memory.
- Pool with HikariCP via `next.jdbc.connection/->pool`; set size and timeouts.

## HTTP and integration (CLJ-API-01..06)
- Ring handlers (map to map) with Reitit routes as data, OpenAPI via `reitit-swagger`; problem details (RFC 9457) from exception middleware mapping `ex-data` `:type`.
- Outbound: `hato` or `clj-http` with a shared client, connect and read timeouts, retries only for idempotent calls (backoff, jitter, circuit breaker via `diehard`).
- Messaging: Java clients through interop (`jackdaw`, `langohr`, aws-api); outbox pattern for DB-plus-message writes.

## Observability (CLJ-OBS-01..04)
- Log with `clojure.tools.logging` on Logback (JSON) or mulog; log maps with `:correlation-id`; no secrets.
- Metrics with iapetos or Micrometer, bounded label cardinality. Traces with the OpenTelemetry Java agent.
- `/healthz` (no deps) and `/readyz` (DB reachable) as routes; readiness fails on shutdown.

## Config and packaging (CLJ-OPS-01..06)
- Config as data: aero (`#env`, `#profile`) or Integrant edn, validated with malli at startup; secrets from env or vault.
- AOT uberjar in a multi-stage Dockerfile, `eclipse-temurin:21-jre`, non-root, `-XX:MaxRAMPercentage=75`. GraalVM native-image only for CLIs; avoid it for services.

## Docs (CLJ-DOC-01..04)
- Docstring every public var; ADRs in `doc/adr/`; runnable examples in `(comment ...)` blocks; delete scratch `(comment ...)`, `tap>` and `println` before merge.
