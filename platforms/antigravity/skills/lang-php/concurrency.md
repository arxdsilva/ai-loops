# PHP concurrency and the race policy

Digest of RULES CONC. `specs/WORKFLOW.md` §6 holds the policy; this is the PHP form.

## Models (`PHP-CONC-01`)
- PHP-FPM/Apache: shared-nothing, one fresh context per request. No data races, only per-request bugs.
- CLI: single-threaded. Fibers (8.1+): cooperative, one thread.
- Long-running servers (FrankenPHP worker, RoadRunner, Swoole/OpenSwoole, ReactPHP, Amp, Laravel Octane): the app lives across requests. The hazard is leaked state, including cross-user data leaks.

## No race detector: the substitute (`PHP-CONC-04`)
Gate checks (`race.mode: none`, `race.checks: ["test","test-parallel"]`, all required):
- `test`: serial, `--order-by=random`, two seeds (`repeat: 2`), `--fail-on-warning --fail-on-risky`, coverage on.
- `test-parallel`: `paratest --processes=2 --order-by=random ...`; without paratest the gate runs a logged serial fallback.
- Reproduce a failure with the printed seed: `vendor/bin/phpunit --order-by=random --random-order-seed=<n>`.
- Suspect global state: `--process-isolation`, or repeat the test.
- A green run is not proof of "race-free".
Anti-patterns (race-guard may deny): retry flags, fixed order.

## Worker mode (`PHP-CONC-02`, `-03`, `PHP-PERF-05`)
- Static properties, globals, singletons, in-memory caches, `ini_set`, `chdir`, timezone, `setlocale`, seeded RNG and the default stream context persist across requests.
- Reset engine state per loop: `error_clear_last()`, `libxml_clear_errors()`, `http_clear_last_response_headers()` (8.4+). Symfony request-scoped services implement `ResetInterface`.
- Octane: never inject the container, `Request` or config repository into a singleton constructor; pass request data as arguments or resolve lazily; never append to a static array.
- `--max-requests` (default 500) is a safety net, not a fix.
- Required smoke test: two requests through one worker, asserting no data from the first appears in the second.

## Async (`PHP-CONC-05`)
Inside the event loop never call blocking functions (`sleep`, `file_get_contents` on URLs, sync PDO/curl). Use non-blocking clients, timeouts and cancellation. Swoole coroutines must not share one PDO or Redis connection: pool them.

## Background jobs (`PHP-CONC-06`)
Messenger (`messenger:consume`) or Laravel queues/Horizon under Supervisor/systemd/k8s. Idempotent jobs; `--time-limit`/`--memory-limit`/`--limit` so workers recycle; handle SIGTERM.

## Tests share nothing (`PHP-CONC-07`)
Reset static state, a transaction or schema per test or process, no dependence on execution order.
