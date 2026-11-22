# PHP data, API, observability and operations

Digest of RULES DATA, API, OBS, OPS.

## Persistence
- Laravel: Eloquent (Active Record) plus the query builder for reports; Symfony: Doctrine ORM (Data Mapper) with DBAL/DQL; framework-free: DBAL, Cycle or PDO behind a port. The repository port returns domain objects, never Eloquent models or Doctrine proxies (`PHP-DATA-01`).
- Migrations: Doctrine Migrations or Laravel migrations (`migrate --force`); commit them, never edit applied ones, expand-contract for zero downtime (`-02`).
- One use case = one transaction (`wrapInTransaction`, `DB::transaction`), in the application layer; outbox rows inside, dispatch after commit (`-03`).
- N+1: `with()`, `preventLazyLoading()` outside production, `chunkById`/`toIterable`, keyset pagination, `clear()` in long Doctrine loops (`-04`, `PHP-PERF-04`).
- Connections: pooler (PgBouncer/ProxySQL), avoid `PDO::ATTR_PERSISTENT` unless tested, timeouts, reconnect in workers (`-05`).
- Money as integer minor units or `DECIMAL`/`brick/money`; UTC timestamps; UUIDv7/ULID from an id port (`-06`).

## API
- Thin controllers: request -> command, result -> response; no business logic (`PHP-API-01`).
- Contracts generated and diffed in CI (API Platform, `swagger-php`, Scramble); RFC 9457 problem details from one mapper (`-02`, `-03`).
- Outbound: explicit connect and total timeouts, bounded retries with backoff and jitter on idempotent verbs, Idempotency-Key on POST, circuit breaker, inbound rate limiting (`-04`). `max_execution_time` does not cap a hung socket.
- Messaging: Messenger or Laravel queues, transactional outbox, idempotent handlers, retry and failure transport (`-05`). Clients behind PSR-18, one per host in workers (`-06`).

## Observability (`PHP-OBS-*`)
PSR-3 logger (Monolog JSON to stdout), context arrays, correlation id (`-01`). Metrics via Prometheus client (shared store in FPM) or OTel, low-cardinality labels (`-02`). OpenTelemetry PHP traces, W3C `traceparent` (`-03`). Liveness is cheap, readiness checks dependencies; Laravel `/up` (`-04`).

## Operations
- 12-factor config; `env()`/`$_ENV` only in config files (Laravel `config:cache` returns null elsewhere); typed config objects at the root (`PHP-OPS-01`).
- Container: multi-stage, `composer install --no-dev --optimize-autoloader --classmap-authoritative --no-scripts`, non-root, no Xdebug (`-02`). OPcache on (`PHP-PERF-03`).
- Lifecycle: FPM graceful stop, `octane:reload`, `messenger:stop-workers`; rebuild caches then reload on deploy (`-04`).
- Loops runspace (`-05`, `-06`): `TMPDIR` and `APP_CACHE_DIR` are scoped per run; `COMPOSER_CACHE_DIR` and `COMPOSER_HOME` stay shared (D15). Cleanup removes new `.phpunit.cache/`, `.php-cs-fixer.cache`, `coverage/`, `var/cache/`, `bootstrap/cache/*.php`; `vendor/` is never an artifact.
- Performance: PHPBench baselines, Xdebug/Blackfire/SPX profiling (never in production) (`PHP-PERF-01`, `-02`).
