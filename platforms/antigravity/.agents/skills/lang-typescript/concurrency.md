# TypeScript concurrency digest

Rules: RULES.md CONC. Race policy and the gate substitute: SKILL.md §4.

## The model (TS-CONC-01, 02, 03)
One JS thread runs an event loop; I/O is async (libuv); CPU work goes to `worker_threads`; processes or `cluster` scale across cores.
- There are no memory-model data races in single-threaded code. The races are logical: another request runs at every `await`.
- Treat module-level mutable state, in-memory caches and check-then-act across an `await` as shared state. `if (!(await exists(id))) await create(id)` is a bug: use a DB unique constraint with upsert, a per-key mutex or queue, or an atomic operation.
- Between workers use `postMessage` (structured clone). `SharedArrayBuffer` with `Atomics` is the only true memory race: use it only with a documented protocol; no detector covers it.

## Promises (TS-CONC-07)
- Every promise is awaited, returned, or `void`-ed with a `.catch`. Node's default `--unhandled-rejections=throw` crashes the process.
- `Promise.all` fails fast; `Promise.allSettled` when partial results are fine; cap parallelism (p-limit). `forEach(async ...)` never awaits.

## Do not block the loop (TS-CONC-06)
No `*Sync` fs or crypto in request paths, no large `JSON.parse` or `JSON.stringify`, no backtracking regex on user input. Offload CPU to `worker_threads` (piscina) or a queue.

## Cancellation and timeouts (TS-CONC-08, 09)
- Propagate `AbortSignal`: `AbortSignal.timeout(ms)`, `AbortSignal.any([...])`; pass `signal` to `fetch`, `fs.promises`, `events.once`, streams and drivers that accept it; abort on client disconnect and shutdown.
- Server `headersTimeout`, `requestTimeout`, `keepAliveTimeout`; a client timeout on every outbound call.

## Background work (TS-CONC-10)
Durable queue (BullMQ, pg-boss, SQS), idempotent handlers, shutdown that stops accepting jobs, drains in-flight ones, then exits. In-process timers die with the pod. See `skills/jobs-and-state-machines`.

## Making the race checks pass
- A test that fails under shuffle depends on order or shared state: reset the module state in `beforeEach`, inject the clock, give each test its own data (schema or transaction).
- A test that fails only under `--sequence.concurrent` shares a fixture: use `it.concurrent` only for tests that own their state, or isolate the fixture per test.
- Replay with the printed seed: `--sequence.seed=N`. Never add retries, `--no-randomize` or `--sequence.concurrent=false`; the race policy rejects them (WORKFLOW §6).
- vitest 5 documents `--retry.count`, and plain `--retry=0` still works; both must stay 0.
- Other runners: `node --test --test-randomize --test-concurrency=2` (Node 26.1+, early development), `jest --randomize`, `bun test --randomize --rerun-each=3`.

## Shutdown (TS-OPS-04)
On `SIGTERM` or `SIGINT`: flip readiness false, `server.close()` plus `closeIdleConnections()`, abort in-flight work, close DB and queue clients, force-exit after a deadline. NestJS: `app.enableShutdownHooks()`.
