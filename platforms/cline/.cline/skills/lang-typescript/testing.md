# TypeScript testing digest

Rules: RULES.md TEST. Universal: `specs/TESTING.md`; `skills/testing` for TDD flow. Race policy: SKILL.md §4.

## Runner (TS-TEST-01, 02)
- Default vitest (ESM and TS native). Jest 30 for existing suites (NestJS default). `node:test` for zero-dependency libraries. Bun: `bun test`. Deno: `deno test`. E2E: supertest, Fastify `app.inject()`, Hono `app.request()`, Playwright for browsers.
- Test files: `*.test.ts`, `*.spec.ts`, `*.e2e-spec.ts`, `__tests__/**`, `test/**` (and JS twins). The test guard recognises these globs.

## Style (TS-TEST-03, 04, 10)
- `describe` / `it`, `it.each` tables, arrange-act-assert, names like `it('rejects an expired token')`.
- Prefer fakes of ports (in-memory repo, fake clock) over module mocking. `vi.fn` / `vi.spyOn` for interaction checks, `vi.useFakeTimers()` for time, `msw` or undici `MockAgent` for HTTP. Avoid `vi.mock` of internal modules: it couples tests to file layout.
- Isolation: no shared module state; keep `isolate: true` and the `forks` pool; `restoreMocks`, reset timers; control `Date` and randomness through injected ports.
- `testTimeout` and `hookTimeout` explicit; jest `--detectOpenHandles`; `why-is-node-running` for hangs (TS-TEST-11).

## Integration (TS-TEST-05)
Testcontainers for Node with a real Postgres, Redis or Kafka. One schema or transaction per test (or `TRUNCATE`), migrations applied once in a global setup. Do not mock the database.

## Property and mutation (TS-TEST-09)
fast-check for parsers, money math, state machines and codecs (`fc.assert(fc.property(...))`, seed with `fc.configureGlobal({seed})`). Stryker on the domain package only, nightly.

## Fast loop (TS-TEST-12)
`vitest run path/to/file.test.ts -t "name"`; `vitest related <file>`; `vitest run --changed <base>`. CI uses `vitest run` (never watch) and `--shard=i/n`. Plain `vitest` in a TTY watches, which hangs a gate.

## Coverage (TS-TEST-06 to 08)
- vitest: `@vitest/coverage-v8`; `vitest run --coverage --coverage.provider=v8 --coverage.reporter=lcov --coverage.reportsDirectory=<dir>` writes `<dir>/lcov.info` (lcov carries lines and branches). The v8 provider uses AST remapping since vitest 3.2. Untested files appear only with `coverage.include` (for example `['src/**/*.ts']`).
- Exclude `**/*.d.ts`, `**/generated/**`, `**/*.config.*`, `**/migrations/**` via `coverage.exclude`. Generated header marker: `@generated` or `// This file was automatically generated`.
- Alternatives: jest `--coverage --coverageReporters=lcov --coverageDirectory=<dir>`; `node --test --experimental-test-coverage --test-reporter=lcov --test-reporter-destination=<file>`; Bun `bun test --coverage --coverage-reporter=lcov --coverage-dir=<dir>`; Deno `deno test --coverage=cov && deno coverage cov --lcov`.
- The gate computes new-code coverage itself: lcov `DA:` lines of changed non-test source files, at least 70%. `coverage.thresholds` is repo-wide and does not count.

## Race substitute (TS-CONC-04, 05)
Shuffle files and tests, concurrent, a printed seed, `--repeats=3` on changed files, no retries: details in SKILL.md §4 and `concurrency.md`.

## Deno (no gate variant)
Detected by `deno.json`; the gate does not cover it. Override in project config: `deno test --coverage=cov` then `deno coverage cov --lcov --output=<dir>/lcov.info`, `deno fmt --check`, `deno lint`, `deno check`. Log it with `loops run override`.
