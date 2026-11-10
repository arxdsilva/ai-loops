# PHP architecture

Digest of RULES ARCH. The neutral rules are `specs/ARCHITECTURE.md` and `specs/SOLID.md`; this maps them to PHP.

## Modules and layers (`PHP-ARCH-01`, `-02`)
Modular monolith first: `src/<Module>/{Domain,Application,Infrastructure,Http}`. Extract a service only along a boundary with no shared tables.
`Http`/`Console` -> `Application` -> `Domain`; `Infrastructure` implements Application ports and may use Domain. Domain depends on PHP core and other domain code only.

## Mapping imports (`PHP-ARCH-03`)
PSR-4: one class per file, path = FQCN minus the prefix. `loops arch` reads `use A\B\C;`, `use function`, `use const` and resolves them through the `composer.json` psr-4 map. Fully-qualified names in code count too (review them).

## Enforcing (`PHP-ARCH-04`, `-05`)
PHP has no package-private and allows namespace cycles, so tooling decides. `loops arch` (regex scanner) covers purity and direction. When `deptrac.php` (or the older `deptrac.yaml`) is committed, the `deptrac` check runs `vendor/bin/deptrac analyse` too; fail on violations and unassigned classes. Alternatives: PHPat, Arkitect (project's choice).

## Composition root and DI (`PHP-ARCH-06`)
Constructor injection; the framework container is wired only at the root (Symfony autowire and `services.yaml`, Laravel providers). Never inject the container, call `app()`, `Container::get()` or facades in Domain or Application: a service locator hides dependencies and breaks worker mode.

## Purity (`PHP-ARCH-07`)
Domain forbids (pack `arch.forbidden_in_domain`): `Illuminate\`, `Symfony\Component\HttpFoundation\`, `Symfony\Component\HttpKernel\`, `Doctrine\ORM\`, `Doctrine\DBAL\`, `PDO`, `Psr\Http\Message\`, `GuzzleHttp\`.
Impure calls (pack `arch.impure_calls`): `new DateTime(`, `new DateTimeImmutable(`, `date(`, `time(`, `microtime(`, `rand(`, `mt_rand(`, `random_int(`, `uniqid(`, `getenv(`, `$_ENV`, `$_SERVER`, `file_get_contents(`, `curl_init(`. Inject a PSR-20 `ClockInterface`, an id/random port and typed config instead.

## Ports (`PHP-ARCH-08`)
Small interfaces in the consuming layer (`Application\Port\Clock`), named by capability. Prefer PSRs for cross-cutting seams: PSR-3 log, PSR-6/16 cache, PSR-7/17/18 HTTP, PSR-11 container, PSR-14 events, PSR-20 clock.

## Sizing (`PHP-ARCH-09`)
Classes at most 300 lines, methods 30, constructors 4 params (else a value object). `final` by default; composition over `extends`; interfaces over traits for behaviour.

## Frameworks (`PHP-ARCH-10`, `PHP-FW-02`)
Laravel: no `User::find()`/`->save()` in Domain or Application; wrap Eloquent in a repository adapter that maps to domain objects; treat `app/Models` as infrastructure. Symfony: keep Doctrine mapping out of the domain (attributes only, or XML), flush only in an application unit of work. Resolve conflicts by placing adapters under `Infrastructure/`, not by fighting the skeleton. Log the deviation (`loops run override`).
