# PHP idioms

Digest of RULES IDIOM.

## Types and errors
- `declare(strict_types=1);` in every file, enforced by PHP-CS-Fixer `declare_strict_types` (`PHP-IDIOM-01`).
- Throw exceptions; never return `false`/`null` as an error. Wrap with `previous: $e`; one hierarchy rooted at an app base class; never a bare `catch (\Throwable)` without rethrow or log.
- No `@` suppression; turn warnings into `ErrorException`; `error_reporting=E_ALL` (`-02`).
- Type every parameter, return and property; `mixed` only on purpose. PHP has no runtime generics: document `list<Foo>`, `array{id:int}`, `@template` in PHPDoc for PHPStan (`-06`).
- Nullable explicitly: `?string $x = null` (implicit null is deprecated in 8.4); `?->`, `??`, `??=`; Null Object over returned `null` (`-03`).

## Values and modelling
- `final readonly class` value objects with promoted constructor properties (8.2+); `public private(set)` in 8.4+; `clone($this, [...])` with-ers in 8.5 (`-04`).
- Backed `enum`s instead of constants and magic strings; `tryFrom()` at the boundary; `match` instead of `switch` (strict, exhaustive) (`-05`).
- First-class callables `strlen(...)` over string callables; property hooks only for trivial validation (`-12`).
- Attributes for metadata (`#[Route]`, `#[ORM\Entity]`); `#[\Override]` (8.3+), `#[\Deprecated]` (8.4+), `#[\NoDiscard]` (8.5) (`-10`).
- Avoid `eval`, `extract`, `$$x`, undocumented `__call`/`__get` magic (`-11`).

## Comparison, time, text, money
- `===`/`!==`; `in_array($x, $l, true)`; validate numeric strings explicitly (`-07`).
- `DateTimeImmutable` in UTC, ISO-8601 across boundaries, "now" from a PSR-20 `Clock`; `mb_*` and UTF-8; integer minor units or `brick/money`/bcmath, never `float` (`-08`).

## Resources and naming
- `try/finally` for files, locks, transactions; do not rely on `__destruct`; workers avoid growing statics (`-09`).
- PER-CS naming: `StudlyCaps` classes, `camelCase` members, `UPPER_SNAKE` constants; one class per file (`-10`).
- PHPDoc only where the native type cannot say it (`PHP-DOC-01`).
