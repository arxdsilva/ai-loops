---
name: testing
description: Use when writing, reviewing or running backend tests in any language pack - unit, table or parametrised, integration, fuzz, benchmark, race-policy and regression tests, or deciding how to test a change without sleeps or real time.
---

# Testing and benchmarking

## Load

- Your language: `skills/lang-<id>/testing.md` and `concurrency.md` (load `loops:lang-<id>`), `specs/languages/<id>/RULES.md` TEST section. Go: `specs/languages/go/RULES.md §30` testing, `§31` benchmarks.
- `specs/ARCHITECTURE.md §24` testing architecture.
- Gate checks and the race policy: `specs/WORKFLOW.md §6`.

## Always the pack's race policy

Every test run follows the pack's race policy (WORKFLOW §6): the race flag (`flag` packs: Go `-race`, Swift `--sanitize=thread`), the race build (`build`: the C++ TSan preset), or the substitute checks (`none`: randomised order with a printed seed, parallel runs, repeats of the changed tests). `loops lang show <id>` shows `race`. The answer to a slow suite is cheaper tests, never a weaker policy: no retries, no fixed order. Agents check their scope with `loops gate --only <check>,… --pkgs <owned>`; the full suite is the lead's, once per run (WORKFLOW §6).

## Pyramid

- Unit tests for pure rules (domain).
- Application tests with fake outbound ports.
- Adapter tests per technology: the framework's in-process test client for handlers (Go `httptest`), a real database (container) for repositories, a fake server for client contract tests.
- Integration tests across real boundaries only where needed.
- End-to-end checks against the running service (the product review).
- Fuzz tests for parsers, decoders, migrations, validators and quantity/money arithmetic.
- Benchmarks for hot paths; soak tests for leaks and invariants where the service is long-running.

## Behaviour, not trivia

A test that would still pass with every function under test returning nothing is hollow: QA fails it (`specs/ENGINEERING.md §12`), and the quality pass's mutation testing finds it (`loops:quality`). See every new test fail for the reason you expect before you make it pass (§7). Name tests for behaviour (`refund rejects an already refunded order`; Go `TestRefund_RejectsAlreadyRefundedOrder`), not tests tied to private helper call counts. One reason to fail per test.

## Table and parametrised tests

Use when many inputs share one behaviour and setup (Go tables, pytest `parametrize`, JUnit `@ParameterizedTest`). Do not force rows with unrelated setup into a table. Each case is named; parallel cases share no mutable state.

## No sleeps, injected time

Inject the `Clock`; advance it in the test. Wait on completion signals (channels, futures, latches) or `Eventually` helpers with a deadline, never a sleep. Randomness comes from a seeded source. Tests must not depend on map order, wall clock, ports in use or the network.

## Headless

Domain and application tests need no network, database, filesystem or listener. Config and fixture validators run without starting the service.

## Fuzz and properties

Fuzz round trips (encode/decode, migrate old to new), parsers and validators; assert invariants (no panic, bounded output, idempotent apply). Commit interesting corpus entries where the pack's fuzzer reads them (Go: `testdata/fuzz`).

## Benchmarks

Representative scale (`N` realistic, not toy). Report allocations where the harness can; separate setup from timed work; keep results alive so the work is not optimised away. Compare runs statistically (`skills/performance`; the harness per pack is in `skills/lang-<id>/testing.md`).

## No-flap and consistency tests

Threshold-driven behaviour ships a no-flap test (`skills/jobs-and-state-machines`). Consistency-sensitive changes add repeat-run, duplicate-delivery and migration-fixture tests (`skills/consistency`).

## Regression rule

Every bug fix ships with a regression test that fails before the fix and passes after it, in the same change (TESTING §1). The five steps that get you there are in `skills/debugging`. Never weaken or delete a test to get green; if a test is wrong, say so in the claims report.

## Coverage (hard gate)

New code ships with unit tests. `loops gate` has a required `coverage` check after the pack's test checks: new-code coverage against the base branch (`--base <ref>`) must be at least 70% (WORKFLOW §6). `gate.coverage_min` may only raise that; `gate.coverage_exclude` adds exclusions to the pack's `coverage.exclude` (Go: `cmd/**`, `internal/compose/**`, `testdata`) and generated files. Only the pack's `source_globs` minus its `test_globs` count. In `flag` packs the race-guard hook denies a test command without the race flag. Coverage prerequisites (a coverage plugin or config) are set up by the `<id>-engineer` in wave 0; the language skill names them.

Reading the coverage table: each row is a changed package, module or file with its new-code percentage. The check fails when a row is under 70%, when a changed file's new lines are all untested, or (Go) when a changed package has no tests. Fix by adding tests for the listed uncovered `file:line` ranges, never by excluding code or lowering the bar. Scoped run for one agent: `loops gate --only <the pack's test checks>,coverage --pkgs <owned>` (Go: `--only test,coverage`). Coverage is still evidence for claims, not a goal in itself: assert behaviour.

## Per language

The test runner, coverage format, race mode and substitute of each pack live in `specs/languages/<id>/pack.json` (`loops lang show <id>`) and `skills/lang-<id>/testing.md`. `loops lang list` shows every pack's race mode and coverage format. A language with no pack has no gate: say so in the claims report.

## Common rationalizations

The rows are binding (`ROLES.md` "Common rationalizations").

| Excuse | Why it's wrong | What to do instead |
|---|---|---|
| "I'll add the tests later." | Strict TDD puts the test first, and it must show its red (ENGINEERING §7). | Write the failing test now and record it under `## Red first`. |
| "This change is too small to test." | Triage skips steps, never the gate or its coverage floor (WORKFLOW §2.8, TESTING §8). | A small code change gets a small test. |
| "The race detector is slow." | The race policy has no opt-out (TESTING §8.2). | Make the tests cheaper and scope the run with `--pkgs <owned>`. |
| "Coverage is already high." | 70% is a floor on new code, and a hollow test covers lines too (ENGINEERING §12). | Assert behaviour on every new path. |
| "It's flaky; a retry will settle it." | A flaky test is a bug, and the gate never retries (TESTING §9). | Fix its isolation or its race in this change. |
| "A short sleep makes it pass." | A test controls time; sleeps hide ordering bugs (TESTING §6). | Inject the clock and wait on a completion signal. |
| "This code can't be tested honestly." | A dishonest test written to reach the floor is worse than none (ENGINEERING §7). | Move the decision into a pure function behind its boundary and test that. |
| "The test is wrong, so I'll loosen it." | Never weaken or delete a test to get green (TESTING §9). | Say so in the claims report; the lead decides. |
