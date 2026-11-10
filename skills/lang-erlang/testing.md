# Erlang testing

Digest of `specs/languages/erlang/RULES.md` TEST. Universal testing: `specs/TESTING.md` when present.

## Tools and layout (`ERL-TEST-01..03`)
- EUnit for units (`*_tests.erl`), Common Test for integration and system tests (`*_SUITE.erl`, not recursive under `test/`), PropEr for properties (`prop_*.erl`).
- CT: one suite per use case with `all/0`, `groups/0`, `init_per_suite`/`end_per_suite`, `init_per_testcase`. Names `<function>_<condition>_<result>`. EUnit fixtures `{setup, ...}` and `{foreach, ...}`, not copy-paste.
- Tests run in the `test` profile (`_build/test`); `-ifdef(TEST).` only sparingly in `src/` (`ERL-TEST-11`).

## Doubles and integration (`ERL-TEST-04/05`)
- Prefer fakes (an ETS-backed or in-memory module implementing the port behaviour). `meck` only for third-party modules, with `meck:unload` in teardown: it is process-global.
- Real Postgres (Testcontainers or compose) or a throw-away schema per suite; start the system with `application:ensure_all_started/1` in `init_per_suite`.

## Coverage (`ERL-TEST-06/07`)
`{cover_enabled, true}.`, `{cover_export_enabled, true}.` and the covertool plugin are wave-0 prerequisites. The gate runs `eunit --cover`, `ct --cover`, then `rebar3 covertool generate`; loops merges `_build/test/covertool/*.covertool.xml` and requires 70% of added lines. Line coverage only. `rebar3 cover --verbose` prints a table; `--min_coverage` fails below a threshold.

## Properties (`ERL-TEST-08`)
`rebar3 proper -n 500`; properties for pure domain functions, `proper_statem` for gen_servers, gen_statems and ETS code, `parallel_commands` to hunt races.

## Flakiness and iteration (`ERL-TEST-09/10/12`)
- No `timer:sleep` for sync: wait on a monitor or message. Unique registered names and ports per test; inject clock and RNG; `{group, g, [shuffle]}`; `[parallel]` groups only for isolated tests.
- Loop: `rebar3 eunit --module=m`, `--test=m:t_test`, `rebar3 ct --suite=test/x_SUITE --case=c`, `--group=g`, `rebar3 ct --repeat N`.
- JUnit XML for CI: `cth_surefire` in `ct_opts`, `eunit_surefire` in `eunit_opts`.
