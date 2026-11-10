# Clojure concurrency

Digest of CLJ-CONC. Race policy: `specs/WORKFLOW.md §6`. Rules: `specs/languages/clojure/RULES.md`.

## Policy
No race detector exists; `race.mode = none`. The required `test-repeat` check (Kaocha, `--randomize --seed {seed}`, 3 runs) is the machine check (`CLJ-CONC-08`). Concurrency-tagged tests repeated 20 times and reflection warnings clean are review items.

## Managed references (CLJ-CONC-01/02/07)
- One `atom` holding one immutable map beats several atoms. `ref` and `dosync` only for coordinated multi-ref changes; `agent` for async single-writer state.
- `swap!`/`dosync` functions are pure: they retry. Do effects after the commit; use `io!` where a function must not run inside `dosync`; `swap-vals!` or `compare-and-set!` when you need the old value.
- Avoid `alter-var-root`, `set!` and cross-thread `binding`: conveyance covers `future`, `pmap` and agent sends, not raw `Thread` or `go` reliably.

## Futures and agents (CLJ-CONC-03/05)
- `deref` with a timeout: `(deref f 5000 :timeout)`. A `future` hides its exception until deref. Call `shutdown-agents` on exit or the JVM may hang about 60 s.
- Catch inside every `go` block, future and agent action (`set-error-handler!`, `:error-mode :continue`).

## core.async (CLJ-CONC-04)
- Never block (`<!!`, `Thread/sleep`, JDBC, HTTP) inside `go`: the pool is small and fixed. Use `thread` or `a/io-thread`.
- Bounded buffers (`(chan 100)`, `sliding-buffer`, `dropping-buffer`); close channels you own; `alts!` with `timeout` when waiting on external input.

## Executors and shutdown (CLJ-CONC-06, CLJ-OPS-04)
- Create `ExecutorService`s in the lifecycle (Integrant `init-key`), name threads, stop in `halt-key!` with `.shutdown` + `.awaitTermination`.
- SIGTERM: shutdown hook calls `(ig/halt! system)`; stop readiness, stop consumers, drain, close pools, `shutdown-agents`.
