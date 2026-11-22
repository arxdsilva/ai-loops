# Ruby concurrency and the race substitute

Digest of RUBY-CONC. Policy: WORKFLOW §6 and `specs/languages/ruby/pack.json` `race`.

## No detector, so the substitute is required
`race.mode = none`, `race.checks = ["test"]`: the `test` check runs random order with a printed seed, twice (RUBY-CONC-05). Do not pin seeds, disable random order or add retries. A failure that vanishes on rerun is a bug. Record the seed from `Randomized with seed N` in claims.
Review items for code with shared state: parallel runs (`parallel_rspec -n N`, Rails `parallelize`) and thread stress tests (N threads against one object).

## The model (RUBY-CONC-01/02)
- The GVL runs Ruby threads concurrently only during blocking I/O. CPU parallelism needs processes (Puma workers, Sidekiq) or Ractors.
- It does not make compound operations atomic: `@x ||= v`, `h[k] ||= v`, `count += 1`. Guard with `Mutex#synchronize`, `Concurrent::Map`, `Concurrent::AtomicFixnum`, or avoid shared state. JRuby and TruffleRuby run truly parallel.
- Fibers with `Fiber.set_scheduler` (the `async` gem) give cooperative I/O. Use `Async` or Falcon only if the whole stack is scheduler-aware; no blocking C extensions in scheduler fibers; `Timeout.timeout` is unsafe, use client timeouts (RUBY-CONC-07).

## Puma and pools (RUBY-CONC-03/04/10)
- Per-request state never lives in class variables, class-level instance variables or mutable constants; use `ActiveSupport::CurrentAttributes` or locals. Class-level memoised caches must be thread-safe. Check gems for thread safety.
- DB pool >= Puma threads per process and >= Sidekiq `concurrency`; read `RAILS_MAX_THREADS` in both `puma.rb` and `database.yml`.
- Manual threads: `with_connection`, join them, no fire-and-forget `Thread.new` in request code.

## Ractors (RUBY-CONC-06)
Experimental in 4.0 (`Ractor::Port`, `Ractor.yield` and `take` removed). Avoid in production services.

## Jobs (RUBY-CONC-08/09)
- Pass JSON-safe ids; `perform` is idempotent (at-least-once delivery); cap retries; handle `sidekiq_retries_exhausted`; shut down with `TSTP` then `TERM` and a `-t` timeout (RUBY-OPS-04).
- Enqueue after commit (`after_commit`, or `enqueue_after_transaction_commit` on Rails 7.2+), never inside the transaction.
- Skill `loops:jobs-and-state-machines` covers outbox and idempotency in general.
