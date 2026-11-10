# Testing (Elixir)

Digest of RULES.md TEST. TDD applies as in the loops testing skill; tests for all new code, at least 70% new-code coverage.

## Layout (`ELIXIR-TEST-01`, `-02`)
`test/` mirrors `lib/`; files end `_test.exs`; `ExUnit.start()` in `test/test_helper.exs`; helpers in `test/support/*.ex`. `Phoenix.ConnTest` for HTTP, `Phoenix.LiveViewTest` for LiveView, `doctest MyApp.Mod` for examples.

## Structure (`ELIXIR-TEST-03`)
`describe "fun/arity"` (cannot nest), sentence names, one behaviour per test, `setup` and named setups, `@tag`/`@moduletag` for `--include/--exclude`, `for` comprehensions for table-style cases.

## Doubles (`ELIXIR-TEST-04`)
Prefer fakes and in-memory adapters. Mox against `@behaviour` ports: `Mox.defmock(Mock, for: Behaviour)` in `test/support/mocks.ex`, `verify_on_exit!`, `Mox.allow/3` for other processes; stay in private mode for `async: true`. Do not mock what you do not own; wrap the client in a behaviour first.

## Database (`ELIXIR-TEST-05`)
Ecto SQL Sandbox: `pool: Ecto.Adapters.SQL.Sandbox` in `config/test.exs`; `Ecto.Adapters.SQL.Sandbox.mode(Repo, :manual)` at the end of `test_helper.exs`; `start_owner!(Repo, shared: not tags[:async])` in the case template; `allow/3` for spawned processes. Async only on PostgreSQL. Never mock the DB.

## Coverage (`ELIXIR-TEST-06`, `-07`)
- Run `MIX_ENV=test mix coveralls.lcov` (the gate's `test` check); output `cover/lcov.info`. Other reports: `coveralls.json` to `cover/excoveralls.json`, `coveralls.cobertura` to `cover/cobertura.xml`.
- `coveralls.json`: `minimum_coverage: 0`, `skip_files` regexes; in code `# coveralls-ignore-start` / `-stop` / `-next-line` (use sparingly and say why).
- Line granularity only; no branch coverage. Exclude `test/**`, migrations, generated code.

## Race substitute and flakiness (`ELIXIR-TEST-09`, `ELIXIR-CONC-04`)
- Gate check `test-repeat`: `mix test --repeat-until-failure 5 --warnings-as-errors`; the seed prints as `Randomized with seed N`; reproduce with `mix test --seed N`.
- Default `async: true`. No global state in async modules (`Application.put_env`, working directory, named processes); otherwise `async: false` with a reason.
- `start_supervised!/1`, `@tag :tmp_dir`, `on_exit/1`; `assert_receive` with a timeout, never `Process.sleep`; inject clocks.

## Properties and data (`ELIXIR-TEST-08`, `-11`)
StreamData (`use ExUnitProperties`, `check all`) for parsers, serialisers and invariants, beside example tests. ExMachina or hand-written fixtures in `test/support/`, one factory per schema; fixtures never mutate shared state.

## Fast loop (`ELIXIR-TEST-10`)
`mix test file_test.exs:LINE`, `--stale`, `--failed`, `--trace` (forces `--max-cases 1`), `--slowest 10`. In CI, `--partitions N` with `MIX_TEST_PARTITION` and `mix test.coverage` to merge.
