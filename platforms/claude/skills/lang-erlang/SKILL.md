---
name: lang-erlang
description: Use when writing, reviewing, testing or setting up Erlang backend code in a loops run (the erlang language pack) - toolchain, layout, race policy, coverage, idioms and footguns.
---

# Erlang backend (OTP, rebar3)

Applies when `loops lang detect` reports `erlang` (`rebar.config`, `rebar.lock` or `erlang.mk`). Rules: `specs/languages/erlang/RULES.md` (IDs `ERL-<SEC>-NN`). Pack data: `specs/languages/erlang/pack.json` (`loops lang show erlang`). Layering and SOLID are language-neutral: `specs/ARCHITECTURE.md`, `specs/SOLID.md`.

## Project rules first

The project's own config wins over this skill: `rebar.config`, `elvis.config`, `.editorconfig`, `.tool-versions`, the framework (Cowboy, Nova, Elli) and any `erlang.mk` Makefile. When you follow one over a loops rule, record it: `loops run override --rule "<loops rule>" --project "<project rule, file:line>" --note "<what you did>"` (ROLES.md "Project rules first"). The practices-researcher reads `config_files` from the pack. An `erlang.mk` repo keeps its Makefile targets; the pack's argv are rebar3.

## Toolchain the gate runs

`loops gate` runs these in order; while iterating run a single one.

| Check | Command | Notes |
|---|---|---|
| compile (required) | `rebar3 compile` | `warnings_as_errors` comes from `rebar.config` (`ERL-TOOL-03`) |
| fmt (optional) | `rebar3 fmt --check` | erlfmt plugin; skipped with a hint if absent (`ERL-TOOL-02`) |
| xref (required, arch) | `rebar3 xref` | undefined and deprecated calls; app direction only with project `xref_queries` (`ERL-TOOL-04`, `ERL-ARCH-02`) |
| elvis (optional) | `rebar3 lint` | runs only when `elvis.config` exists; exits 1 without one (`ERL-TOOL-05`) |
| dialyzer (required) | `rebar3 dialyzer` | first run builds the PLT; keep `REBAR_CACHE_DIR` shared (`ERL-TOOL-06`) |
| eunit (required) | `rebar3 eunit --cover` | race check, once per `+S` variant |
| ct (required) | `rebar3 ct --cover` | race check, once per `+S` variant |
| proper (optional) | `rebar3 proper -n 100 --cover` | `ERL-TEST-08` |
| coverage (required) | `rebar3 covertool generate`, then loops parses `_build/test/covertool/*.covertool.xml` | Cobertura, new code at least 70% |
| audit, sbom (off by default) | `rebar3 audit`, `rebar3 sbom` | `--vuln`; audit needs network (`ERL-TOOL-08`) |

Narrow scope while working: `rebar3 eunit --module=m`, `rebar3 ct --suite=test/x_SUITE --case=c` (`ERL-TEST-10`). Never run the full gate as an implementer; the lead and QA do (ROLES.md). `ERL_CRASH_DUMP` is scoped per run by the gate; `REBAR_CACHE_DIR` and `REBAR_GLOBAL_CONFIG_DIR` stay shared (they hold packages, project plugins and the PLT), only `_build` is per project (`ERL-OPS-05`).

## Race policy (WORKFLOW §6)

The BEAM has no race detector, so the pack's mode is `none`; the substitute is required and never skipped:
- `eunit` and `ct` each run twice: `ERL_FLAGS="+S 1:1"` and `ERL_FLAGS="+S 8:8"` (`race.variants`). `ERL_FLAGS` reaches the BEAM that rebar3 starts. A test that passes under only one is flaky; never add retries.
- Every module with a gen_statem, an ETS table or a multi-process protocol has a `proper_statem` test (`ERL-CONC-03`, `ERL-TEST-08`). Use `rebar3 ct --repeat N` to hunt flakes locally.
- No `timer:sleep` for sync: wait on a monitor or message (`ERL-TEST-09`). Concuerror is not used (stale, dropped).

## Coverage recipe and wave-0 prerequisites (D13)

The architect or `erlang-engineer` adds these in wave 0 and logs each as an override if the project already differs:
- In `rebar.config`: `{cover_enabled, true}.` and `{cover_export_enabled, true}.`.
- `{project_plugins, [covertool]}.` and `{covertool, [{coverdata_files, ["eunit.coverdata", "ct.coverdata"]}]}.`. Without them `covertool generate` writes nothing and coverage fails.
- `{erl_opts, [debug_info, warnings_as_errors, warn_missing_spec]}.`, `{xref_checks, [...]}`, `{dialyzer, [...]}` (`ERL-TOOL-03/04/06`).
- Plugins `erlfmt`, `rebar3_lint` (with a committed `elvis.config`) and `rebar3_proper` when the project wants those checks; commit `rebar.lock`.
- Exclude generated modules with `{cover_excl_mods, [...]}`.

## How the loops layers map

```
apps/ms_domain/    pure functions, records, types; applications [kernel, stdlib]; no process, ets, os, clock
apps/ms_app/       use cases; ports are -callback behaviours here (consumer-owned); gen_server/gen_statem, supervisors
apps/ms_http/      inbound adapter: Cowboy handlers decode, call a use case, encode
apps/ms_pg/        outbound adapter: epgsql/pgo implementing an ms_app behaviour
apps/ms_release/   composition root: sup tree, adapter modules injected via application env, sys.config.src
```
Direction is the `.app.src` `applications` list plus project `xref_queries` (plain `rebar3 xref` does not check direction); modules are flat atoms, so use a per-app prefix (`ERL-ARCH-01/02`). There is no import parser; `loops arch` defers to the `xref` check, and reviewers check direction.

## Ten footguns

1. `binary_to_atom/2` or `list_to_atom/1` on input: `binary_to_existing_atom/2` or a lookup table.
2. `binary_to_term/1` on network data: `[safe]` and still no funs; use JSON or Protobuf.
3. Distribution cookie as security: TLS distribution, internal interface, never expose 4369.
4. Unbounded mailbox: admission control, a pool, `max_heap_size`, watch `message_queue_len`.
5. Selective `receive` over a large mailbox: use `gen_server:call` or aliases.
6. `try f() catch _:_ -> ok end`: let it crash or catch a specific class with the stacktrace.
7. `gen_server:call` cycles or the 5 s default: cast, async reply, chosen timeouts.
8. Bare `spawn`: `proc_lib` or a `gen_*` under a supervisor.
9. ETS `lookup` then `insert` from many processes: `update_counter/4`, `insert_new/2` or one owner.
10. Big terms copied between processes, or a small slice keeping a huge binary: `binary:copy/1`, ETS, `persistent_term`.

## Load on demand

- `repo.md`: rebar3 layout, lock, toolchain pin, CI, Hex.
- `architecture.md`: apps as layers, behaviours as ports, OTP process design, purity.
- `idioms.md`: let it crash, specs, strings, atoms, time, macros.
- `testing.md`: EUnit, Common Test, PropEr, fakes, cover, flakiness.
- `concurrency.md`: processes, mailboxes, ETS, call timeouts, the race substitute.
- `security.md`: atoms, deserialization, distribution, crypto, supply chain.
- `data-api.md`: persistence, migrations, HTTP, messaging, observability, releases.
- `frameworks.md`: Cowboy, Nova, Elli, erlang.mk, footguns.
