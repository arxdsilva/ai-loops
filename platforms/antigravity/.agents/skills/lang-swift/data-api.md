# Swift: data, API, observability and ops digest

Rules in RULES.md DATA, API, OBS, OPS. Universal consistency rules: `loops:consistency`.

## Persistence
- Hexagonal services: PostgresNIO (`PostgresClient`, async, pooled) or SQLKit, mapping rows to domain types in the repository adapter. Fluent suits Vapor; its `Model` classes stay in the adapter (SWIFT-DATA-01, -04).
- Migrations: Fluent `AsyncMigration` or plain SQL applied by Flyway, golang-migrate or sqitch; forward-only in production, expand-contract for zero downtime (SWIFT-DATA-02).
- Transactions at the use-case boundary (`withTransaction { }`), the connection passed through a unit-of-work port; default `READ COMMITTED`, `SERIALIZABLE` plus retry for invariants (SWIFT-DATA-03).
- One shared `PostgresClient` per process, run inside the `ServiceGroup`, with connect and statement timeouts and a sized pool (SWIFT-DATA-05).
- Fluent traps: lazy relations need `.with(\.$children)`; unloaded `@Parent`/`@Children` throw; models are not `Sendable` (SWIFT-DATA-04).

## API and integration
- Hummingbird 2 or Vapor 4 (Vapor 5 is beta as of 2026-11) (SWIFT-API-01). OpenAPI-first with swift-openapi-generator, gRPC with grpc-swift-2; generated sources are never hand-edited (SWIFT-API-02).
- Map domain errors to RFC 9457 problem+json in one error middleware; never leak internal text (SWIFT-API-03).
- Every outbound call has a timeout, bounded jittered retries, idempotency keys on mutations; write circuit breakers as an actor (SWIFT-API-04).
- Messaging clients (Kafka, NATS, SQS, RabbitMQ) sit behind ports with an outbox and idempotent consumers (SWIFT-API-05).
- One long-lived `HTTPClient`, shut down through the `ServiceGroup`; `URLSession` on Linux needs `import FoundationNetworking` (SWIFT-API-06).

## Observability
swift-log (`LoggingSystem.bootstrap` once, structured metadata, no `print`, no secrets), swift-metrics, swift-distributed-tracing with swift-otel in the `ServiceGroup`, `/health/live` and `/health/ready` routes (SWIFT-OBS-01..04).

## Config, packaging, deployment
- Typed `Sendable` config from env at startup (swift-configuration or swift-argument-parser); fail fast on missing values (SWIFT-OPS-01).
- Multi-stage Dockerfile: `swift:6.4-noble` builds `swift build -c release --static-swift-stdlib`, runtime `ubuntu:noble` or `swift:6.4-slim`, non-root. Cross-compile with Swift SDKs (SWIFT-OPS-02, -03).
- Graceful shutdown: `ServiceGroup` with SIGTERM, drain, close the pool, flush OTel; Kubernetes grace period above drain time (SWIFT-OPS-04).
- Perf: benchmark with package-benchmark in `-c release`; profile with `perf` or Instruments; ship `-c release` (SWIFT-PERF-01..04).
