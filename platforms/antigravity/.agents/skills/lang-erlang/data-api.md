# Erlang data, API, observability and ops

Digest of `specs/languages/erlang/RULES.md` DATA, API, OBS, OPS.

## Persistence (`ERL-DATA-01..05`)
- Raw SQL through `epgsql`, `pgo` or `mysql-otp` behind a repository behaviour; no dominant ORM. Mnesia, ETS and DETS are node-local state, and Mnesia has no partition merge (DETS 2 GB limit): durable business data goes to Postgres.
- Forward-only `.sql` migrations in `priv/migrations` run by Flyway, dbmate or sqitch, or an in-app runner at release start; expand then contract.
- Transactions begin and commit in the adapter; expose a use-case unit of work (a function taking a fun). `mnesia:transaction/1`, no dirty ops in the domain.
- Pool connections (`pgo`'s pool, `poolboy`), set query and checkout timeouts, let a supervisor restart crashed connections.

## HTTP and integration (`ERL-API-01..06`)
- Cowboy 2: compiled routes, `cowboy_rest` for resources, `cowboy_websocket` for sockets; set `idle_timeout`, `max_keepalive` and request limits.
- OpenAPI first, `jesse` validation, Protobuf with `gpb` or `grpcbox`, version in the path. Map error tuples to RFC 9457 problem details at the adapter; never leak crash reasons.
- Clients: `gun` or `hackney`, not bare `httpc`; a timeout on every call, retries with backoff and jitter only for idempotent calls, `fuse` breakers, idempotency keys.
- Messaging: `brod` (Kafka), `amqp_client` (RabbitMQ); outbox table, idempotent at-least-once handlers, commit offsets after processing.

## Observability (`ERL-OBS-01..05`)
`logger` with structured maps (`?LOG_INFO(#{event => ...})`), a JSON handler in production, a correlation id in process metadata, never secrets or `io:format`. `telemetry` events plus a `prometheus.erl` reporter. OpenTelemetry Erlang (exporter listed before the SDK; context does not cross spawns). `/health/live` and `/health/ready` checking pools and supervisor children. `recon` and `observer_cli` in the release.

## Config, packaging, lifecycle (`ERL-OPS-01..06`)
- `config/sys.config.src` and `vm.args.src` with `${VAR}` or `${VAR:-default}`; read `application:get_env` at the edge into a typed record.
- `rebar3 as prod tar`; Docker multi-stage with a matching libc and OpenSSL. No escripts for services.
- SIGTERM goes through `init:stop()`; stop Ranch listeners before workers; hot upgrades only with a tested `relup` path, else rolling restarts.
- Caches: `REBAR_CACHE_DIR` and `REBAR_GLOBAL_CONFIG_DIR` shared; `ERL_CRASH_DUMP` per run. Leftovers: `_build/`, `*.beam`, `ct_logs/`, `erl_crash.dump`, `rebar3.crashdump`, `*.coverdata`, `bom.xml`, `Mnesia.*`, `.rebar3/`.
- Docs (`ERL-DOC-01..04`): `-moduledoc` and `-doc` (OTP 27+) with ExDoc; keep `-spec`, `-callback`, `-opaque` current because Dialyzer fails on drift; ADR or README for the supervision tree.
