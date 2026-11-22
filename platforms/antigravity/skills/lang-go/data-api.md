# Go data, API, observability and ops digest

Rules: `specs/languages/go/RULES.md §24, §28, §36, §44-§47`. Research IDs `GO-DATA-*`, `GO-API-*`, `GO-OBS-*`, `GO-OPS-*`. Frameworks: `frameworks.md`.

## Persistence (GO-DATA-01..09)
- Default SQL-first: `sqlc` (queries in `db/queries/*.sql`) with `pgx/v5`; `squirrel` or `goqu` for dynamic filters; GORM or ent only where the team accepts them. Generated code stays in the outbound adapter.
- Migrations: versioned SQL files, forward-only in prod, run by `goose`, `golang-migrate` or Atlas as a separate job, never from `init()` or on every pod start; expand, backfill, switch, contract (RULES §45).
- Transactions belong to the use case through a port (`UnitOfWork.Do(ctx, func(ctx, tx) error)`); `*sql.Tx` and `pgx.Tx` never reach the domain; `defer tx.Rollback()`; explicit isolation for money-moving code.
- Pool: `pgxpool`, or `SetMaxOpenConns`/`SetMaxIdleConns`/`SetConnMaxLifetime`/`SetConnMaxIdleTime`, sized from the DB limit. Always `*Context` methods; close `rows` and check `rows.Err()`.
- N+1: batch with `WHERE id = ANY($1)`, joins, or sqlc `:many`. Always `ORDER BY` with a unique tie-break. Nullable columns via `sql.Null*`, `pgtype` or pointers, converted at the boundary. Optimistic concurrency with a `version` column (check rows affected); `FOR UPDATE SKIP LOCKED` for queues.
- CI: `sqlc vet` and `sqlc diff`, migration lint (`atlas migrate lint`, `squawk`). Caches (Redis) behind a port with documented invalidation (RULES §36).

## HTTP and RPC (GO-API-01..10)
- `net/http` with the 1.22 `ServeMux`: `mux.HandleFunc("GET /orders/{id}", h)`, `r.PathValue("id")`, `{$}`, `{rest...}`; most specific wins; conflicts panic at registration; the trailing-slash redirect is 307 since 1.26. Add chi when you need route groups. Importing `net/http/pprof` registers on `DefaultServeMux`: serve it on an internal listener.
- Middleware is `func(http.Handler) http.Handler`: recover, request id, logging and tracing, auth, limits. Handlers decode, call one use case, encode.
- Contracts: OpenAPI-first (`oapi-codegen`) or protobuf-first (`buf` with Connect or gRPC; `buf lint`, `buf breaking`). Version in the path (`/v1/`) or proto package; additive changes only inside a version.
- Errors at the edge: one mapper from domain result, validation and infrastructure errors to status codes; RFC 9457 `application/problem+json`; never leak internals.
- Outbound: one shared `*http.Client` with `Timeout` and a tuned `Transport` (never `http.DefaultClient`), `http.NewRequestWithContext`, `defer resp.Body.Close()` and drain, `otelhttp.NewTransport`. Retry only idempotent calls with exponential backoff and jitter; circuit breaker in the adapter; `golang.org/x/time/rate`; idempotency keys for POST (RULES §46).
- Messaging: `kafka-go` or `franz-go`, `nats.go`, `amqp091-go`, AWS SDK v2; at-least-once, idempotent handlers, commit after processing, outbox for sends.
- gRPC and Connect: deadlines, max message sizes, interceptors, `grpc.health.v1`, reflection only outside prod.
- JSON: `encoding/json` (v2-backed on 1.27, `GOEXPERIMENT=nojsonv2` opts out); `DisallowUnknownFields` at the edge; adopt `encoding/json/v2` for new code after reading its behaviour diffs.

## Observability (GO-OBS-01..07; RULES §24, §47)
- `log/slog`, JSON handler in prod, typed attrs (`LogAttrs`, `slog.String`), `*Context` methods, `sloglint`; `slog.NewMultiHandler` (1.26) and `slog.DiscardHandler` for tests; one correlation id per request; no secrets or PII.
- Traces and metrics: OpenTelemetry (`go.opentelemetry.io/otel`: traces and metrics stable, logs beta), initialised in `platform`, OTLP export, `otelhttp`/`otelgrpc`, W3C `traceparent` through `ctx`; spans in adapters and use-case entry points. A handler wrapper or `otelslog` adds `trace_id` and `span_id` to logs.
- Metrics: OTel or `prometheus/client_golang`; `snake_case` with unit suffixes, bounded label cardinality, RED per endpoint.
- Health: `/livez` (no deps) and `/readyz` (deps, fails during shutdown); build info from `debug.ReadBuildInfo`. `pprof` and `/metrics` on a separate internal listener. Observability never changes behaviour.

## Config and packaging (GO-OPS-01..11; RULES §28)
- Config: env vars parsed once in `cmd`/`platform` into a typed struct (`caarlos0/env`, `go-envconfig`, `koanf`), validated at startup with all errors listed; a `-check` flag validates without serving.
- Build: `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=$V" -o bin/app ./cmd/app`; reproducible (`-trimpath`, pinned toolchain), no UPX. Cross-compile with `GOOS`/`GOARCH`.
- Container: multi-stage Dockerfile, builder `golang:<ver>` with `--mount=type=cache` for `GOCACHE` and `GOMODCACHE`, final `gcr.io/distroless/static` or `scratch`, non-root; scan (`trivy`), sign (`cosign`), provenance in CI. The production image stays `CGO_ENABLED=0`; the test image keeps gcc for `-race`.
- Runtime: `GOMEMLIMIT` and CPU requests and limits in the manifest (`GOMAXPROCS` reads the cgroup limit since 1.25). Graceful shutdown: `concurrency.md`.
- loops run caches: `GOCACHE` and `GOTMPDIR` go into the run directory; `GOMODCACHE` and `GOPATH` stay shared. Build and test leftovers (`*.test`, `*.out`, `*.prof`, `*.pprof`, `*.cov`, `*.exe`, `coverage*.txt`, `__debug_bin*`) are cleaned by loops.
