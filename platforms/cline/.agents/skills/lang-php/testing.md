# PHP testing and coverage

Digest of RULES TEST. Neutral testing rules: `skills/testing`, `specs/ARCHITECTURE.md` (testing architecture).

## Stack and layout (`PHP-TEST-01`, `-02`)
PHPUnit 12/13 (Pest 4 on top of it is acceptable, one per repo, `-11`). Framework kits for HTTP: Symfony `WebTestCase`/`KernelTestCase`, Laravel `Tests\TestCase`. Browser E2E stays out of the unit gate.
`tests/Unit`, `tests/Integration`, `tests/Functional` (Symfony) or `tests/Feature` (Laravel), mirroring `src/`. Files `*Test.php`; tests never in the PSR-4 `autoload` map.

## Style (`-03`, `-04`)
`#[DataProvider]` with a static public method returning named cases (Pest `->with([...])`); AAA; names describe behaviour. Hand-written fakes (in-memory repository, fixed `Clock`) over mocks; mocks only at true boundaries, never the class under test, value objects or the ORM.

## Integration and DB isolation (`-05`)
Real database (container or compose service); SQLite only if production is SQLite. Isolate per test with transaction rollback (`DatabaseTransactions`, DAMA DoctrineTestBundle) or `RefreshDatabase`. Under paratest put `TEST_TOKEN` in the database name.

## Coverage setup (`-06`, `-07`)
1. Driver: `pecl install pcov` (line only) or Xdebug 3 (branch and path coverage need Xdebug). The gate sets `XDEBUG_MODE=coverage`; with PCOV set `pcov.enabled=1`.
2. `phpunit.xml.dist`:
```xml
<source><include><directory>src</directory></include></source>
```
plus `failOnWarning="true"`, `failOnRisky="true"`, `beStrictAboutOutputDuringTests="true"`, `beStrictAboutTestsThatDoNotTestAnything="true"`, `executionOrder="random"`, `cacheDirectory=".phpunit.cache"`.
3. Report: `--coverage-cobertura <file>`; the gate passes the run's `{coverage}` path. Paths are relative to a `<sources>` root.
4. Exclude generated code with `<source><exclude>` or `#[CodeCoverageIgnore]`; `--disable-coverage-ignore` in CI.
5. New-code coverage must be at least 70%; Pest `--min=N` is total coverage only. Missing driver = gate failure with the snippet above (D12/D13).

## Flaky signals (`-09`)
`--fail-on-warning --fail-on-risky` (and `--fail-on-deprecation` when the project allows), `--order-by=random --random-order-seed=<n>` (record the seed), inject `Clock`, `Carbon::setTestNow`/`ClockMock`, seeded RNG. Never add retries or `--order-by=default` to hide order dependence (`PHP-CONC-04`). Reset static state between tests (`Mockery::close()`, clear singletons).

## Iterating (`-10`)
`vendor/bin/phpunit --filter=FooTest::testBar`, `--group`, `--testdox`, `--stop-on-failure`, `--order-by=defects`; Laravel `php artisan test --parallel`; Pest `--parallel`. Targeted runs while iterating; the lead and QA run the full gate (`loops gate`).

## Extras (`-08`)
`--strict-coverage` for libraries only; Infection (`--min-msi=70`) on domain code; property tests with `eris/eris`.
