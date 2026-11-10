# PHP frameworks

Digest of RULES FW. The framework conventions in the project win; log overrides.

## Landscape (`PHP-FW-01`, as of 2026-11)
Laravel 13 (PHP 8.3-8.5; security to 2028-03-17; 12 is security-only to 2027-02-24). Symfony 8.1 stable, 7.4 LTS (security to Nov 2029); 6.4 LTS bug fixes end Nov 2026. Also Slim, Mezzio, API Platform, CodeIgniter.

## Follow the skeleton (`PHP-FW-02`)
- Laravel: `app/`, `routes/`, `database/migrations`, `tests/{Unit,Feature}`, Eloquent, facades at the edge.
- Symfony: `src/`, `config/`, `templates/`, `tests/`, `var/`, autowiring, Doctrine attributes, `bin/console`.
- Hexagonal conflicts (Active Record, facades, `app/Models`) are solved by putting adapters under `Infrastructure/`, not by restructuring the framework. Domain code stays framework-free (`loops arch`).

## Laravel footguns (`PHP-FW-03`)
`env()` outside config after `config:cache`; `$guarded = []`; lazy-loading N+1; facades and `app()` in the domain; static or singleton state under Octane; heavy work in provider `boot`; stale `bootstrap/cache` after deploy (`php artisan optimize` / `optimize:clear`). Pint default preset is `laravel`, not PER: the project's `pint.json` wins. Tests: `php artisan test` (and `--parallel`).

## Symfony footguns (`PHP-FW-04`)
Public services and `$container->get()`; missing `ResetInterface` on stateful services in worker mode; scattered `flush()`; `APP_ENV=dev` in production; stale `var/cache` (`cache:clear --env=prod --no-debug`, `cache:warmup`); entities with framework attributes leaking into the domain. `APP_CACHE_DIR`/`APP_BUILD_DIR` are honoured by `MicroKernelTrait` (the pack scopes `APP_CACHE_DIR` per run).

## Worker servers
FrankenPHP worker, RoadRunner, Swoole and Octane: adopt only with the state-leak rules and a two-request leak test (`concurrency.md`); set `--max-requests`.

## Extensions for static analysis
`larastan/larastan`, `phpstan/phpstan-symfony`, `phpstan-doctrine`, `phpstan-strict-rules`, `phpstan-deprecation-rules`: framework magic is invisible to PHPStan without them (`PHP-TOOL-05`).
