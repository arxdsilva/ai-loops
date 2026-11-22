# Erlang concurrency

Digest of `specs/languages/erlang/RULES.md` CONC. Race policy: SKILL.md and WORKFLOW §6.

## Model (`ERL-CONC-01/02`)
Lightweight processes, preemptive schedulers, asynchronous copying messages (binaries over 64 bytes are reference counted). Spawn per independent unit of work. Share state by messages; ETS only with `{write_concurrency, true}`, `{read_concurrency, true}` and one owner; `persistent_term` only for read-mostly data (updates trigger a global GC scan).

## The race substitute (`ERL-CONC-03/04`)
`mode = none`. `eunit` and `ct` run under `ERL_FLAGS="+S 1:1"` and `"+S 8:8"`; either failing blocks. Add `proper_statem` (`parallel_commands`) for gen_statem, ETS and multi-process modules. Concuerror is dropped (stale).

## Ordering and calls (`ERL-CONC-05/06`)
- Only the pairwise order between two processes is guaranteed. Use `gen_server:call`, monitors, references and aliases, not sleeps. OTP 28 priority messages do not fix ordering bugs.
- Every `gen_server:call` has a chosen timeout (default 5000 ms); never call yourself; avoid call cycles; hand blocking IO to a worker or use `gen_server:reply/2`.

## Overload (`ERL-CONC-07`)
Bound queues: admission control, a pool (`poolboy`, `pooler`), `max_heap_size`, `message_queue_data`. Avoid selective `receive` on a large mailbox. Watch `process_info(Pid, message_queue_len)`; `recon` in production.

## Background work and shutdown (`ERL-CONC-08/09`)
Supervised workers via `proc_lib` or `gen_*`, never bare `spawn`; `erpc` with a timeout; stop with `application:stop` and `init:stop()` honouring supervisor `shutdown` times. Distribution only for clusters of identical nodes; use `pg` for groups.
