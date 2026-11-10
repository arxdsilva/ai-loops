# C++ data, API, observability, operations (digest)

Cites RULES.md. The neutral rules: `ARCHITECTURE §13`, `§18`, `§19`, `§22`.

## Data (CPP-DATA-01..05)
- Thin SQL layer over a full ORM: libpqxx, SOCI, sqlpp11, Boost.MySQL, or the framework driver (userver, Drogon). Keep SQL in the outbound adapter.
- Migrations: a standalone forward-only tool run before app start (Flyway, Liquibase, sqitch, goose, dbmate), expand then contract; never in the service binary.
- Transactions open in the application layer behind a unit-of-work port, implemented as an RAII guard (`pqxx::work` rolls back on destruction). Short transactions; document isolation; retry on serialisation failure.
- Avoid N+1: joins or batched `ANY($1)`; never return a connection-bound cursor from a repository.
- Pool with timeouts, query timeouts and max lifetime; never share a connection across threads without a lock; async drivers on I/O threads.

## API (CPP-API-01..07)
- Contract first: `.proto` compiled by `protoc` with the gRPC plugin in CMake (`protobuf_generate`), checked with `buf lint` and `buf breaking`; for REST an OpenAPI document and contract tests. Generated code lands in the build tree and is excluded from coverage and lint.
- gRPC: prefer the callback API (`CallbackService`, reactors) over the CQ API; a deadline on every client call (`ClientContext::set_deadline`); canonical status codes.
- Validate at the adapter boundary and map domain errors to RFC 9457 problem details or gRPC status; the domain never sees HTTP types.
- Outbound: timeouts on every call, retries with backoff and jitter on idempotent operations only, circuit breaker, idempotency keys for writes (gRPC retry policy via service config).
- Messaging: librdkafka (cppkafka), AMQP-CPP or rabbitmq-c, cnats; transactional outbox in the persistence adapter; idempotent at-least-once consumers; commit offsets after processing.
- HTTP client: Beast/Asio, libcurl (`CURLOPT_TIMEOUT_MS`, `CURLOPT_CONNECTTIMEOUT_MS`), cpr; reuse connections; check TLS.

## Observability (CPP-OBS-01..05)
- spdlog (async, JSON sink) or the framework logger; structured fields and correlation ids; no secrets; no `std::cout` in services.
- prometheus-cpp or OpenTelemetry metrics, Prometheus naming, bounded label cardinality.
- OpenTelemetry C++ SDK for traces; W3C `traceparent`; pass context explicitly across async boundaries (no auto-instrumentation agent).
- `/healthz` (liveness, no dependency checks) and `/readyz` (readiness) on a separate light handler; gRPC `grpc.health.v1`.
- Diagnosable crashes: `-g` with split debug info, symbols kept per release, core dumps enabled, a `std::set_terminate` handler that logs `std::stacktrace`.

## Operations (CPP-OPS-01..07)
- Typed config struct from env or files at startup (CLI11, toml++, yaml-cpp); validate and fail fast; secrets from env or a vault.
- Multi-stage OCI image: pinned toolchain image to build, `gcr.io/distroless/cc-debian12` (dynamic glibc) or scratch/alpine for static musl; non-root, read-only root filesystem, probes. Link `-static-libstdc++ -static-libgcc`; fully static only without `dlopen`/NSS needs. Strip and ship symbols separately.
- Graceful shutdown: `asio::signal_set` for SIGTERM and SIGINT, stop accepting, drain with a deadline, join threads, flush logs, exit 0; raw handlers only write or set an atomic.
- Reproducible builds: toolchain image by digest, `SOURCE_DATE_EPOCH`, no `__DATE__`/`__TIME__`, `-ffile-prefix-map=$PWD=.`, SBOM (vcpkg SPDX, conan-sbom, syft) and signature.
- Caches: `CCACHE_DIR`, vcpkg and Conan caches shared; only `CCACHE_TEMPDIR` per run (CPP-OPS-05).
