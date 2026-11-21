# Python: idioms and performance

Digest of RULES `PY-IDIOM` and `PY-PERF`. Ruff rule families that enforce them are in brackets.

## Errors

- One exception hierarchy per package rooted at one base; narrowest `except`; chain with `raise X from err`; never bare `except:` or `except Exception: pass` [E722, BLE, TRY] (`PY-IDIOM-01`). Error taxonomy: `specs/ARCHITECTURE.md`.
- `CancelledError` is a `BaseException`: re-raise after cleanup, never catch `BaseException` [ASYNC, BLE] (`PY-IDIOM-02`).
- No `assert` for runtime validation; it is stripped under `-O` [S101] (`PY-IDIOM-15`).

## Types and data

- Absence is `T | None`, narrowed explicitly; return `None` only when "no result" is normal (`PY-IDIOM-03`).
- Never a mutable default argument [B006] (`PY-IDIOM-04`).
- `@dataclass(frozen=True, slots=True)` or `NamedTuple` for values and DTOs; Pydantic only at the edge (`PY-IDIOM-05`).
- Closed sets: `Enum`/`StrEnum`/`Literal`, `match`, and `assert_never` so new members fail type checking (`PY-IDIOM-06`).
- Avoid `Any`: `object`, generics, `Protocol`, `TypedDict`; cast once at an untyped boundary [ANN401] (`PY-IDIOM-07`).
- Define `__eq__` and `__hash__` together; never mutate dict keys or set members (`PY-IDIOM-11`).
- Money is `Decimal` or integer minor units, never `float` (`PY-IDIOM-10`).

## Time

Timezone-aware only: `datetime.now(UTC)`, store UTC, convert at the edge with `zoneinfo`; never `utcnow()` [DTZ]. Inject a clock (`specs/ARCHITECTURE.md`, `skills/consistency`) (`PY-IDIOM-09`).

## Resources and structure

- Every resource under `with`/`async with`; custom ones via `contextlib` [SIM115] (`PY-IDIOM-08`).
- Imports at module top, absolute within the package, no `import *`, no import-time side effects (I/O, config reads, threads) [F403, PLC0415] (`PY-IDIOM-14`).
- Avoid metaprogramming in business code (`setattr`, metaclasses, `exec`, monkey-patching); decorators and `__init_subclass__` are fine for cross-cutting concerns (`PY-IDIOM-13`).
- PEP 8 names, PEP 257 docstrings on public API [N, D] (`PY-IDIOM-12`). Comprehensions, `"".join`, `pathlib`, `zip(strict=True)` [PERF, PTH, B905] (`PY-IDIOM-16`).

## Performance

Measure first, tune second (`skills/performance`).
- Benchmark with `pytest-benchmark` or `pyperf`, never `time.time()` loops (`PY-PERF-01`).
- CPU: `py-spy` or `cProfile`; memory: `tracemalloc` or `memray` (`PY-PERF-02`).
- Runtime knobs are deliberate: `gc.freeze()` after preload, `-X importtime`, uvloop; JIT and tail-call builds are experimental (`PY-PERF-03`).
- Hot paths: no N+1, no per-request client construction, no `+=` string building, precompiled regexes, `set`/`dict` lookups, bounded `functools.cache` [PERF] (`PY-PERF-04`).
- CPU-bound pure Python goes to NumPy/Polars, a Rust/C extension or a process pool, not threads (`PY-PERF-05`).
