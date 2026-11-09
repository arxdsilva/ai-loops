---
name: testing
description: Use when writing, reviewing or running Go tests - unit, table, integration, fuzz, benchmark, race, regression tests for a backend service, or deciding how to test a change without sleeps or real time.
---

# Testing and benchmarking

## Load

- `specs/GO_BEST_PRACTICES.md §30` testing, `§31` benchmarks.
- `specs/HEXAGONAL_ARCHITECTURE.md §24` testing architecture.
- Gate commands: `specs/WORKFLOW.md §6`.

## Always `-race`

Every `go test` runs with `-race` (locally, CI, agents, one-off runs). The answer to a slow suite is cheaper tests, never dropping the flag. Agents run targeted packages with `GOFLAGS=-p=2 go test -race -count=1 -cover ./<owned>/...`; the full suite is the lead's and QA's (`ROLES.md`).

## Pyramid

- Unit tests for pure rules (domain).
- Application tests with fake outbound ports.
- Adapter tests per technology: `httptest` for handlers, a real database (container) for repositories, a fake server for client contract tests.
- Integration tests across real boundaries only where needed.
- End-to-end checks against the running service (the product review).
- Fuzz tests for parsers, decoders, migrations, validators and quantity/money arithmetic.
- Benchmarks for hot paths; soak tests for leaks and invariants where the service is long-running.

## Behaviour, not trivia

Name tests for behaviour: `TestRefund_RejectsAlreadyRefundedOrder`, not tests tied to private helper call counts. One reason to fail per test.

## Table tests

Use when many inputs share one behaviour and setup. Do not force rows with unrelated setup into a table. Subtests name each case; parallel subtests share no mutable state.

## No sleeps, injected time

Inject the `Clock`; advance it in the test. Wait on channels, `sync.WaitGroup` or `Eventually` helpers with a deadline, not `time.Sleep`. Randomness comes from a seeded source. Tests must not depend on map order, wall clock, ports in use or the network.

## Headless

Domain and application tests need no network, database, filesystem or listener. Config and fixture validators run without starting the service.

## Fuzz and properties

Fuzz round trips (encode/decode, migrate old to new), parsers and validators; assert invariants (no panic, bounded output, idempotent apply). Commit interesting corpus entries under `testdata/fuzz`.

## Benchmarks

Representative scale (`N` realistic, not toy). Call `b.ReportAllocs()`; separate setup from timed work (`b.ResetTimer()`); keep results alive. Compare with `benchstat` (`skills/performance`).

## No-flap and consistency tests

Threshold-driven behaviour ships a no-flap test (`skills/jobs-and-state-machines`). Consistency-sensitive changes add repeat-run, duplicate-delivery and migration-fixture tests (`skills/consistency`).

## Regression rule

For a bug: reproduce; add a failing test or fixture; fix; verify the test passes; keep the regression artifact. Never weaken or delete a test to get green; if a test is wrong, say so in the claims report.

## Coverage (hard gate)

New code ships with unit tests. `loops gate` has a required `coverage` check after `test`: new-code coverage against the base branch (`--base <ref>`) must be at least 70%. `gate.coverage_min` may only raise that; `gate.coverage_exclude` adds exclusions to the defaults (`cmd/**`, `internal/compose/**`, `testdata`, generated files). The race-guard hook denies any `go test` without `-race`.

Reading the coverage table: each row is a changed package or file with its new-code percentage. The check fails when a row is under 70%, when a changed package has no tests, or when a changed file's new statements are all untested. Fix by adding tests for the listed uncovered `file:line` ranges, never by excluding code or lowering the bar. Scoped run for one agent: `loops gate --only test,coverage --pkgs <owned>`. Coverage is still evidence for claims, not a goal in itself: assert behaviour.

## Other languages

Conventions the agents follow; the loops gate automates Go only. Minimum is 70% of new code.

- Rust: `cargo test`; race via `loom` or `RUSTFLAGS=-Zsanitizer=thread` on nightly; coverage via `cargo-llvm-cov`.
- Python: `pytest` with `pytest-cov --cov-fail-under=70`; shake out races with `pytest-xdist` and `pytest-repeat`.
- TypeScript/JS: `vitest` or `jest --coverage` with thresholds of 70.
- Java/Kotlin: JaCoCo minimum 0.70; `jcstress` for concurrency.
- C/C++: `-fsanitize=thread` plus `gcov` or `llvm-cov`.
