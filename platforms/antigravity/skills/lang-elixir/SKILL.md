---
name: lang-elixir
description: Use when writing, reviewing, testing or setting up Elixir backend code in a loops run (the elixir language pack) - toolchain, layout, race policy, coverage, idioms and footguns.
---
# lang-elixir

The Elixir pack: `specs/languages/elixir/pack.json` (data the gate reads) and `specs/languages/elixir/RULES.md` (rule IDs `ELIXIR-<SEC>-NN`, cited below). Elixir `1.20` on OTP 27-29, as of 2026-11.

## When it applies
`loops lang detect` reports `elixir` (a `mix.exs` at the root). Umbrella apps and Phoenix services are covered; see `frameworks.md`.

## Project rules first
The project's own config wins over every default here: `mix.exs` aliases and `cli/0`, `.formatter.exs`, `.credo.exs`, `.tool-versions`, CI files, `AGENTS.md`, Phoenix conventions. Read them before coding (`config_files` in the pack lists them). When you follow a project rule over a loops or pack rule, log it: `loops run override --rule "<loops rule>" --project "<project rule, file:line>" --note "<what you did>"` (WORKFLOW §1).

## Toolchain the gate runs
Agents run `loops gate` (or `--only <check>` while iterating). The checks, in order (pack `checks`), run with `MIX_ENV=test`:

| Check | Command | Required |
|---|---|---|
| deps | `mix deps.get --check-locked` | yes |
| compile | `mix compile --warnings-as-errors` | yes |
| format | `mix format --check-formatted` | yes when `.formatter.exs` exists |
| credo | `mix credo --strict` | no; only with `.credo.exs` |
| xref-cycles | `mix xref graph --format cycles --fail-above 0` | no; off by default (enable per project) |
| dialyzer | `mix dialyzer` | no; only with `.dialyzer_ignore.exs` or `priv/plts` |
| test | `mix coveralls.lcov --warnings-as-errors` | yes |
| test-repeat | `mix test --repeat-until-failure 5 --warnings-as-errors` | yes (race substitute) |
| coverage | loops reads `cover/lcov.info` | yes |
| deps-audit, hex-audit, sobelow | `mix deps.audit`, `mix hex.audit`, `mix sobelow --exit medium` | off by default (`--vuln`) |

Iterate narrowly: `mix test path/to/file_test.exs:LINE`, `--stale`, `--failed` (`ELIXIR-TEST-10`). `mix format <file>` formats one file (post-edit hook, `ELIXIR-TOOL-03`). Warnings are errors: the compiler's type system reports real bugs as warnings (`ELIXIR-TOOL-05`, `-06`); fix them, do not silence them.

## Race policy (WORKFLOW §6)
The BEAM has no shared-memory data races, so `race.mode = none` and there is no flag. The substitute is a required gate check, `test-repeat`: `mix test --repeat-until-failure 5 --warnings-as-errors` over async modules in random order with the seed ExUnit prints (`ELIXIR-CONC-04`). To satisfy it:
- Write `use ExUnit.Case, async: true` by default; set `async: false` only for global state (`Application.put_env`, named processes) and say why (`ELIXIR-TEST-09`).
- Never add retries or fixed order (`--seed 0`, `--max-cases 1` to hide a failure). Reproduce a failure with `mix test --seed N`, then fix the cause.
- Logical races remain (check-then-act, ETS read-modify-write, `Registry` lookup then `call`); review them (`ELIXIR-CONC-03`, `concurrency.md`).
- Concuerror is stale (last release 2020): advisory only.

## Coverage recipe
excoveralls over Erlang `:cover`; format lcov; `MIX_ENV=test mix coveralls.lcov` writes `cover/lcov.info` (line granularity, absolute `SF:` paths loops normalises). The built-in `mix test --cover` writes no lcov. New code needs at least 70% coverage (WORKFLOW §6, `ELIXIR-TEST-06`, `-07`).

**Wave 0 prerequisites** (D13). The architect or `elixir-engineer` adds these when missing and logs each as an override:
- `{:excoveralls, "~> 0.18", only: :test}` in `deps/0`;
- `test_coverage: [tool: ExCoveralls]` in `project/0`;
- `def cli, do: [preferred_envs: [coveralls: :test, "coveralls.lcov": :test, "coveralls.json": :test, "coveralls.html": :test]]` (so `mix coveralls.lcov` runs under `:test`);
- `coveralls.json` with `{"coverage_options": {"minimum_coverage": 0}}` (loops owns the threshold) and `skip_files` for `test/`, `priv/repo/migrations/` and generated code;
- when absent: `.formatter.exs` (the format check then runs), `.tool-versions` (`ELIXIR-REPO-08`).
The gate preflight fails with the exact snippet when excoveralls is missing.

## Layers in Elixir
Phoenix contexts are the boundary; hexagonal separates core from web (`specs/ARCHITECTURE.md`).
```
lib/my_app/                      core: NO Phoenix/Plug/MyAppWeb references
  application.ex                 composition root: supervision tree, adapter wiring
  orders.ex                      APPLICATION: context API, scope first
  orders/order.ex                DOMAIN: struct + pure functions
  orders/ports.ex                PORTS: @behaviour owned by the consumer
  orders/services/*.ex           APPLICATION: one use case per plain module
  orders/adapters/*.ex           OUTBOUND: Ecto store + schema, Req client
lib/my_app_web/                  INBOUND: endpoint, router, controllers, LiveViews, plugs
config/runtime.exs               adapter choice and secrets (config, not code)
test/support/                    Mox defmocks, case templates, fixtures
```
`loops arch` has no scanner for Elixir (`parser: none`): layering is enforced by Boundary compile warnings plus the optional `xref-cycles` check (`ELIXIR-ARCH-04`, `-05`); purity rules are review items (`architecture.md`).

## Footguns
1. A GenServer around pure functions: use a plain module.
2. `String.to_atom/1` on input: `String.to_existing_atom/1` or a map.
3. `spawn`/`Task.start` for long-lived work: `Task.Supervisor` or a child spec.
4. Slow `init/1`, unbounded `call`: `handle_continue/2`, explicit timeouts.
5. `map[:id]` for a required key: `map.id`.
6. `async: true` with global state: isolate or `async: false`; reproduce with `--seed N`.
7. Spawned process without `Sandbox.allow/3`; async on MySQL.
8. Blocking index migration: `concurrently: true`, `@disable_ddl_transaction true`.
9. `System.get_env` in `config.exs`: use `config/runtime.exs`.
10. Whole structs (`conn`) sent between processes: extract the fields.

## Run-scoped caches
`MIX_BUILD_ROOT` is per run (not `MIX_BUILD_PATH`); `deps/`, `MIX_HOME`, `HEX_HOME` are shared. Each run recompiles dependencies: expect a slow first check (`ELIXIR-OPS-05`). Cleanup never touches `deps/` or `mix.lock`.

## Load on demand
Universal rules: `specs/TESTING.md`, `specs/REPO_PRACTICES.md`, `specs/ARCHITECTURE.md`. Elixir digests:
- `repo.md`: layout, lockfile, toolchain pins, CI order, ignores.
- `architecture.md`: contexts, ports as behaviours, Boundary, purity, size limits.
- `idioms.md`: errors, pattern matching, structs, strings and time, naming.
- `testing.md`: ExUnit, Mox, Sandbox, coverage config, StreamData, flakiness.
- `concurrency.md`: OTP, supervision, GenServer rules, logical races.
- `security.md`: injection, atoms, deserialisation, Phoenix and release hardening.
- `data-api.md`: Ecto, migrations, Oban, HTTP, observability, releases.
- `frameworks.md`: Phoenix, LiveView, Ecto, Oban, Broadway, umbrella notes.
