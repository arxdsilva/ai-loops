# Rust data, API, observability and operations (digest)

Cites RULES.md. Project choices (ORM, framework, config crate) win; log overrides.

## Persistence (DATA)
- Default `sqlx` (async, compile-time-checked SQL); `diesel` (+ `diesel_async`) for a typed DSL; `sea-orm` for CRUD-heavy relations. The choice stays inside the outbound adapter crate (RUST-DATA-01).
- Migrations: `sqlx migrate` or `diesel migration`, forward-only, expand/contract. Commit `.sqlx/` (`cargo sqlx prepare --workspace`), build CI with `SQLX_OFFLINE=true`, check with `cargo sqlx prepare --check` (RUST-DATA-02).
- Transactions are an application concern behind a `UnitOfWork` port; a dropped `Transaction` rolls back; never hold one across a network call (RUST-DATA-03).
- N+1 in Rust is a loop of `.await` queries: use `JOIN`, `= ANY($1)`, or the ORM loader (RUST-DATA-04).
- One pool per process from `bootstrap` with `max_connections`, `acquire_timeout`, `idle_timeout`, `max_lifetime`; async drivers only; sync `diesel` needs `spawn_blocking` (RUST-DATA-05).
- Private row structs in the adapter, `TryFrom` to domain types; no `FromRow` or `Serialize` on entities (RUST-DATA-06). Money as `rust_decimal` or minor units; UUID v7 via an injected `IdGenerator`; UTC `timestamptz` (RUST-DATA-07).

## API (API)
- Default `axum` + `tower` + `tower-http`; handlers are inbound adapters that extract, validate, call one use case and map the result; state via `State<AppState>` holding `Arc`s (RUST-API-01, -02). Details in [frameworks.md](frameworks.md).
- Contracts: `utoipa` or `aide` for OpenAPI, `tonic` + `prost` for gRPC (`.proto` in `proto/`, lint with `buf`), `async-graphql`; version in the path (RUST-API-03).
- One `ApiError` enum with `IntoResponse` mapping errors to status codes and RFC 9457 problem JSON; log the source, return a stable code (RUST-API-04).
- Resilience as tower layers: timeout, concurrency limit, load shed, rate limit (`tower-governor`); retries with backoff and jitter only for idempotent calls; `Idempotency-Key` for POST (RUST-API-05).
- Messaging (`rdkafka`, `lapin`, `async-nats`): transactional outbox, idempotent consumers, ack after commit (RUST-API-06).
- One long-lived `reqwest::Client` with `timeout` and `connect_timeout`, `rustls`; never per request (RUST-API-07). SSE and WebSockets use bounded channels and a `Drop` guard (RUST-API-08).

## Observability (OBS)
- `tracing` + `tracing-subscriber` (`EnvFilter`, JSON in production); `#[instrument(skip(...))]`; no `println!`, no secrets or PII in logs (RUST-OBS-01).
- Request id (`SetRequestIdLayer`, `PropagateRequestIdLayer`) and `TraceLayer` with a span (RUST-OBS-02). Metrics with the `metrics` facade, low-cardinality labels, route templates (RUST-OBS-03). OpenTelemetry via `tracing-opentelemetry`, W3C `traceparent` (RUST-OBS-04).
- `/healthz` (liveness) and `/readyz` (readiness with a DB ping and timeout) as plain routes (RUST-OBS-05). `tokio-console` in development only (RUST-OBS-06).

## Operations (OPS)
- Typed config from env over a file (`figment` or `config`), validated at startup, secrets as `SecretString`, `dotenvy` locally only (RUST-OPS-01).
- Multi-stage Dockerfile: builder with `cargo-chef` or BuildKit cache mounts, final `distroless/cc` or musl `scratch`, `--release --locked`, non-root (RUST-OPS-02, -03). Reproducible builds per RUST-OPS-07.
- SIGTERM/SIGINT drain with a deadline, flush telemetry, close the pool (RUST-OPS-04, RUST-CONC-10).
- Cache and cleanup: per-run `CARGO_TARGET_DIR`, shared `CARGO_HOME`; leftover `target/`, `lcov.info`, `*.profraw` are artifacts; `proptest-regressions/` and `.sqlx/` are committed and never deleted (RUST-OPS-05, -06, -08).
- Docs: `///` on every public item with `# Errors`, `# Panics`, `# Safety`; `cargo doc` with `RUSTDOCFLAGS="-D warnings"` (the pack's optional `doc` check); ADRs in `docs/adr/` (RUST-DOC-01, -02, -04).
