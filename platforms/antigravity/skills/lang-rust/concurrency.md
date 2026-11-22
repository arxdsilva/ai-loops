# Rust concurrency and the race substitute (digest)

Cites RULES.md. The rule: `WORKFLOW §6`; the pack: `race.mode = none`, `race.checks = ["clippy"]`.

## The substitute
- Compile time: `Send`/`Sync` bounds make unsynchronised shared mutation a compile error in safe code (RUST-CONC-02).
- Gate (required): `cargo clippy ... -- -D warnings -D unsafe_code -D clippy::await_holding_lock -D clippy::await_holding_refcell_ref`. Opt out of `unsafe_code` only per crate with `#[allow(unsafe_code)]`, `// SAFETY:` comments and the tiers below (RUST-CONC-04, RUST-IDIOM-04).
- Optional tiers, `default: off` in the pack (enable in config or `loops gate --only <name>`):

| Check | Command | Use for |
|---|---|---|
| `miri` | `MIRIFLAGS=-Zmiri-many-seeds cargo +nightly miri test {pkgs}` | UB and data races in crates with `unsafe` or atomics |
| `loom` | `RUSTFLAGS="--cfg loom" cargo test --release {pkgs}` | interleaving models of lock-free code (`#[cfg(loom)]` types, `loom::model`) |
| `tsan` | `RUSTFLAGS=-Zsanitizer=thread cargo +nightly test -Zbuild-std --target <host> {pkgs}` | FFI and mixed code; Linux and macOS only |

`miri` and `tsan` need nightly (`rustup toolchain install nightly`, plus `miri` or `rust-src`); `loom` changes `RUSTFLAGS`, so it rebuilds the graph in the run's target dir (RUST-CONC-03).

## Async rules
- One `tokio` multi-thread runtime per process; OS threads only for blocking or CPU work; `rayon` never inside an async task without `spawn_blocking` (RUST-CONC-01).
- Never block the runtime: `tokio::fs`, `tokio::time::sleep`, `spawn_blocking` for short blocking work, dedicated threads for long work; aborting a `spawn_blocking` task has no effect (RUST-CONC-05).
- No `block_on` inside the runtime; no `std::sync::mpsc` or `Condvar` in async code (RUST-CONC-11).
- Futures given to `tokio::spawn` are `Send + 'static`: no `Rc`, `RefCell`, `MutexGuard` held across `.await`; `LocalSet` only for `!Send` work (RUST-CONC-08, RUST-IDIOM-14).
- Every `.await` is a cancellation point: no half-applied state across it, check the cancel-safety note of each `select!` branch, make DB writes transactional (RUST-CONC-06).
- Structured concurrency: `JoinSet` or `join!`/`try_join!`; keep every `JoinHandle` and observe `JoinError`; no fire-and-forget spawn without error logging (RUST-CONC-07).
- Share by ownership: move data into tasks, `mpsc`/`oneshot`/`watch`/`broadcast`; `Arc<Mutex<T>>` only for short critical sections; atomics for counters and flags (RUST-CONC-02).
- Bound everything: `mpsc::channel(n)` (unbounded only with a documented cap), `Semaphore`, `tokio::time::timeout` on every outbound call, tower `timeout`/`limit` layers inbound (RUST-CONC-09).
- Graceful shutdown: one `CancellationToken` or `watch`, `tokio::signal` for SIGTERM and Ctrl-C, `with_graceful_shutdown`, drain with `JoinSet` and a deadline, no `process::exit` in handlers (RUST-CONC-10, RUST-OPS-04).
