# TypeScript data, API, observability and operations digest

Rules: RULES.md DATA, API, OBS, OPS. Consistency patterns: `skills/consistency`.

## Persistence (TS-DATA-01 to 06)
- Typed query builder or thin ORM: Kysely or Drizzle (SQL-first), Prisma (schema-first), TypeORM or MikroORM only where already used; raw `pg` or `postgres.js` for hot paths. ORM entities stay in adapters.
- Migrations are versioned files in `migrations/`, applied by a tool (Prisma Migrate, drizzle-kit, node-pg-migrate, Knex, Flyway) in CI or deploy; never `synchronize: true` in prod; expand-contract for zero downtime.
- Transactions open in the application layer through a `UnitOfWork` port (`runInTransaction(fn)`); pass the tx handle explicitly or via `AsyncLocalStorage`; keep them short, no network calls inside.
- Avoid N+1 (joins, `include`, DataLoader, `inArray` batches; no lazy-loading proxies).
- One shared pool per process: `pg.Pool` with `max`, `idleTimeoutMillis`, `connectionTimeoutMillis`, `statement_timeout`; handle `pool.on('error')`; release clients in `finally`; size `max` against processes times replicas.
- Outbox pattern for events published with a DB write; idempotent consumers.

## APIs (TS-API-01 to 07)
- Routes live in inbound adapters; middleware handles cross-cutting concerns only.
- Generate the contract: OpenAPI from zod or typebox (`@fastify/swagger`, `@asteasolutions/zod-to-openapi`, `@nestjs/swagger`) or OpenAPI-first with `openapi-typescript`; gRPC via `@grpc/grpc-js` and `ts-proto`.
- Errors are RFC 9457 `application/problem+json` from one central handler; domain errors map to status at the adapter; never leak stack traces.
- Outbound: timeout (`AbortSignal.timeout`), bounded jittered retries only for idempotent calls, circuit breaker (`opossum`, `cockatiel`), idempotency keys on mutating endpoints. Inbound rate limits (`@fastify/rate-limit`, `express-rate-limit`).
- Built-in `fetch` (undici) with a shared `Agent` and a `signal`; default `fetch` has no timeout; avoid `axios` defaults.
- Limit body size and content types (`bodyLimit`, `express.json({limit})`).
- Messaging: kafkajs or Confluent with manual commit after processing; BullMQ, NATS or SQS with a DLQ; handlers idempotent (at-least-once).

## Observability (TS-OBS-01 to 05)
- pino JSON logs (`nestjs-pino`), request id via `AsyncLocalStorage`, no PII, no `console.log` in services.
- Prometheus via `prom-client` (or the OTel metrics SDK): `snake_case`, base units, bounded label cardinality.
- OpenTelemetry JS: `@opentelemetry/sdk-node` loaded before app code (`node --import ./instrumentation.js`); W3C trace context into logs. Tracing and metrics are stable, logs are maturing (as of 2026-11).
- `/healthz` (liveness, no dependencies) and `/readyz` (readiness, checks the DB); readiness goes false when shutdown starts.
- Sentry or OTel exception events; `process.on('unhandledRejection')` and `'uncaughtException'` log then exit non-zero, never continue.

## Configuration (TS-OPS-01)
Parse `process.env` once in a config module with a zod schema (or `@nestjs/config` with validation, `env-schema`); fail fast on boot; inject typed config; read `process.env` nowhere else. `--env-file=.env` is for dev only.

## Packaging (TS-OPS-02, 03, 05, 06)
- Multi-stage Docker: build stage `npm ci` plus `tsc`; runtime stage `npm ci --omit=dev`, copy `dist/`; pinned LTS tag or digest (slim or distroless), `USER node`, `NODE_ENV=production`; `.dockerignore` for `node_modules`, `.git`, `.env`.
- Run `node dist/main.js` in exec form with an init (`--init`, tini), not `npm start`.
- Single binaries (Node SEA, `bun build --compile`, `deno compile`) suit CLIs; default to images.
- Run-scoped state: `TMPDIR` is scoped per run; package-manager download caches (`npm_config_cache`, pnpm store, `YARN_CACHE_FOLDER`, `BUN_INSTALL_CACHE_DIR`, `DENO_DIR`) are shared on purpose (they are file-locked and costly to refill). Cleanup removes only artifacts the run created: `dist/`, `build/`, `coverage/`, `*.tsbuildinfo`, `.turbo/`, `node_modules/.cache/`, `.eslintcache`, `*.cpuprofile`, `*.heapprofile`, `*.heapsnapshot`, `.nyc_output/`, `junit.xml`, `lcov.info`.

## Documentation (TS-DOC-01, 02, 04)
TSDoc on exported API (TypeDoc for published libraries); README states Node version, package manager, scripts and env vars; ADRs in `docs/adr/`; Changesets `CHANGELOG.md`; publish OpenAPI from CI and treat `oasdiff` contract diffs as review artifacts.
