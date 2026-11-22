# Data, API, observability and operations (Elixir)

Digest of RULES.md DATA, API, OBS, OPS.

## Persistence (`ELIXIR-DATA-01`..`-07`)
- Ecto with `Repo` as the only DB gateway; no lazy loading: preload or join explicitly (`ELIXIR-DATA-05`). With hexagonal, schemas live in adapters and return domain structs.
- Migrations: `mix ecto.gen.migration`; prefer `change/0`; `up/down` when irreversible; `modify/3` needs `:from` (`ELIXIR-DATA-02`).
- Zero downtime (`ELIXIR-DATA-03`): nullable column then backfill; `create index(..., concurrently: true)` with `@disable_ddl_transaction true` and `migration_lock: :pg_advisory_lock`; `validate: false` constraints then validate; split rename/drop across releases.
- Transactions in the application layer: `Repo.transact/2` or `Ecto.Multi`; `lock: "FOR UPDATE"` for read-modify-write (`ELIXIR-DATA-04`).
- Keyset pagination, no unbounded `Repo.all`, `insert_all` for bulk (no autogen timestamps).
- Pools: `pool_size` at least expected concurrency; PgBouncer transaction mode only with `prepare: :unnamed`; Oban needs its own connections (`ELIXIR-DATA-06`).
- Oban: enqueue in the same transaction, idempotent workers, deliberate `unique:`, `Oban.Testing` with `testing: :manual` (`ELIXIR-DATA-07`).

## API (`ELIXIR-API-01`..`-06`)
- Phoenix on Bandit over Plug; thin controllers call contexts; `action_fallback` for error tuples.
- Errors mapped in one place to RFC 9457 problem details; changeset errors to 422 via `Ecto.Changeset.traverse_errors/2`.
- Contracts: `open_api_spex` or OpenAPI-first; Absinthe; `grpc` + `protobuf`; version with a deprecation window.
- Clients: `Req` with a shared named client and `receive_timeout`; retries with backoff; rate limiting (`Hammer`, `PlugAttack`); idempotency keys. `fuse` is stale (2021).
- Messaging: Broadway (ack after processing), outbox via Oban, idempotent handlers.

## Observability (`ELIXIR-OBS-01`..`-04`)
- `Logger` with structured reports and `Logger.metadata/1`; JSON via `logger_json`; never log secrets or PII.
- `:telemetry.execute/3` and `:telemetry.span/3`; handlers fast and attached once; `telemetry_metrics` plus a reporter.
- OpenTelemetry (traces stable): propagate context into `Task`s.
- `/healthz` (no dependencies) and `/readyz` (DB, queues) via a plug before the router; 503 while draining.

## Operations (`ELIXIR-OPS-01`..`-06`)
- `config.exs` is build time; `runtime.exs` reads `System.fetch_env!`; no `compile_env` for deploy-time values.
- `MIX_ENV=prod mix release`; `mix phx.gen.release --docker` (multi-stage, pinned `hexpm/elixir` builder, slim runner running `bin/server`); build on the target OS and libc; no AOT for servers.
- Lifecycle: SIGTERM stops applications in reverse start order; Oban and Broadway drain.
- Runspace: per-run `MIX_BUILD_ROOT`, shared `deps/`, `MIX_HOME`, `HEX_HOME`; cleanup never removes `deps/` or `mix.lock`.
