# Ruby data, API, observability and operations

Digest of RUBY-DATA, RUBY-API, RUBY-OBS, RUBY-OPS.

## Data
- Active Record in Rails; Sequel or ROM (Hanami) for repository style; Arel or `exec_query` with binds for hard queries; no string-built SQL (RUBY-DATA-01).
- Migrations: reversible `change` or `up`/`down`, commit `db/schema.rb` (or `structure.sql`), never edit a merged migration, `strong_migrations` blocks unsafe operations; expand, backfill in batches, contract (RUBY-DATA-02).
- Transactions belong to the use case, short, with no HTTP call or enqueue inside; `with_lock` or `lock_version` for concurrent updates (RUBY-DATA-03).
- N+1: `includes`/`preload`/`eager_load`, Bullet in dev and test, `strict_loading`, `counter_cache`, `pluck`; avoid default scopes (RUBY-DATA-04, RUBY-PERF-04).
- Pool, `checkout_timeout`, statement and lock timeouts in `database.yml`; PgBouncer when processes x threads exceed server limits (RUBY-DATA-05).
- Constraints in the database (NOT NULL, unique index, foreign keys); `validates_uniqueness_of` alone races (RUBY-DATA-06).

## API
- `rails new --api`, routes in `config/routes.rb`; Roda or Sinatra for tiny services (RUBY-API-01). OpenAPI from request specs (`rswag`) (RUBY-API-02).
- Map domain errors to statuses in one place (`rescue_from`), return RFC 9457 `application/problem+json`, never leak messages or traces (RUBY-API-03).
- Outbound: set `open_timeout` and `read_timeout` (Net::HTTP defaults to 60 s), retry with jittered backoff, `semian` breakers, idempotency keys on POST, `rack-attack` inbound; reuse connections; pass deadlines explicitly (RUBY-API-04/06).
- Messaging: karafka, bunny, aws-sdk; outbox plus idempotent consumers (RUBY-API-05).

## Observability
- JSON logs to STDOUT with request and job ids, filtered params, no secrets or PII (RUBY-OBS-01).
- Rails 8.1 `Rails.event` and `ActiveSupport::Notifications`; metrics via `prometheus-client` or `yabeda` (RUBY-OBS-02).
- OpenTelemetry Ruby with W3C context into jobs and HTTP; traces are stable, check metrics and logs maturity (RUBY-OBS-03).
- `/up` liveness plus a separate fast readiness check; `Rails.error` reporters (RUBY-OBS-04).

## Operations
- `ENV.fetch("X")` in `config/*.rb`, encrypted credentials for secrets; no scattered `ENV` in domain code (RUBY-OPS-01).
- Multi-stage Rails `Dockerfile` (`BUNDLE_DEPLOYMENT=1`, `BUNDLE_WITHOUT="development:test"`, non-root, jemalloc); Kamal without an orchestrator (RUBY-OPS-02). No AOT binary exists (RUBY-OPS-03).
- Signals: Puma `TERM` drains, `SIGUSR2` phased restart; Sidekiq `TSTP` then `TERM`; grace period above `-t` (RUBY-OPS-04).
- Caches and cleanup: `BUNDLE_PATH` shared, `BOOTSNAP_CACHE_DIR` and `TMPDIR` per run; cleanup list in the pack `artifacts` (RUBY-OPS-05/06).
- Docs: YARD on public API, README with `bin/setup`, ADRs, `.rubocop_todo.yml` debt only shrinks (RUBY-DOC-01/02/04). Reviewer first checks: RUBY-DOC-03.
