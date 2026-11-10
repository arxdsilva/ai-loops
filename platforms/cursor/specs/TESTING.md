# Testing

The universal test rules, and the hard test gate every language pack enforces. Each rule is stated once here; a pack's `specs/languages/<id>/RULES.md §TEST` and `§CONC` add only its mechanics (runner, flags, files), and `skills/testing` is the how-to. Cite as `TESTING §8`.

## 1. Test pyramid and ownership

| Level | Covers | Doubles | Owner (`ROLES.md`) |
|---|---|---|---|
| Unit | domain rules, application use cases | none in domain; fakes at ports in application (§4) | the layer's engineer |
| Adapter | one technology translation: handler, repository, client | a real dependency (§5) or a fake server | the adapter's engineer |
| Integration | real adapters wired together, where needed | real dependencies in containers | test-engineer |
| End-to-end | the running service through its real interface | none | test-engineer; product-reviewer's proofs |

Most tests are unit tests. A bug fix starts with a failing test at the lowest level that reproduces it. Layer rules: `specs/ARCHITECTURE.md §24`.

## 2. Recognition

Tests live where the ecosystem expects them, and loops recognises them by the pack's `test_globs` (`*_test.go`, `test_*.py`, `*.spec.ts`, `src/test/java/**`, …). Source files are the pack's `source_globs` minus its `test_globs` and generated files (`generated.globs`, `generated.header_regex`). A test file outside the globs counts as source, so it lowers coverage instead of raising it.

## 3. Style

- Names describe behaviour: `Refund rejects an already refunded order`, `Retry stops after max attempts`, in the runner's naming form.
- Arrange, act, assert; one behaviour per test.
- Parametrised, table or data-driven tests when cases share structure. Not for rows that need unrelated setup.
- Assert on observable results and errors, not on private helpers or call counts.

## 4. Doubles: fakes at ports

- Outbound ports get in-memory fakes: store, deterministic clock, scripted gateway, recording publisher. Every port has one (`specs/ARCHITECTURE.md §29`).
- Do not mock what you own. Prefer real domain objects to doubles of them.
- A mocking library is for third-party boundaries a fake can't reach; it never replaces the domain.

## 5. Integration with real dependencies

Adapter and integration tests run against the real technology (Testcontainers or the ecosystem's equivalent, a documented fixture otherwise), with isolation per test: a fresh schema, a transaction rolled back, or unique keys. They never share state across tests.

## 6. Determinism

- Time, randomness and IDs are injected (`specs/ARCHITECTURE.md §20`). No sleeps; a test controls time.
- Tests run in random order where the runner supports it, and the seed is printed so a failure can be replayed.
- Tests are parallel-safe: no shared mutable state, no fixed ports or temp paths, no dependence on order.
- No network, filesystem or wall clock in domain or application tests.

## 7. Property and fuzz tests

Where they pay: parsers, decoders, codecs, migrations, validators, money and quantity arithmetic, state machines, DTO round trips. A found counterexample becomes a regression test.

## 8. The hard gate

Both parts are hard for every pack, in every run, and checked by `loops gate` (WORKFLOW §6). There is no opt-out: the fix is a cheaper test, never a weaker rule.

### 8.1 At least 70% of new code is covered

New code is the set of changed lines against the git base (`--base <ref>`, else `gate.base`, else the merge base with `origin/main`) in candidate files. A candidate:

- matches the pack's `source_globs`;
- is not in its `test_globs`;
- is not generated;
- is outside the pack's `coverage.exclude` and the config's `gate.coverage_exclude` (config only adds exclusions).

The pack's `produced_by` checks write a coverage report; loops parses it, normalises it and judges the changed lines itself (stdlib only; no diff-cover):

| `coverage.format` | Packs | Normalisation |
|---|---|---|
| `gocover` | go | strip the module path from file names (`go.mod`); statements become line ranges |
| `lcov` | typescript, cpp, swift, rust, elixir, clojure, ruby | `SF:` paths made repo-relative (absolute and `./` prefixes stripped); line hits |
| `cobertura` | python, csharp, php, scala, erlang | `filename` resolved against `<sources>`; several files (one per project or app) merged |
| `jacoco` | java, kotlin | package name + source-file name resolved against the source roots; one file per module, merged |

- Granularity is normalised to lines, whatever the tool reports (statements, branches, forms).
- The gate fails when covered new lines are under 70% of the instrumented new lines, and when a changed file's new lines are all untested. It lists the uncovered `file:line` ranges.
- "A changed unit has no tests" is checked where the pack can map a source unit to its tests: today the Go package (TOOLING D7).
- The floor is 70%. `gate.coverage_min` in config may only raise it.
- Missing coverage output fails the check; it is never skipped.

### 8.2 Race policy

Every test run follows the pack's race policy (TAXONOMY CONC-04):
- `flag`: every test command carries the race flag, and the race-guard (the `test-guard` hook) denies a test command without it.
- `build`: the full gate runs the pack's race build (its `race: true` test check), and it must pass.
- `none`: the full gate runs the pack's substitute checks (its `race: true` checks), and they must pass. The pack states, and the gate records:
  - the seed of every randomised run;
  - the parallelism used;
  - the repeat count applied to the changed tests.

  A pack whose substitute is static (Rust) names the lint checks that carry it.

A required race or substitute check that is skipped fails the full gate, as a skipped required check does today. There is no opt-out: the fix is a cheaper test, never a weaker policy.

The checks that carry the policy are the pack's `race.checks`. If the policy can't run on the host (Go without cgo, TSan unsupported on the platform), the gate fails with "race policy unavailable: …" instead of skipping. Language-specific stress tools (jcstress, Lincheck, loom, proper_statem) are review items, not gate checks.

| Mode | Packs |
|---|---|
| `flag` | go (`-race`), swift (`--sanitize=thread`) |
| `build` | cpp (a TSan preset) |
| `none` + substitute | python, typescript, java, csharp, php, ruby, kotlin, rust, scala, elixir, erlang, clojure |

## 9. Flakiness

- The gate never retries a failed test. Retry flags (`--retry.count`, rerun-on-failure plugins) are off in the pack's checks and must stay off in the project's config.
- A flaky test is a bug: fix its isolation (§6) or its race, in the same change that found it. Never quarantine it to get green.
- Never weaken or delete a test to make it pass.

## 10. TDD loop speed

- Write the failing test, make it pass, refactor.
- While iterating, run only your owned tests (the pack's test command narrowed to your scope, through `loops exec` or the hook-scoped command, WORKFLOW §6). The lead and QA run the full gate.
- Keep unit tests fast enough to run on every save: no containers, no network, no sleeps.
