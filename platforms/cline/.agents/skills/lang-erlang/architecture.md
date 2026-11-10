# Erlang architecture

Digest of `specs/languages/erlang/RULES.md` ARCH. Layering rules: `specs/ARCHITECTURE.md`; size limits: `specs/SOLID.md`.

## Apps as layers (`ERL-ARCH-01..06`)
- One OTP app per layer; `.app.src` `applications` lists only inward deps (domain: `[kernel, stdlib]`). The list is also the start order.
- Modules are flat atoms, so there is no `internal/`. Use a per-app prefix (`ms_pg_*`), a small `-export` facade, `-doc false.` on internals (OTP 27+), `-export_type` only for public types (`ERL-ARCH-03`).
- No import cycles between apps: `rebar3 xref`, `rebar3 tree`, the `applications` lists.
- Ports are `-callback` behaviours in the consuming app (`ms_app_repo`); the adapter app implements them; the release app injects the module with `application:set_env` or a function argument. No `application:get_env` in the domain.
- `loops arch` has no Erlang import parser: the `xref` check enforces direction (pack `arch.native_check`).

## Domain purity (`ERL-ARCH-07`)
No processes, `ets`, `file`, `gen_tcp`, `httpc`, `os:`, `erlang:system_time`, `os:timestamp`, `rand:` or `!` in the domain. Pass time and random values as arguments. Records stay private to one app: share through accessors or maps (`ERL-IDIOM-05`).

## Process design (`ERL-ARCH-08..10`)
- A process only where state must be serialised or has a lifecycle; never a gen_server per use case.
- `gen_statem` for state machines and sagas, `gen_server` for stateful servers, `gen_event` only for pluggable handlers; every long-lived process under a supervisor.
- Supervisors: `one_for_one` by default, `rest_for_one` for ordered dependencies, `one_for_all` for coupled siblings; set `intensity` and `period`; use dynamic `one_for_one` with `child_spec` instead of `simple_one_for_one`.

## Styles and size
Hexagonal is the default for services. DDD: records or maps as entities, a process per aggregate only for concurrent state. CQRS/event sourcing and `pg` or `brod` fan-out when audit or replay needs them. Modules under about 400 lines, functions under about 25 lines and 5 parameters (`ERL-ARCH-11`).
