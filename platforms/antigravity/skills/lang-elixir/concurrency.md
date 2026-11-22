# Concurrency and OTP (Elixir)

Digest of RULES.md CONC. Race policy: `mode = none`; the required gate check is `test-repeat` (see SKILL.md, WORKFLOW §6).

## Model (`ELIXIR-CONC-01`, `-02`)
Isolated processes with private heaps, message passing, links and monitors. One process per unit of concurrent state or fault domain (session, connection, aggregate), not per function. Smallest tool that works:
- state: `Agent`/`GenServer`; lookup by key: `Registry`; work: `Task`/`Task.Supervisor`; per-entity processes: `DynamicSupervisor`;
- read-heavy shared data: `:ets` (owned, `:public` with care) or `:persistent_term`.

## When not to use a process (`ELIXIR-ARCH-09`, `ELIXIR-CONC-09`)
Pure logic, a single call, config lookup, serialising CPU-bound work, limiting request concurrency (Phoenix/Bandit isolate each request already). Wrap each process behind one API module.

## Logical races that remain (`ELIXIR-CONC-03`)
- check-then-act across two calls to a process or the DB: do it in one process or one transaction (`lock: "FOR UPDATE"`, `ELIXIR-DATA-04`);
- ETS read-modify-write on `:public` tables: `update_counter`, `insert_new`, `select_replace`, or one owner process;
- ordering is guaranteed only per sender-receiver pair;
- `Registry.lookup` then `GenServer.call` can hit a dead pid: handle the exit.
The repeat gate surfaces order-dependent failures by seed; it does not prove their absence, so review these patterns.

## GenServer discipline (`ELIXIR-CONC-05`, `-06`)
- Deliberate `call` timeouts (default 5000 ms; caller exits).
- Never block in `init/1`: use `handle_continue/2`.
- No own `receive` in callbacks; no cleanup reliance on `terminate/2` (use links and monitors).
- Send only needed fields across processes; bound mailboxes (GenStage, Broadway, Oban queues, `:max_heap_size`).

## Supervision (`ELIXIR-CONC-08`, `ELIXIR-OPS-04`)
Children in dependency order; `:one_for_one` by default, `:rest_for_one` for ordered dependencies; tune `max_restarts`/`max_seconds`; error-kernel supervisors apart from workers. Start the Endpoint last so it stops first; set `shutdown:` on children; SIGTERM stops applications in reverse start order.

## Background work (`ELIXIR-CONC-07`, `ELIXIR-DATA-07`)
Oban for durable jobs (enqueue in the same transaction, idempotent workers); `Task.Supervisor.async_nolink` for in-memory fire-and-forget; Broadway for ingestion.

## Tests (`ELIXIR-TEST-09`)
Spawn with `start_supervised!/1`; wait with `assert_receive`; propagate Sandbox ownership with `allow/3`; keep modules async unless they touch global state.
