---
name: lang-php
description: Use when writing, reviewing, testing or setting up PHP backend code in a loops run (the php language pack) - toolchain, layout, race policy, coverage, idioms and footguns.
---

# PHP pack

Rules: `specs/languages/php/RULES.md` (cite `PHP-<SEC>-NN`). Toolchain data: `specs/languages/php/pack.json` (`loops lang show php`). Architecture and SOLID are the neutral specs (`specs/ARCHITECTURE.md`, `specs/SOLID.md`); this skill only maps them onto PHP.

## When it applies
`loops lang detect` reports `php` (a `composer.json` at the root). PHP 8.4 or 8.5 for new code, 8.3 minimum (`PHP-TOOL-01`).

## Project rules first
Follow the project's own configs over the pack defaults: `phpstan.neon*` (level, baseline), `psalm.xml`, `.php-cs-fixer*.php`, `pint.json`, `phpcs.xml`, `phpunit.xml*`, `deptrac.*`, `rector.php`, and the framework skeleton (Laravel `app/`, Symfony `src/`). A PER-CS or PSR-12 ruleset is the project's choice. Log every override with `loops run override` (`specs/WORKFLOW.md` §1).

## The gate
Agents run `loops gate` (or `loops gate --only <check>` while iterating). Checks, in order, from `pack.json`:

| Check | Command | Runs when |
|---|---|---|
| `composer-install` (deps) | `composer install --no-interaction --prefer-dist --no-progress` | `composer.lock` exists; never `composer update` |
| `composer-validate` | `composer validate --strict --no-interaction` | always (required) |
| `php-cs-fixer` | `vendor/bin/php-cs-fixer check --diff --using-cache=no --no-interaction` (exit 8 = needs fixing) | `.php-cs-fixer(.dist).php` exists (required then) |
| `pint` | `vendor/bin/pint --test` | `pint.json` exists |
| `phpstan` | `vendor/bin/phpstan analyse --no-progress --no-interaction --memory-limit=1G` | `phpstan.neon(.dist)` exists |
| `arch` | `loops arch` (regex over `use` + PSR-4) | optional |
| `deptrac` | `vendor/bin/deptrac analyse` | `deptrac.php`/`deptrac.yaml` exists |
| `test` | `vendor/bin/phpunit --order-by=random --random-order-seed=<seed> --fail-on-warning --fail-on-risky` (+ coverage), twice | always (required) |
| `test-parallel` | `vendor/bin/paratest --processes=2 --order-by=random --fail-on-warning --fail-on-risky` | paratest installed, else a logged serial fallback |
| `coverage` | new-code coverage from Cobertura | always, not skippable |
| `composer-audit` | `composer audit --locked --format=plain` | off by default; `--vuln` |

The edit hook runs `vendor/bin/php-cs-fixer fix --using-cache=no --no-interaction <file>` after each PHP edit when a config exists. Do not run `composer update`, and do not regenerate `phpstan-baseline.neon` to silence errors (`PHP-TOOL-05`).

## Race policy
PHP has no race detector (`race.mode: none`, `specs/WORKFLOW.md` §6). The substitute is part of the full gate and every check in it is required:
- `test` runs the whole suite serially under two different random seeds (`repeat: 2`, the seed is printed and recorded), with `--fail-on-warning --fail-on-risky`.
- `test-parallel` runs it once under `paratest --processes=2`. Each process gets `TEST_TOKEN`/`UNIQUE_TEST_TOKEN`; put it in the database name (`PHP-TEST-05`).
- A failure that appears only under another seed or in parallel is a real order or shared-state bug: fix the test or the code. Never add a retry, `--no-random`, or a fixed `--order-by=default` (`PHP-CONC-04`, `PHP-CONC-07`). Do not claim "race-free" from a green run.
- Worker-mode apps (FrankenPHP worker, RoadRunner, Swoole, Laravel Octane) also need a smoke test that sends two requests through one worker and asserts no state leaks (`PHP-CONC-02/03`).
- If the pack cannot run the substitute (no PHP, no coverage driver) the gate fails with a preflight message; it never skips.

## Coverage recipe
Format `cobertura`, written to the run's `{coverage}` path by `--coverage-cobertura <file>` (D6). Loops needs:
- a coverage driver: `pcov` (`pecl install pcov`, line coverage only) or Xdebug 3. The test check sets `XDEBUG_MODE=coverage`; with PCOV set `pcov.enabled=1`. Without a driver PHPUnit warns "No code coverage driver available" and the gate fails (D12).
- `<source><include><directory>src</directory></include></source>` in `phpunit.xml.dist`; PHPUnit calls source configuration mandatory for coverage (`PHP-TEST-06`).
- New-code coverage at least 70% (`PHP-TEST-07`); the gate reads the git diff and the report itself.

These are wave 0 prerequisites: the architect or `php-engineer` adds them (D13), logged as an override when the project had none. See `testing.md`.

## Layer map
```
src/
  <Module>/Domain/          pure: entities, value objects, rules, ports owned by Application; PHP core only
  <Module>/Application/     use cases, Port\ interfaces, transaction boundary
  <Module>/Infrastructure/  Doctrine/Eloquent repositories, HTTP clients, queue adapters
  <Module>/Http/ | Console/ controllers, commands: thin inbound adapters
config/  bootstrap or composition root: container wiring only
tests/{Unit,Integration,Functional|Feature}/   mirrors src/
```
Direction: `Http`/`Console` -> `Application` -> `Domain`; `Infrastructure` -> `Application` and `Domain` (`PHP-ARCH-02`). `loops arch` checks `use` statements against the PSR-4 map in `composer.json`; deptrac enforces the layers natively when `deptrac.php` exists. Domain forbids `Illuminate\`, HttpFoundation, Doctrine ORM/DBAL, PDO, PSR-7 and Guzzle, and the impure calls `new DateTimeImmutable(`, `time(`, `rand(`, `getenv(`, `$_ENV`, `file_get_contents(` (inject a PSR-20 `Clock`, an id port, config objects).

## Top 10 footguns
1. No `declare(strict_types=1);`: scalars coerce silently (`PHP-IDIOM-01`).
2. Loose `==` and `in_array` without `true`: use `===`, `hash_equals`.
3. `unserialize()` on user data: use `json_decode(..., JSON_THROW_ON_ERROR)` (`PHP-SEC-02`).
4. SQL by string concatenation: prepared statements, allow-listed identifiers (`PHP-SEC-01`).
5. State leaking across requests in worker mode: no request or container in singletons, `ResetInterface` (`PHP-CONC-02`).
6. N+1 through lazy relations: eager load, `preventLazyLoading()` (`PHP-DATA-04`).
7. Float money and mutable `DateTime`: integer minor units, `DateTimeImmutable` plus a `Clock` (`PHP-IDIOM-08`).
8. `env()` outside `config/*.php`, or `allow-plugins: true` (`PHP-OPS-01`, `PHP-REPO-07`).
9. Xdebug and dev dependencies in production: `composer install --no-dev --classmap-authoritative` (`PHP-OPS-02`).
10. Growing the PHPStan baseline or using `@`: only shrink the baseline, handle errors (`PHP-TOOL-05`, `PHP-IDIOM-02`).

## Load on demand
- `repo.md`: composer, lockfile, platform pin, ignore files, monorepos, CI order, versions.
- `architecture.md`: hexagonal on PSR-4, deptrac, DI, ports, framework conflicts.
- `idioms.md`: strict types, enums, readonly, match, null, time, money, errors.
- `testing.md`: PHPUnit/Pest layout, fakes, DB isolation, coverage driver setup, flaky signals.
- `concurrency.md`: the PHP models, worker-mode leaks, the substitute checks, queues, async.
- `security.md`: injection, unserialize, passwords, php.ini, supply chain, SAST.
- `data-api.md`: ORM and migrations, transactions, N+1, HTTP, resilience, logs, config, packaging.
- `frameworks.md`: Laravel and Symfony conventions and footguns, Octane and worker servers.
